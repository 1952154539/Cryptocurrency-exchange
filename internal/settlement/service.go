package settlement

import (
	"context"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Service handles post-trade settlement: balance updates and fee collection.
type Service struct {
	pool     *pgxpool.Pool
	feeSvc   *FeeService
	eventBus events.EventBus
}

// NewService creates a settlement service.
func NewService(pool *pgxpool.Pool, feeSvc *FeeService, eventBus events.EventBus) *Service {
	return &Service{
		pool:     pool,
		feeSvc:   feeSvc,
		eventBus: eventBus,
	}
}

// SettleTrade processes a single trade: updates balances, calculates fees.
func (s *Service) SettleTrade(ctx context.Context, match *matching.MatchResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tradeID := fmt.Sprintf("trd_%s_%s", match.MakerOrderID, match.TakerOrderID)
	symbol := common.Symbol(match.Symbol)

	// Calculate fees
	makerFee := s.feeSvc.CalculateFee(ctx, match.MakerUserID, true, match.QuoteQty)
	takerFee := s.feeSvc.CalculateFee(ctx, match.TakerUserID, false, match.QuoteQty)

	var takerBaseDelta, takerQuoteDelta, makerBaseDelta, makerQuoteDelta decimal.Decimal

	if match.TakerSide == common.SideBuy {
		takerBaseDelta = match.Quantity
		takerQuoteDelta = match.QuoteQty.Add(takerFee).Mul(decimal.NewFromInt64(-1))
		makerBaseDelta = match.Quantity.Mul(decimal.NewFromInt64(-1))
		makerQuoteDelta = match.QuoteQty.Sub(makerFee)
	} else {
		takerBaseDelta = match.Quantity.Mul(decimal.NewFromInt64(-1))
		takerQuoteDelta = match.QuoteQty.Sub(takerFee)
		makerBaseDelta = match.Quantity
		makerQuoteDelta = match.QuoteQty.Add(makerFee).Mul(decimal.NewFromInt64(-1))
	}

	// Update balances with optimistic locking
	if err := s.updateBalance(ctx, tx, match.TakerUserID, symbol.Base(), takerBaseDelta); err != nil {
		return fmt.Errorf("update taker base balance: %w", err)
	}
	if err := s.updateBalance(ctx, tx, match.TakerUserID, symbol.Quote(), takerQuoteDelta); err != nil {
		return fmt.Errorf("update taker quote balance: %w", err)
	}
	if err := s.updateBalance(ctx, tx, match.MakerUserID, symbol.Base(), makerBaseDelta); err != nil {
		return fmt.Errorf("update maker base balance: %w", err)
	}
	if err := s.updateBalance(ctx, tx, match.MakerUserID, symbol.Quote(), makerQuoteDelta); err != nil {
		return fmt.Errorf("update maker quote balance: %w", err)
	}

	// Collect fees to exchange revenue account
	feeAccountID := "00000000-0000-0000-0000-000000000000" // exchange fee collection account
	totalFees := makerFee.Add(takerFee)
	if err := s.updateBalance(ctx, tx, feeAccountID, symbol.Quote(), totalFees); err != nil {
		return fmt.Errorf("collect exchange fees: %w", err)
	}

	// Record trade
	if err := s.insertTrade(ctx, tx, tradeID, match, makerFee, takerFee); err != nil {
		return fmt.Errorf("insert trade: %w", err)
	}

	// Commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit settlement: %w", err)
	}

	// Emit events
	s.emitBalanceChanged(ctx, match.MakerUserID, symbol.Base(), makerBaseDelta, tradeID)
	s.emitBalanceChanged(ctx, match.MakerUserID, symbol.Quote(), makerQuoteDelta, tradeID)
	s.emitBalanceChanged(ctx, match.TakerUserID, symbol.Base(), takerBaseDelta, tradeID)
	s.emitBalanceChanged(ctx, match.TakerUserID, symbol.Quote(), takerQuoteDelta, tradeID)

	log.Info().
		Str("trade_id", tradeID).
		Str("symbol", string(match.Symbol)).
		Str("price", match.Price.String()).
		Str("quantity", match.Quantity.String()).
		Str("maker_fee", makerFee.String()).
		Str("taker_fee", takerFee.String()).
		Msg("trade settled")

	return nil
}

