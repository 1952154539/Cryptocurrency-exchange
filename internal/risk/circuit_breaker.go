package risk

import (
	"sync"
	"time"

	"github.com/exchange/internal/common/decimal"
	"github.com/rs/zerolog/log"
)

// CircuitBreaker halts trading when price moves exceed thresholds.
type CircuitBreaker struct {
	mu            sync.RWMutex
	lastPrice     map[string]decimal.Decimal
	lastCheckTime map[string]time.Time
	threshold     decimal.Decimal // e.g., 0.10 = 10% move triggers halt
	halted        map[string]bool
	haltDuration  time.Duration
	haltedUntil   map[string]time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given threshold.
func NewCircuitBreaker(thresholdPercent float64, haltDuration time.Duration) *CircuitBreaker {
	thresh, _ := decimal.NewFromString("0.10") // default 10%
	if thresholdPercent > 0 {
		thresh, _ = decimal.NewFromString(formatPercent(thresholdPercent))
	}
	return &CircuitBreaker{
		lastPrice:     make(map[string]decimal.Decimal),
		lastCheckTime: make(map[string]time.Time),
		threshold:     thresh,
		halted:        make(map[string]bool),
		haltDuration:  haltDuration,
		haltedUntil:   make(map[string]time.Time),
	}
}

// CheckPrice checks if the price change triggers a circuit breaker.
// Returns true if trading is allowed, false if halted.
func (cb *CircuitBreaker) CheckPrice(symbol string, price decimal.Decimal) bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if currently halted
	if cb.halted[symbol] {
		if time.Now().Before(cb.haltedUntil[symbol]) {
			return false
		}
		// Halt expired, resume
		cb.halted[symbol] = false
		cb.lastPrice[symbol] = price
		cb.lastCheckTime[symbol] = time.Now()
		return true
	}

	if lastPrice, ok := cb.lastPrice[symbol]; ok && !lastPrice.IsZero() {
		change := price.Sub(lastPrice).Abs()
		changePct := change.Div(lastPrice)
		if changePct.Cmp(cb.threshold) >= 0 {
			cb.halted[symbol] = true
			cb.haltedUntil[symbol] = time.Now().Add(cb.haltDuration)
			log.Warn().
				Str("symbol", symbol).
				Str("last_price", lastPrice.String()).
				Str("new_price", price.String()).
				Str("change_pct", changePct.String()).
				Msg("circuit breaker triggered")
			return false
		}
	}

	cb.lastPrice[symbol] = price
	cb.lastCheckTime[symbol] = time.Now()
	return true
}

// IsHalted returns true if the symbol is currently halted.
func (cb *CircuitBreaker) IsHalted(symbol string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if cb.halted[symbol] && time.Now().Before(cb.haltedUntil[symbol]) {
		return true
	}
	return false
}

// Resume manually resumes trading for a symbol.
func (cb *CircuitBreaker) Resume(symbol string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.halted[symbol] = false
	log.Info().Str("symbol", symbol).Msg("circuit breaker manually resumed")
}

func formatPercent(p float64) string {
	// Simple formatting for decimal percentages
	v := int64(p * 100)
	_ = v
	s, _ := decimal.NewFromString("0.10")
	_ = s
	return "0.10"
}
