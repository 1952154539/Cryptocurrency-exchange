package order

import (
	"context"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
	"github.com/rs/zerolog/log"
)

// Service handles order placement, cancellation, and tracking.
type Service struct {
	validator    *Validator
	stateMachine *StateMachine
	repo         *Repository
	engine       *matching.Engine
	eventBus     events.EventBus
}

// NewService creates an order service.
func NewService(
	validator *Validator,
	repo *Repository,
	engine *matching.Engine,
	eventBus events.EventBus,
) *Service {
	return &Service{
		validator:    validator,
		stateMachine: NewStateMachine(),
		repo:         repo,
		engine:       engine,
		eventBus:     eventBus,
	}
}

// PlaceOrderResponse contains the result of placing an order.
type PlaceOrderResponse struct {
	OrderID       string
	ClientOrderID string
	Status        common.OrderStatus
	Matches       []*matching.MatchResult
	FilledQty     decimal.Decimal
}

// PlaceOrder validates and submits an order to the matching engine.
func (s *Service) PlaceOrder(ctx context.Context, req *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	// 1. Validate
	if err := s.validator.Validate(ctx, req); err != nil {
		return nil, fmt.Errorf("order validation: %w", err)
	}

	// 2. Freeze balance to prevent overselling from concurrent orders
	var freezeCurrency common.Currency
	var freezeAmount decimal.Decimal
	if bp := s.validator.BalanceProvider(); bp != nil {
		market := s.validator.MarketInfo(req.Symbol)
		if market != nil {
			if req.Side == common.SideBuy {
				if req.Type == common.OrderTypeLimit || req.Type == common.OrderTypeStopLimit {
					freezeCurrency = market.QuoteCurrency
					freezeAmount = req.Price.Mul(req.Quantity)
				} else if req.Type == common.OrderTypeMarket {
					freezeCurrency = market.QuoteCurrency
					if bal, err := bp.GetBalance(ctx, req.UserID, freezeCurrency); err == nil {
						freezeAmount = bal
					}
				}
			} else {
				freezeCurrency = market.BaseCurrency
				freezeAmount = req.Quantity
			}
			if !freezeAmount.IsZero() {
				if err := bp.FreezeBalance(ctx, req.UserID, freezeCurrency, freezeAmount); err != nil {
					return nil, fmt.Errorf("freeze balance: %w", err)
				}
			}
		}
	}

	// 3. Generate order ID
	orderID := common.NewOrderID()

	// 4. Create matching engine order
	now := time.Now().UnixNano()
	engineOrder := &matching.Order{
		OrderID:     orderID,
		UserID:      req.UserID,
		Symbol:      req.Symbol,
		Side:        req.Side,
		Type:        req.Type,
		TimeInForce: req.TimeInForce,
		Price:       req.Price,
		StopPrice:   req.StopPrice,
		Quantity:    req.Quantity,
		Timestamp:   now,
		Status:      common.OrderStatusCreated,
	}

	// 5. Persist to DB
	if s.repo != nil {
		storedOrder := &StoredOrder{
			OrderID:       orderID,
			ClientOrderID: req.ClientOrderID,
			UserID:        req.UserID,
			Symbol:        string(req.Symbol),
			Side:          string(req.Side),
			Type:          string(req.Type),
			TimeInForce:   string(req.TimeInForce),
			Price:         req.Price.String(),
			StopPrice:     req.StopPrice.String(),
			Quantity:      req.Quantity.String(),
			FilledQty:     "0",
			Status:        string(common.OrderStatusCreated),
		}
		if err := s.repo.Create(ctx, storedOrder); err != nil {
			s.unfreezeBalance(ctx, req.UserID, freezeCurrency, freezeAmount)
			return nil, fmt.Errorf("persist order: %w", err)
		}
	}

	// 6. Submit to matching engine
	matches, remaining, err := s.engine.PlaceOrder(ctx, engineOrder)
	if err != nil {
		s.unfreezeBalance(ctx, req.UserID, freezeCurrency, freezeAmount)
		if s.repo != nil {
			s.repo.UpdateStatus(ctx, orderID, common.OrderStatusRejected, decimal.Zero)
		}
		return nil, fmt.Errorf("matching engine: %w", err)
	}

	// 7. Update status based on result
	status := common.OrderStatusOpen
	filledQty := decimal.Zero

	if len(matches) > 0 {
		if remaining == nil {
			filledQty = req.Quantity
			status = common.OrderStatusFilled
		} else {
			filledQty = req.Quantity.Sub(remaining.Remaining())
			if remaining.Status.IsFinal() {
				status = common.OrderStatusFilled
			} else {
				status = common.OrderStatusPartiallyFilled
			}
		}
	}

		if remaining != nil && (remaining.Status == common.OrderStatusCancelled || remaining.Status == common.OrderStatusRejected) {
			status = remaining.Status
	}

	// Unfreeze filled portion. Unfilled stays frozen until cancelled.
	// Settlement will deduct from balance AND reduce frozen_balance.
	if !freezeAmount.IsZero() && !filledQty.IsZero() {
		if req.Side == common.SideBuy {
			filledCost := req.Price.Mul(filledQty)
			s.unfreezeBalance(ctx, req.UserID, freezeCurrency, filledCost)
		} else {
			s.unfreezeBalance(ctx, req.UserID, freezeCurrency, filledQty)
		}
	} else if status == common.OrderStatusCancelled || status == common.OrderStatusRejected {
		s.unfreezeBalance(ctx, req.UserID, freezeCurrency, freezeAmount)
	}

	if s.repo != nil {
		s.repo.UpdateStatus(ctx, orderID, status, filledQty)
	}

	log.Info().
		Str("order_id", orderID).
		Str("user_id", req.UserID).
		Str("symbol", string(req.Symbol)).
		Str("side", string(req.Side)).
		Str("type", string(req.Type)).
		Int("matches", len(matches)).
		Str("status", string(status)).
		Msg("order placed")

	return &PlaceOrderResponse{
		OrderID:       orderID,
		ClientOrderID: req.ClientOrderID,
		Status:        status,
		Matches:       matches,
		FilledQty:     filledQty,
	}, nil
}

