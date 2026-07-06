package wallet

import (
	"context"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles wallet operations: deposits, withdrawals, address generation.
type Service struct {
	pool     *pgxpool.Pool
	eventBus events.EventBus
	hotWallet  *HotWallet
}

// HotWallet manages online signing keys for withdrawals.
type HotWallet struct {
	balanceCap map[common.Currency]decimal.Decimal // max balance per currency in hot wallet
}

// NewService creates a wallet service.
func NewService(pool *pgxpool.Pool, eventBus events.EventBus) *Service {
	return &Service{
		pool:    pool,
		eventBus: eventBus,
		hotWallet: &HotWallet{
			balanceCap: map[common.Currency]decimal.Decimal{
				"ETH":  decimalMust("500"),
				"USDT": decimalMust("1000000"),
			},
		},
	}
}

func decimalMust(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

// GenerateDepositAddress creates a new deposit address for a user.
func (s *Service) GenerateDepositAddress(ctx context.Context, userID string, currency common.Currency, chain common.Chain) (string, error) {
	// Check if user already has an address for this currency+chain
	var existing string
	err := s.pool.QueryRow(ctx,
		`SELECT address FROM deposit_addresses WHERE user_id = $1 AND currency = $2 AND chain = $3`,
		userID, string(currency), string(chain),
	).Scan(&existing)

	if err == nil {
		return existing, nil
	}

	// Generate new address
	// In production, this uses the HD wallet to derive the next unused address
	masterKey, err := GenerateMasterKey()
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}

	address := PublicAddress(masterKey)
	derivationPath := fmt.Sprintf("m/44'/60'/0'/0/%d", time.Now().UnixNano()%10000)

	_, err = s.pool.Exec(ctx,
		`INSERT INTO deposit_addresses (user_id, currency, chain, address, derivation_path, wallet_type)
		 VALUES ($1, $2, $3, $4, $5, 'hot')`,
		userID, string(currency), string(chain), address, derivationPath,
	)
	if err != nil {
		return "", fmt.Errorf("insert deposit address: %w", err)
	}

	log.Info().
		Str("user_id", userID).
		Str("currency", string(currency)).
		Str("chain", string(chain)).
		Str("address", address).
		Msg("deposit address generated")

	return address, nil
}

// ProcessDeposit handles a detected on-chain deposit.
func (s *Service) ProcessDeposit(ctx context.Context, deposit *DepositEvent) error {
	// Find the user by deposit address
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM deposit_addresses WHERE address = $1 AND chain = $2`,
		deposit.ToAddress, deposit.Chain,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("unknown deposit address %s: %w", deposit.ToAddress, err)
	}

	// Insert deposit record (UNIQUE constraint prevents double-crediting)
	_, err = s.pool.Exec(ctx,
		`INSERT INTO deposits (tx_hash, currency, chain, from_address, to_address, amount, block_number, confirmations, required_confs, status, user_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (tx_hash, to_address) DO NOTHING`,
		deposit.TxHash, deposit.Currency, deposit.Chain,
		deposit.FromAddress, deposit.ToAddress, deposit.Amount.String(),
		deposit.BlockNumber, deposit.Confirmations, deposit.RequiredConfirmations,
		string(common.DepositStatusPending), userID,
	)
	if err != nil {
		return fmt.Errorf("insert deposit: %w", err)
	}

	// If already confirmed, credit user balance
	if deposit.Confirmations >= deposit.RequiredConfirmations {
		return s.confirmDeposit(ctx, deposit.TxHash, deposit.ToAddress, userID, deposit.Amount, deposit.Currency)
	}

	return nil
}

// confirmDeposit credits the user's balance after sufficient confirmations.
func (s *Service) confirmDeposit(ctx context.Context, txHash, toAddress, userID string, amount decimal.Decimal, currency string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Update deposit status
	ct, err := tx.Exec(ctx,
		`UPDATE deposits SET status = 'confirmed', credited_at = NOW() WHERE tx_hash = $1 AND to_address = $2 AND status = 'pending'`,
		txHash, toAddress,
	)
	if err != nil {
		return fmt.Errorf("update deposit status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil // already processed
	}

	// Credit user balance
	_, err = tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = NOW() WHERE user_id = $2 AND currency = $3`,
		amount.String(), userID, currency,
	)
	if err != nil {
		return fmt.Errorf("credit balance: %w", err)
	}

	// Record audit
	_, _ = tx.Exec(ctx,
		`INSERT INTO balance_transactions (user_id, currency, type, amount, balance_before, balance_after, reference_id)
		 VALUES ($1, $2, 'deposit', $3, 0, $3, $4)`,
		userID, currency, amount.String(), txHash,
	)

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit deposit: %w", err)
	}

	log.Info().
		Str("tx_hash", txHash).
		Str("user_id", userID).
		Str("currency", currency).
		Str("amount", amount.String()).
		Msg("deposit confirmed")

	// Emit event
	s.emitDepositConfirmed(ctx, userID, currency, amount, txHash)

	return nil
}

