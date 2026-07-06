package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rs/zerolog/log"
)

// Writer handles ClickHouse data ingestion.
type Writer struct {
	conn clickhouse.Conn
}

// NewWriter creates a ClickHouse writer connected to the given DSN.
func NewWriter(ctx context.Context, dsn string) (*Writer, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{dsn},
		Auth: clickhouse.Auth{Database: "exchange"},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse connect: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	log.Info().Str("dsn", dsn).Msg("ClickHouse connected")

	if err := initTables(ctx, conn); err != nil {
		return nil, fmt.Errorf("init tables: %w", err)
	}
	return &Writer{conn: conn}, nil
}

func initTables(ctx context.Context, conn clickhouse.Conn) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS trades (
			trade_id String, symbol String, price Decimal(36,18), quantity Decimal(36,18),
			quote_qty Decimal(36,18), maker_order_id String, taker_order_id String,
			maker_user_id String, taker_user_id String, maker_fee Decimal(36,18),
			taker_fee Decimal(36,18), taker_side String, executed_at DateTime64(9),
			ingested_at DateTime DEFAULT now()
		) ENGINE = MergeTree() ORDER BY (symbol, executed_at)
		PARTITION BY toYYYYMM(executed_at)
		TTL executed_at + INTERVAL 90 DAY`,

		`CREATE TABLE IF NOT EXISTS klines_1m (
			symbol String, open_time DateTime, open Decimal(36,18), high Decimal(36,18),
			low Decimal(36,18), close Decimal(36,18), volume Decimal(36,18),
			quote_vol Decimal(36,18), trades UInt32
		) ENGINE = MergeTree() ORDER BY (symbol, open_time)
		PARTITION BY toYYYYMM(open_time)`,

		`CREATE TABLE IF NOT EXISTS audit_log (
			id String, user_id String, currency String, type String,
			amount Decimal(36,18), balance_before Decimal(36,18),
			balance_after Decimal(36,18), reference_id String,
			metadata String, created_at DateTime64(9), ingested_at DateTime DEFAULT now()
		) ENGINE = MergeTree() ORDER BY (user_id, created_at)
		PARTITION BY toYYYYMM(created_at)
		TTL created_at + INTERVAL 365 DAY`,
	}
	for _, ddl := range tables {
		if err := conn.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("create table: %w", err)
		}
	}
	log.Info().Msg("ClickHouse tables initialized")
	return nil
}

// InsertTrade writes a trade record to ClickHouse.
func (w *Writer) InsertTrade(ctx context.Context, trade TradeRecord) error {
	return w.conn.Exec(ctx,
		`INSERT INTO trades (trade_id, symbol, price, quantity, quote_qty,
		 maker_order_id, taker_order_id, maker_user_id, taker_user_id,
		 maker_fee, taker_fee, taker_side, executed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trade.TradeID, trade.Symbol, trade.Price, trade.Quantity, trade.QuoteQty,
		trade.MakerOrderID, trade.TakerOrderID, trade.MakerUserID, trade.TakerUserID,
		trade.MakerFee, trade.TakerFee, trade.TakerSide, trade.ExecutedAt,
	)
}

// InsertKline writes a candlestick to ClickHouse.
func (w *Writer) InsertKline(ctx context.Context, k KlineRecord) error {
	return w.conn.Exec(ctx,
		`INSERT INTO klines_1m (symbol, open_time, open, high, low, close, volume, quote_vol, trades)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Symbol, k.OpenTime, k.Open, k.High, k.Low, k.Close, k.Volume, k.QuoteVol, k.Trades,
	)
}

// InsertAudit writes an audit log entry.
func (w *Writer) InsertAudit(ctx context.Context, a AuditRecord) error {
	return w.conn.Exec(ctx,
		`INSERT INTO audit_log (id, user_id, currency, type, amount, balance_before, balance_after, reference_id, metadata, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.UserID, a.Currency, a.Type, a.Amount, a.BalanceBefore, a.BalanceAfter, a.ReferenceID, a.Metadata, a.CreatedAt,
	)
}

// Close shuts down the ClickHouse connection.
func (w *Writer) Close() error { return w.conn.Close() }

type TradeRecord struct {
	TradeID, Symbol, MakerOrderID, TakerOrderID, MakerUserID, TakerUserID, TakerSide string
	Price, Quantity, QuoteQty, MakerFee, TakerFee                                    float64
	ExecutedAt                                                                         time.Time
}

type KlineRecord struct {
	Symbol                      string
	OpenTime                    time.Time
	Open, High, Low, Close, Volume, QuoteVol float64
	Trades                      uint32
}

type AuditRecord struct {
	ID, UserID, Currency, Type, ReferenceID, Metadata string
	Amount, BalanceBefore, BalanceAfter                float64
	CreatedAt                                          time.Time
}
