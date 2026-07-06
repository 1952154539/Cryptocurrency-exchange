package margin

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/rs/zerolog/log"
)

// Position represents an open leveraged position.
type Position struct {
	ID               string
	UserID           string
	Symbol           common.Symbol
	Side             common.Side
	Quantity         decimal.Decimal
	EntryPrice       decimal.Decimal
	Leverage         int
	MarginType       string // "isolated" or "cross"
	MarginBalance    decimal.Decimal
	UnrealizedPnL    decimal.Decimal
	LiquidationPrice decimal.Decimal
	OpenedAt         time.Time
	LastFundingTime  time.Time
}

// Engine manages leveraged positions and liquidations.
type Engine struct {
	mu           sync.RWMutex
	positions    map[string][]*Position // userID -> positions
	markPrices   map[common.Symbol]decimal.Decimal
	fundingRate  map[common.Symbol]decimal.Decimal // per 8h
	fundingTimes map[common.Symbol]time.Time
	mmr          decimal.Decimal // maintenance margin ratio (e.g., 0.005 = 0.5%)
	lastFunding  time.Time
}

// NewEngine creates a margin trading engine.
func NewEngine(maintenanceMarginRatio float64) *Engine {
	mmr, _ := decimal.NewFromString(fmt.Sprintf("%.4f", maintenanceMarginRatio))
	return &Engine{
		positions:    make(map[string][]*Position),
		markPrices:   make(map[common.Symbol]decimal.Decimal),
		fundingRate:  make(map[common.Symbol]decimal.Decimal),
		fundingTimes: make(map[common.Symbol]time.Time),
		mmr:          mmr,
		lastFunding:  time.Now(),
	}
}

// OpenPosition creates a new leveraged position.
func (e *Engine) OpenPosition(userID string, symbol common.Symbol, side common.Side, quantity, entryPrice decimal.Decimal, leverage int, marginType string, marginBalance decimal.Decimal) (*Position, error) {
	if leverage < 1 || leverage > 125 {
		return nil, fmt.Errorf("leverage must be 1-125, got %d", leverage)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	pos := &Position{
		ID:            common.NewOrderID(),
		UserID:        userID,
		Symbol:        symbol,
		Side:          side,
		Quantity:      quantity,
		EntryPrice:    entryPrice,
		Leverage:      leverage,
		MarginType:    marginType,
		MarginBalance: marginBalance,
		OpenedAt:      time.Now(),
	}

	// Calculate liquidation price
	pos.LiquidationPrice = e.calcLiquidationPrice(pos)
	e.positions[userID] = append(e.positions[userID], pos)

	log.Info().
		Str("user_id", userID).Str("symbol", string(symbol)).
		Str("side", string(side)).Str("quantity", quantity.String()).
		Int("leverage", leverage).Str("liquidation", pos.LiquidationPrice.String()).
		Msg("position opened")
	return pos, nil
}

// CheckLiquidations scans all positions and liquidates those below maintenance margin.
func (e *Engine) CheckLiquidations(ctx context.Context, symbol common.Symbol) []*Position {
	e.mu.Lock()
	defer e.mu.Unlock()

	var liquidated []*Position
	markPrice, ok := e.markPrices[symbol]
	if !ok {
		return nil
	}

	for userID, positions := range e.positions {
		var remaining []*Position
		for _, pos := range positions {
			if pos.Symbol != symbol {
				remaining = append(remaining, pos)
				continue
			}
			if e.shouldLiquidate(pos, markPrice) {
				log.Warn().
					Str("user_id", userID).Str("position_id", pos.ID).
					Str("mark_price", markPrice.String()).
					Str("liquidation_price", pos.LiquidationPrice.String()).
					Msg("position liquidated")
				liquidated = append(liquidated, pos)
			} else {
				remaining = append(remaining, pos)
			}
		}
		e.positions[userID] = remaining
	}
	return liquidated
}

// UpdateMarkPrice sets the current mark price for a symbol.
func (e *Engine) UpdateMarkPrice(symbol common.Symbol, price decimal.Decimal) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.markPrices[symbol] = price
}

// CalculateFunding applies funding rate payments every 8 hours.
func (e *Engine) CalculateFunding(symbol common.Symbol) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if time.Since(e.lastFunding) < 8*time.Hour {
		return
	}

	rate, ok := e.fundingRate[symbol]
	if !ok {
		return
	}

	for _, positions := range e.positions {
		for _, pos := range positions {
			if pos.Symbol != symbol {
				continue
			}
			notional := pos.EntryPrice.Mul(pos.Quantity)
			fundingPayment := notional.Mul(rate)
			if (pos.Side == common.SideBuy && rate.IsNegative()) || (pos.Side == common.SideSell && !rate.IsNegative()) {
				pos.MarginBalance = pos.MarginBalance.Sub(fundingPayment.Abs())
			} else {
				pos.MarginBalance = pos.MarginBalance.Add(fundingPayment.Abs())
			}
		}
	}
	e.lastFunding = time.Now()

	log.Info().Str("symbol", string(symbol)).Str("rate", rate.String()).Msg("funding applied")
}

func (e *Engine) calcLiquidationPrice(pos *Position) decimal.Decimal {
	requiredMargin := pos.EntryPrice.Mul(pos.Quantity).Div(decimal.NewFromInt64(int64(pos.Leverage)))
	liquidationThreshold := requiredMargin.Mul(e.mmr)

	if pos.Side == common.SideBuy {
		return pos.EntryPrice.Sub(pos.EntryPrice.Mul(liquidationThreshold).Div(requiredMargin))
	}
	return pos.EntryPrice.Add(pos.EntryPrice.Mul(liquidationThreshold).Div(requiredMargin))
}

func (e *Engine) shouldLiquidate(pos *Position, markPrice decimal.Decimal) bool {
	if pos.Side == common.SideBuy {
		return markPrice.Cmp(pos.LiquidationPrice) <= 0
	}
	return markPrice.Cmp(pos.LiquidationPrice) >= 0
}

// GetPositions returns all positions for a user.
func (e *Engine) GetPositions(userID string) []*Position {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.positions[userID]
}
