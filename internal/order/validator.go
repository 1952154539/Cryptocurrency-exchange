package order

import (
	"context"
	"fmt"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
)

// MarketConfig describes the trading parameters for a market.
type MarketConfig struct {
	Symbol       common.Symbol
	BaseCurrency common.Currency
	QuoteCurrency common.Currency
	PriceTick    decimal.Decimal
	QtyStep      decimal.Decimal
	MinOrderQty  decimal.Decimal
	MaxOrderQty  decimal.Decimal
	MinNotional  decimal.Decimal
	Status       string
}

// BalanceProvider checks user balances.
type BalanceProvider interface {
	GetBalance(ctx context.Context, userID string, currency common.Currency) (decimal.Decimal, error)
	FreezeBalance(ctx context.Context, userID string, currency common.Currency, amount decimal.Decimal) error
}

// RateLimiter checks request rate limits.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int) bool
}

// Validator validates incoming orders.
type Validator struct {
	markets     map[common.Symbol]*MarketConfig
	balances    BalanceProvider
	rateLimiter RateLimiter
}

// NewValidator creates an order validator.
func NewValidator(markets []*MarketConfig, balances BalanceProvider, rateLimiter RateLimiter) *Validator {
	m := make(map[common.Symbol]*MarketConfig)
	for _, mc := range markets {
		m[mc.Symbol] = mc
	}
	return &Validator{
		markets:     m,
		balances:    balances,
		rateLimiter: rateLimiter,
	}
}

// PlaceOrderRequest represents a request to place an order.
type PlaceOrderRequest struct {
	UserID        string
	Symbol        common.Symbol
	Side          common.Side
	Type          common.OrderType
	TimeInForce   common.TimeInForce
	Price         decimal.Decimal
	StopPrice     decimal.Decimal
	Quantity      decimal.Decimal
	ClientOrderID string
}

// Validate checks the entire order request.
func (v *Validator) Validate(ctx context.Context, req *PlaceOrderRequest) error {
	// 1. Market exists and is active
	market, ok := v.markets[req.Symbol]
	if !ok {
		return common.ErrMarketNotAvailable
	}
	if market.Status != "active" {
		return common.ErrMarketNotAvailable
	}

	// 2. Validate side
	if req.Side != common.SideBuy && req.Side != common.SideSell {
		return fmt.Errorf("invalid side: %s", req.Side)
	}

	// 3. Validate order type
	if err := v.validateOrderType(req); err != nil {
		return err
	}

	// 4. Validate time in force
	if req.Type == common.OrderTypeLimit || req.Type == common.OrderTypeStopLimit {
		if req.TimeInForce == "" {
			req.TimeInForce = common.TIF_GTC
		}
		if req.TimeInForce != common.TIF_GTC &&
			req.TimeInForce != common.TIF_IOC &&
			req.TimeInForce != common.TIF_FOK {
			return common.ErrInvalidTimeInForce
		}
	}

	// 5. Price precision (tick size)
	if req.Type == common.OrderTypeLimit || req.Type == common.OrderTypeStopLimit {
		if !v.validPrecision(req.Price, market.PriceTick) {
			return fmt.Errorf("%w: tick size is %s", common.ErrInvalidPricePrecision, market.PriceTick.String())
		}
	}

	// 6. Quantity precision (step size)
	if !v.validPrecision(req.Quantity, market.QtyStep) {
		return fmt.Errorf("%w: step size is %s", common.ErrInvalidQtyPrecision, market.QtyStep.String())
	}

	// 7. Order size range
	if req.Quantity.Cmp(market.MinOrderQty) < 0 {
		return fmt.Errorf("%w: min is %s", common.ErrOrderSizeOutOfRange, market.MinOrderQty.String())
	}
	if req.Quantity.Cmp(market.MaxOrderQty) > 0 {
		return fmt.Errorf("%w: max is %s", common.ErrOrderSizeOutOfRange, market.MaxOrderQty.String())
	}

	// 8. Min notional check (for limit orders)
	if req.Type == common.OrderTypeLimit && !req.Price.IsZero() {
		notional := req.Price.Mul(req.Quantity)
		if notional.Cmp(market.MinNotional) < 0 {
			return fmt.Errorf("%w: min notional is %s", common.ErrOrderSizeOutOfRange, market.MinNotional.String())
		}
	}

	// 9. Balance check
	if err := v.checkBalance(ctx, req, market); err != nil {
		return err
	}

	// 10. Rate limit check
	if v.rateLimiter != nil && !v.rateLimiter.Allow(ctx, req.UserID, 20) {
		return common.ErrRateLimitExceeded
	}

	return nil
}

func (v *Validator) validateOrderType(req *PlaceOrderRequest) error {
	switch req.Type {
	case common.OrderTypeMarket, common.OrderTypeLimit:
		return nil
	case common.OrderTypeStopLoss, common.OrderTypeStopLimit:
		if req.StopPrice.IsZero() {
			return fmt.Errorf("stop price is required for stop orders")
		}
		return nil
	default:
		return common.ErrInvalidOrderType
	}
}

func (v *Validator) checkBalance(ctx context.Context, req *PlaceOrderRequest, market *MarketConfig) error {
	if req.Side == common.SideBuy {
		// For buys, need quote currency
		var required decimal.Decimal
		if req.Type == common.OrderTypeMarket {
			// For market buys, verify user has at least some balance
			// Full price is unknown until matching, but we check that the account exists
			balance, err := v.balances.GetBalance(ctx, req.UserID, market.QuoteCurrency)
			if err != nil {
				return err
			}
			if balance.IsZero() {
				return fmt.Errorf("%w: zero balance for %s", common.ErrInsufficientBalance, market.QuoteCurrency)
			}
			return nil
		}
		required = req.Price.Mul(req.Quantity)
		balance, err := v.balances.GetBalance(ctx, req.UserID, market.QuoteCurrency)
		if err != nil {
			return err
		}
		if balance.Cmp(required) < 0 {
			return fmt.Errorf("%w: need %s %s, have %s",
				common.ErrInsufficientBalance, required.String(), market.QuoteCurrency, balance.String())
		}
	} else {
		// For sells, need base currency
		balance, err := v.balances.GetBalance(ctx, req.UserID, market.BaseCurrency)
		if err != nil {
			return err
		}
		if balance.Cmp(req.Quantity) < 0 {
			return fmt.Errorf("%w: need %s %s, have %s",
				common.ErrInsufficientBalance, req.Quantity.String(), market.BaseCurrency, balance.String())
		}
	}
	return nil
}

// validPrecision checks if a value is a multiple of the step size.
func (v *Validator) validPrecision(value, step decimal.Decimal) bool {
	if step.IsZero() {
		return true
	}
	// Use modulo: remainder = value % step using big.Int math
	remainder := value.Mod(step)
	return remainder.IsZero()
}

// BalanceProvider returns the configured balance provider.
func (v *Validator) BalanceProvider() BalanceProvider {
	return v.balances
}

// MarketInfo returns market configuration for a symbol.
func (v *Validator) MarketInfo(symbol common.Symbol) *MarketConfig {
	return v.markets[symbol]
}
