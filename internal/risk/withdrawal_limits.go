package risk

import (
	"context"
	"sync"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// WithdrawalLimits enforces withdrawal amount restrictions.
type WithdrawalLimits struct {
	pool *pgxpool.Pool
	mu   sync.RWMutex

	// Per-currency limits
	maxSingle  map[common.Currency]decimal.Decimal
	maxDaily   map[common.Currency]decimal.Decimal
	minAmount  map[common.Currency]decimal.Decimal

	// Daily tracking (userID:currency -> amount withdrawn today)
	dailyTracker map[string]decimal.Decimal
	lastReset    time.Time
}

// NewWithdrawalLimits creates a withdrawal limits enforcer.
func NewWithdrawalLimits(pool *pgxpool.Pool) *WithdrawalLimits {
	return &WithdrawalLimits{
		pool: pool,
		maxSingle: map[common.Currency]decimal.Decimal{
			"ETH":  decimalMust("50"),
			"USDT": decimalMust("500000"),
		},
		maxDaily: map[common.Currency]decimal.Decimal{
			"ETH":  decimalMust("200"),
			"USDT": decimalMust("2000000"),
		},
		minAmount: map[common.Currency]decimal.Decimal{
			"ETH":  decimalMust("0.01"),
			"USDT": decimalMust("10"),
		},
		dailyTracker: make(map[string]decimal.Decimal),
		lastReset:    time.Now(),
	}
}

// Validate checks if a withdrawal request is within limits.
func (wl *WithdrawalLimits) Validate(ctx context.Context, userID string, currency common.Currency, amount decimal.Decimal) error {
	wl.mu.Lock()
	defer wl.mu.Unlock()

	// Reset daily tracker if day changed
	if time.Since(wl.lastReset) > 24*time.Hour {
		wl.dailyTracker = make(map[string]decimal.Decimal)
		wl.lastReset = time.Now()
	}

	// Check minimum
	if minAmt, ok := wl.minAmount[currency]; ok {
		if amount.Cmp(minAmt) < 0 {
			return common.ErrWithdrawalTooLarge
		}
	}

	// Check single withdrawal limit
	if maxSingle, ok := wl.maxSingle[currency]; ok {
		if amount.Cmp(maxSingle) > 0 {
			log.Warn().
				Str("user_id", userID).
				Str("currency", string(currency)).
				Str("amount", amount.String()).
				Msg("withdrawal exceeds single limit")
			return common.ErrWithdrawalTooLarge
		}
	}

	// Check daily limit
	key := userID + ":" + string(currency)
	dailyTotal := wl.dailyTracker[key]
	newDailyTotal := dailyTotal.Add(amount)
	if maxDaily, ok := wl.maxDaily[currency]; ok {
		if newDailyTotal.Cmp(maxDaily) > 0 {
			log.Warn().
				Str("user_id", userID).
				Str("currency", string(currency)).
				Str("daily_total", newDailyTotal.String()).
				Msg("withdrawal exceeds daily limit")
			return common.ErrWithdrawalTooLarge
		}
	}
	wl.dailyTracker[key] = newDailyTotal

	return nil
}

func decimalMust(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