// RequestWithdrawal initiates a withdrawal.
func (s *Service) RequestWithdrawal(ctx context.Context, req *WithdrawalRequest) (*WithdrawalResult, error) {
	// 1. Check balance
	var balance decimal.Decimal
	err := s.pool.QueryRow(ctx,
		`SELECT balance FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`,
		req.UserID, req.Currency,
	).Scan(&balance)
	if err != nil {
		return nil, fmt.Errorf("get balance: %w", err)
	}

	total := req.Amount.Add(req.Fee)
	if balance.Cmp(total) < 0 {
		return nil, common.ErrInsufficientBalance
	}

	// 2. Debit balance
	_, err = s.pool.Exec(ctx,
		`UPDATE accounts SET balance = balance - $1, updated_at = NOW() WHERE user_id = $2 AND currency = $3`,
		total.String(), req.UserID, req.Currency,
	)
	if err != nil {
		return nil, fmt.Errorf("debit balance: %w", err)
	}

	// 3. Create withdrawal record
	withdrawalID := common.NewWithdrawalID()
	walletType := "hot"
	if req.Amount.Cmp(s.hotWallet.balanceCap[common.Currency(req.Currency)]) > 0 {
		walletType = "cold"
	}

	_, err = s.pool.Exec(ctx,
		`INSERT INTO withdrawals (withdrawal_id, user_id, currency, chain, to_address, amount, fee, wallet_type, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')`,
		withdrawalID, req.UserID, req.Currency, req.Chain,
		req.ToAddress, req.Amount.String(), req.Fee.String(), walletType,
	)
	if err != nil {
		return nil, fmt.Errorf("insert withdrawal: %w", err)
	}

	log.Info().
		Str("withdrawal_id", withdrawalID).
		Str("user_id", req.UserID).
		Str("currency", req.Currency).
		Str("amount", req.Amount.String()).
		Str("wallet_type", walletType).
		Msg("withdrawal requested")

	return &WithdrawalResult{
		WithdrawalID: withdrawalID,
		Status:       "pending",
		WalletType:   walletType,
	}, nil
}

// DepositEvent is received from the blockchain monitor.
type DepositEvent struct {
	TxHash                string
	Currency              string
	Chain                 string
	FromAddress           string
	ToAddress             string
	Amount                decimal.Decimal
	BlockNumber           uint64
	Confirmations         uint64
	RequiredConfirmations uint64
}

// WithdrawalRequest represents a withdrawal request.
type WithdrawalRequest struct {
	UserID    string
	Currency  string
	Chain     string
	ToAddress string
	Amount    decimal.Decimal
	Fee       decimal.Decimal
}

// WithdrawalResult contains the result of a withdrawal request.
type WithdrawalResult struct {
	WithdrawalID string
	Status       string
	WalletType   string
	TxHash       string
}

// emitDepositConfirmed publishes a deposit event.
func (s *Service) emitDepositConfirmed(ctx context.Context, userID, currency string, amount decimal.Decimal, txHash string) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventDepositConfirmed,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.BalanceChangedPayload{
		UserID:    userID,
		Currency:  currency,
		Amount:    amount.String(),
		Reason:    "deposit",
		Reference: txHash,
	})
	_ = s.eventBus.Publish(ctx, "deposit.confirmed", evt)
}
