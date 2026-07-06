package order

import (
	"context"
	"fmt"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DbBalanceProvider implements BalanceProvider using the database.
type DbBalanceProvider struct {
	pool *pgxpool.Pool
}

// NewDbBalanceProvider creates a database-backed balance provider.
func NewDbBalanceProvider(pool *pgxpool.Pool) *DbBalanceProvider {
	return &DbBalanceProvider{pool: pool}
}

// GetBalance returns the available balance for a user and currency.
func (p *DbBalanceProvider) GetBalance(ctx context.Context, userID string, currency common.Currency) (decimal.Decimal, error) {
	var balanceStr string
	err := p.pool.QueryRow(ctx,
		`SELECT balance FROM accounts WHERE user_id = $1 AND currency = $2`,
		userID, string(currency),
	).Scan(&balanceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get balance: %w", err)
	}
	balance, err := decimal.NewFromString(balanceStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse balance: %w", err)
	}
	return balance, nil
}

// FreezeBalance deducts from available balance and adds to frozen balance.
func (p *DbBalanceProvider) FreezeBalance(ctx context.Context, userID string, currency common.Currency, amount decimal.Decimal) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE accounts SET balance = balance - $1, frozen_balance = frozen_balance + $1, updated_at = NOW()
		 WHERE user_id = $2 AND currency = $3 AND balance >= $1`,
		amount.String(), userID, string(currency),
	)
	if err != nil {
		return fmt.Errorf("freeze balance: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return common.ErrInsufficientBalance
	}
	return nil
}

// UnfreezeBalance returns frozen balance to available balance.
func (p *DbBalanceProvider) UnfreezeBalance(ctx context.Context, userID string, currency common.Currency, amount decimal.Decimal) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE accounts SET balance = balance + $1, frozen_balance = frozen_balance - $1, updated_at = NOW()
		 WHERE user_id = $2 AND currency = $3 AND frozen_balance >= $1`,
		amount.String(), userID, string(currency),
	)
	return err
}

// GetFrozenBalance returns the frozen balance for a user and currency.
func (p *DbBalanceProvider) GetFrozenBalance(ctx context.Context, userID string, currency common.Currency) (decimal.Decimal, error) {
	var frozenStr string
	err := p.pool.QueryRow(ctx,
		`SELECT COALESCE(frozen_balance, '0') FROM accounts WHERE user_id = $1 AND currency = $2`,
		userID, string(currency),
	).Scan(&frozenStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get frozen balance: %w", err)
	}
	frozen, err := decimal.NewFromString(frozenStr)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse frozen balance: %w", err)
	}
	return frozen, nil
}
