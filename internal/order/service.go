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
	validator     *Validator
	stateMachine  *StateMachine
	repo          *Repository
	engine        *matching.Engine
	eventBus      events.EventBus
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

	// 2. Generate order ID
	orderID := common.NewOrderID()

	// 3. Create matching engine order
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

	// 4. Persist to DB
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
		return nil, fmt.Errorf("persist order: %w", err)
	}

	// 5. Submit to matching engine
	matches, remaining, err := s.engine.PlaceOrder(ctx, engineOrder)
	if err != nil {
		// Engine rejected; update DB
		s.repo.UpdateStatus(ctx, orderID, common.OrderStatusRejected, decimal.Zero)
		return nil, fmt.Errorf("matching engine: %w", err)
	}

	// 6. Update status based on result
	status := common.OrderStatusOpen
	filledQty := decimal.Zero

	if len(matches) > 0 {
		filledQty = req.Quantity.Sub(remaining.Remaining())
		if remaining == nil || remaining.Status.IsFinal() {
			status = common.OrderStatusFilled
		} else {
			status = common.OrderStatusPartiallyFilled
		}
	}

	if remaining != nil && remaining.Status == common.OrderStatusCancelled {
		status = common.OrderStatusCancelled
	}

	s.repo.UpdateStatus(ctx, orderID, status, filledQty)

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
	// 1. Verify order exists and belongs to user
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

	// 2. Remove from matching engine
	cancelled, err := s.engine.CancelOrder(ctx, symbol, orderID)
	if err != nil {
		return err
	}
	_ = cancelled

	// 3. Update DB
	if err := s.repo.UpdateStatus(ctx, orderID, common.OrderStatusCancelled, decimal.Zero); err != nil {
		return err
	}

	log.Info().
		Str("order_id", orderID).
		Str("user_id", userID).
		Msg("order cancelled")

	return nil
}

// GetOrder returns order details.
func (s *Service) GetOrder(ctx context.Context, orderID string) (*StoredOrder, error) {
	return s.repo.GetByID(ctx, orderID)
}

// GetOpenOrders returns all open orders for a user.
func (s *Service) GetOpenOrders(ctx context.Context, userID string, symbol string) ([]*StoredOrder, error) {
	return s.repo.GetOpenOrders(ctx, userID, symbol)
}

// Ensure imports are used
var _ = decimal.Zero
