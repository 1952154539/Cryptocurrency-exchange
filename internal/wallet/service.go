package wallet

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles wallet operations: deposits, withdrawals, address generation.
type Service struct {
	pool      *pgxpool.Pool
	eventBus  events.EventBus
	hotWallet *HotWallet
	masterKey *ExtendedKey
	addrIdx   map[string]uint32 // next address index per user+currency+chain
	idxMu     sync.Mutex
}

// HotWallet manages online signing keys for withdrawals.
type HotWallet struct {
	balanceCap map[common.Currency]decimal.Decimal
}

// NewService creates a wallet service.
func NewService(pool *pgxpool.Pool, eventBus events.EventBus) *Service {
	return &Service{
		pool:      pool,
		eventBus:  eventBus,
		addrIdx:   make(map[string]uint32),
		hotWallet: &HotWallet{
			balanceCap: map[common.Currency]decimal.Decimal{
				"ETH":  decimalMust("500"),
				"USDT": decimalMust("1000000"),
			},
		},
	}
}

// LoadMasterSeed initializes the HD wallet from a hex-encoded seed.
func (s *Service) LoadMasterSeed(seedHex string) error {
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		return fmt.Errorf("decode seed hex: %w", err)
	}
	mk, err := MasterKeyFromSeed(seed)
	if err != nil {
		return fmt.Errorf("derive master key: %w", err)
	}
	s.masterKey = mk
	log.Info().Msg("HD wallet master key loaded")
	return nil
}

// LoadOrGenerateMasterSeed loads the seed from env, or generates a random one for dev.
func (s *Service) LoadOrGenerateMasterSeed(seedHex string) error {
	if seedHex == "" {
		log.Warn().Msg("no WALLET_MASTER_SEED_HEX set, deposit addresses will use random keys (NOT for production)")
		return nil
	}
	return s.LoadMasterSeed(seedHex)
}

func decimalMust(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

// GenerateDepositAddress creates a new deposit address for a user.
func (s *Service) GenerateDepositAddress(ctx context.Context, userID string, currency common.Currency, chain common.Chain) (string, error) {
	// Check existing address
	var existing string
	err := s.pool.QueryRow(ctx,
		`SELECT address FROM deposit_addresses WHERE user_id = $1 AND currency = $2 AND chain = $3`,
		userID, string(currency), string(chain),
	).Scan(&existing)

	if err == nil {
		return existing, nil
	}

	// Generate new address
	var address string

	var derivationPath string
	if s.masterKey != nil {
		// Derive deterministically from master seed using BIP44
		s.idxMu.Lock()
		idxKey := fmt.Sprintf("%s:%s:%s", userID, currency, chain)
		index := s.addrIdx[idxKey]
		s.addrIdx[idxKey] = index + 1
		s.idxMu.Unlock()

		var err error
		address, _, derivationPath, err = DeriveBIP44AddressForUser(s.masterKey, userID, index)
		if err != nil {
			return "", fmt.Errorf("derive bip44: %w", err)
		}
	} else {
		// Fallback: generate random key (dev mode without seed)
		mk, err := MasterKeyFromSeed(randomSeed())
		if err != nil {
			return "", fmt.Errorf("generate fallback key: %w", err)
		}
		address = PublicAddress(mk.Key)
		derivationPath = "random"
	}

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

// randomSeed generates a 32-byte random seed for dev fallback.
func randomSeed() []byte {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return b
}

// ProcessDeposit handles a detected on-chain deposit.
func (s *Service) ProcessDeposit(ctx context.Context, deposit *DepositEvent) error {
	var userID string
	err := s.pool.QueryRow(ctx,
		`SELECT user_id FROM deposit_addresses WHERE address = $1 AND chain = $2`,
		deposit.ToAddress, deposit.Chain,
	).Scan(&userID)
	if err != nil {
		return fmt.Errorf("unknown deposit address %s: %w", deposit.ToAddress, err)
	}

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

	ct, err := tx.Exec(ctx,
		`UPDATE deposits SET status = 'confirmed', credited_at = NOW() WHERE tx_hash = $1 AND to_address = $2 AND status = 'pending'`,
		txHash, toAddress,
	)
	if err != nil {
		return fmt.Errorf("update deposit status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return nil
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance, frozen_balance, version)
		 VALUES ($1, $2, $3, '0', 1)
		 ON CONFLICT (user_id, currency) DO UPDATE
		 SET balance = accounts.balance + EXCLUDED.balance, updated_at = NOW()`,
		userID, currency, amount.String(),
	)
	if err != nil {
		return fmt.Errorf("credit balance: %w", err)
	}

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

	s.emitDepositConfirmed(ctx, userID, currency, amount, txHash)

	return nil
}

// RequestWithdrawal initiates a withdrawal within a database transaction.
func (s *Service) RequestWithdrawal(ctx context.Context, req *WithdrawalRequest) (*WithdrawalResult, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var balance decimal.Decimal
	err = tx.QueryRow(ctx,
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

	_, err = tx.Exec(ctx,
		`UPDATE accounts SET balance = balance - $1, updated_at = NOW() WHERE user_id = $2 AND currency = $3`,
		total.String(), req.UserID, req.Currency,
	)
	if err != nil {
		return nil, fmt.Errorf("debit balance: %w", err)
	}

	withdrawalID := common.NewWithdrawalID()
	walletType := "hot"
	if req.Amount.Cmp(s.hotWallet.balanceCap[common.Currency(req.Currency)]) > 0 {
		walletType = "cold"
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawals (withdrawal_id, user_id, currency, chain, to_address, amount, fee, wallet_type, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'pending')`,
		withdrawalID, req.UserID, req.Currency, req.Chain,
		req.ToAddress, req.Amount.String(), req.Fee.String(), walletType,
	)
	if err != nil {
		return nil, fmt.Errorf("insert withdrawal: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit withdrawal: %w", err)
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