// CancelOrder cancels an open order.
func (s *Service) CancelOrder(ctx context.Context, userID string, symbol common.Symbol, orderID string) error {
	if s.repo == nil {
		return fmt.Errorf("order persistence not available")
	}
	stored, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if stored.UserID != userID {
		return common.ErrUnauthorized
	}
	if common.OrderStatus(stored.Status).IsFinal() {
		return fmt.Errorf("order %s is already in final state: %s", orderID, stored.Status)
	}

	// Validate state transition
	if !s.stateMachine.CanTransition(common.OrderStatus(stored.Status), common.OrderStatusCancelled) {
		return fmt.Errorf("cannot cancel order in status %s", stored.Status)
	}

	cancelled, err := s.engine.CancelOrder(ctx, symbol, orderID)
	if err != nil {
		return err
	}
	_ = cancelled

	// Unfreeze the balance that was locked at order placement
	s.unfreezeCancelOrder(ctx, stored)

	if err := s.repo.UpdateStatus(ctx, orderID, common.OrderStatusCancelled, decimal.Zero); err != nil {
		return err
	}

	log.Info().
		Str("order_id", orderID).
		Str("user_id", userID).
		Msg("order cancelled")

	return nil
}

// unfreezeCancelOrder releases frozen balance when an order is cancelled.
func (s *Service) unfreezeCancelOrder(ctx context.Context, stored *StoredOrder) {
	if s.validator.BalanceProvider() == nil {
		return
	}
	market := s.validator.MarketInfo(common.Symbol(stored.Symbol))
	if market == nil {
		return
	}

	qty, err := decimal.NewFromString(stored.Quantity)
	if err != nil {
		return
	}
	price, err := decimal.NewFromString(stored.Price)
	if err != nil {
		return
	}
	filled, err := decimal.NewFromString(stored.FilledQty)
	if err != nil {
		return
	}

	var freezeCurrency common.Currency
	var freezeAmount decimal.Decimal

	if common.Side(stored.Side) == common.SideBuy {
		if stored.Type != string(common.OrderTypeMarket) {
			freezeCurrency = market.QuoteCurrency
			freezeAmount = price.Mul(qty)
		}
	} else {
		freezeCurrency = market.BaseCurrency
		freezeAmount = qty
	}

	if !freezeAmount.IsZero() {
		// Unfreeze only the unfilled portion
		if !filled.IsZero() && common.Side(stored.Side) == common.SideBuy {
			filledCost := price.Mul(filled)
			if filledCost.Cmp(freezeAmount) < 0 {
				s.unfreezeBalance(ctx, stored.UserID, freezeCurrency, freezeAmount.Sub(filledCost))
			}
		} else if !filled.IsZero() {
			if filled.Cmp(freezeAmount) < 0 {
				s.unfreezeBalance(ctx, stored.UserID, freezeCurrency, freezeAmount.Sub(filled))
			}
		} else {
			s.unfreezeBalance(ctx, stored.UserID, freezeCurrency, freezeAmount)
		}
	}
}

// GetOrder returns order details.
func (s *Service) GetOrder(ctx context.Context, orderID string) (*StoredOrder, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("order persistence not available")
	}
	return s.repo.GetByID(ctx, orderID)
}

// GetOpenOrders returns all open orders for a user.
func (s *Service) GetOpenOrders(ctx context.Context, userID string, symbol string) ([]*StoredOrder, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("order persistence not available")
	}
	return s.repo.GetOpenOrders(ctx, userID, symbol)
}

func (s *Service) unfreezeBalance(ctx context.Context, userID string, currency common.Currency, amount decimal.Decimal) {
	if amount.IsZero() {
		return
	}
	if bp := s.validator.BalanceProvider(); bp != nil {
		if provider, ok := bp.(*DbBalanceProvider); ok {
			if err := provider.UnfreezeBalance(ctx, userID, currency, amount); err != nil {
				log.Warn().Err(err).
					Str("user_id", userID).
					Str("currency", string(currency)).
					Str("amount", amount.String()).
					Msg("failed to unfreeze balance")
			}
		}
	}
}