// updateBalance atomically updates a user's balance with optimistic locking.
func (s *Service) updateBalance(ctx context.Context, tx pgx.Tx, userID string, currency common.Currency, delta decimal.Decimal) error {
	var currentBalance decimal.Decimal
	var currentVersion int64

	// INSERT first to handle missing accounts, then SELECT FOR UPDATE atomically.
	_, _ = tx.Exec(ctx,
		`INSERT INTO accounts (user_id, currency, balance, frozen_balance, version) VALUES ($1, $2, '0', '0', 1)
		 ON CONFLICT (user_id, currency) DO NOTHING`,
		userID, string(currency))

	query := `SELECT balance, version FROM accounts WHERE user_id = $1 AND currency = $2 FOR UPDATE`
	err := tx.QueryRow(ctx, query, userID, string(currency)).Scan(&currentBalance, &currentVersion)
	if err != nil {
		return fmt.Errorf("get balance for %s/%s: %w", userID, currency, err)
	}

	newBalance := currentBalance.Add(delta)
	if newBalance.IsNegative() {
		return fmt.Errorf("%w: would go negative (%s → %s)", common.ErrInsufficientBalance, currentBalance.String(), newBalance.String())
	}

	updateQuery := `UPDATE accounts SET balance = $1, frozen_balance = GREATEST(frozen_balance + $5, 0), version = version + 1, updated_at = NOW() WHERE user_id = $2 AND currency = $3 AND version = $4`
		frozenAdjust := decimal.Zero
		if delta.IsNegative() {
			frozenAdjust = delta // negative, reduces frozen_balance
		}
	ct, err := tx.Exec(ctx, updateQuery, newBalance.String(), userID, string(currency), currentVersion, frozenAdjust.String())
	if err != nil {
		return fmt.Errorf("update balance: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("balance update conflict for %s/%s", userID, currency)
	}

	auditQuery := `INSERT INTO balance_transactions (user_id, currency, type, amount, balance_before, balance_after) VALUES ($1, $2, 'trade_fill', $3, $4, $5)`
	if _, err := tx.Exec(ctx, auditQuery, userID, string(currency), delta.String(), currentBalance.String(), newBalance.String()); err != nil {
			log.Warn().Err(err).Str("user_id", userID).Str("currency", string(currency)).Msg("audit log insert failed")
		}

	return nil
}

// insertTrade records a trade in the database.
func (s *Service) insertTrade(ctx context.Context, tx pgx.Tx, tradeID string, match *matching.MatchResult, makerFee, takerFee decimal.Decimal) error {
	query := `
		INSERT INTO trades (trade_id, symbol, maker_order_id, taker_order_id, maker_user_id, taker_user_id, price, quantity, quote_quantity, maker_fee, taker_fee, side, executed_at)
			ON CONFLICT (trade_id) DO NOTHING
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := tx.Exec(ctx, query,
		tradeID, string(match.Symbol), match.MakerOrderID, match.TakerOrderID,
		match.MakerUserID, match.TakerUserID,
		match.Price.String(), match.Quantity.String(), match.QuoteQty.String(),
		makerFee.String(), takerFee.String(), string(match.TakerSide),
		time.Unix(0, match.Timestamp),
	)
	return err
}

// emitBalanceChanged publishes a balance change event.
func (s *Service) emitBalanceChanged(ctx context.Context, userID string, currency common.Currency, delta decimal.Decimal, tradeID string) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventBalanceChanged,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.BalanceChangedPayload{
		UserID:    userID,
		Currency:  string(currency),
		Amount:    delta.String(),
		Reason:    "trade",
		Reference: tradeID,
	})
	_ = s.eventBus.Publish(ctx, "balance.changed", evt)
}

// Start begins consuming trade events from the event bus.
func (s *Service) Start(ctx context.Context) error {
	log.Info().Msg("settlement service starting")
	if s.eventBus == nil {
		log.Warn().Msg("no event bus configured, settlement will not process trades automatically")
		return nil
	}

	return s.eventBus.Subscribe(ctx, "trade.executed", "settlement", func(ctx context.Context, evt *events.Event) error {
		var payload events.TradeExecutedPayload
		if err := evt.GetPayload(&payload); err != nil {
			return fmt.Errorf("unmarshal trade payload: %w", err)
		}

		price, _ := decimal.NewFromString(payload.Price)
		quantity, _ := decimal.NewFromString(payload.Quantity)
		quoteQty, _ := decimal.NewFromString(payload.QuoteQty)

		match := &matching.MatchResult{
			TakerOrderID: payload.TakerOrderID,
			MakerOrderID: payload.MakerOrderID,
			TakerUserID:  payload.TakerUserID,
			MakerUserID:  payload.MakerUserID,
			Symbol:       common.Symbol(payload.Symbol),
			Price:        price,
			Quantity:     quantity,
			QuoteQty:     quoteQty,
			TakerSide:    common.Side(payload.TakerSide),
		}

		return s.SettleTrade(ctx, match)
	})
}
