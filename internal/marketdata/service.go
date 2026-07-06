package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// Service provides market data: order book depth, tickers, candles, trades.
type Service struct {
	redis  *redis.Client
	engine *matching.Engine
}

// NewService creates a market data service.
func NewService(redis *redis.Client, engine *matching.Engine) *Service {
	return &Service{redis: redis, engine: engine}
}

// GetOrderBook returns the current order book depth for a symbol.
func (s *Service) GetOrderBook(ctx context.Context, symbol common.Symbol, depth int) (*matching.BookSnapshot, error) {
	return s.engine.GetOrderBook(symbol, depth)
}

// GetRecentTrades returns the most recent trades for a symbol.
func (s *Service) GetRecentTrades(ctx context.Context, symbol common.Symbol, limit int) ([]TradeItem, error) {
	key := fmt.Sprintf("trades:%s", symbol)
	trades, err := s.redis.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}

	items := make([]TradeItem, 0, len(trades))
	for _, t := range trades {
		var item TradeItem
		if err := json.Unmarshal([]byte(t), &item); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal trade from redis")
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

// GetTicker returns 24h ticker statistics for a symbol.
func (s *Service) GetTicker(ctx context.Context, symbol common.Symbol) (*Ticker, error) {
	key := fmt.Sprintf("ticker:%s", symbol)
	data, err := s.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return &Ticker{Symbol: string(symbol)}, nil
	}
	return &Ticker{
		Symbol:    string(symbol),
		Open:      data["open"],
		High:      data["high"],
		Low:       data["low"],
		Last:      data["last"],
		Volume:    data["volume"],
		Change:    data["change"],
		ChangePct: data["change_pct"],
		Count:     data["count"],
	}, nil
}

// GetKlines returns candlestick/k-line data.
func (s *Service) GetKlines(ctx context.Context, symbol common.Symbol, interval string, limit int) ([]Candlestick, error) {
	return []Candlestick{}, nil
}

// RecordTrade persists a trade to Redis for public market data queries.
func (s *Service) RecordTrade(ctx context.Context, match *matching.MatchResult) error {
	item := TradeItem{
		ID:           common.NewTradeID(),
		Price:        match.Price.String(),
		Quantity:     match.Quantity.String(),
		QuoteQty:     match.QuoteQty.String(),
		Time:         time.Unix(0, match.Timestamp),
		IsBuyerMaker: match.TakerSide == common.SideSell,
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal trade: %w", err)
	}

	key := fmt.Sprintf("trades:%s", match.Symbol)
	pipe := s.redis.Pipeline()
	pipe.LPush(ctx, key, string(data))
	pipe.LTrim(ctx, key, 0, 999) // keep last 1000 trades
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("redis push trade: %w", err)
	}

	return nil
}

// Start subscribes to trade events and records them to Redis.
func (s *Service) Start(ctx context.Context, eventBus events.EventBus) error {
	log.Info().Msg("market data service starting")
	if eventBus == nil {
		log.Warn().Msg("no event bus configured, market data will not be populated")
		return nil
	}

	return eventBus.Subscribe(ctx, "trade.executed", "marketdata", func(ctx context.Context, evt *events.Event) error {
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
			Timestamp:    evt.Timestamp.UnixNano(),
		}
		return s.RecordTrade(ctx, match)
	})
}

// TradeItem represents a single trade.
type TradeItem struct {
	ID           string    `json:"id"`
	Price        string    `json:"price"`
	Quantity     string    `json:"qty"`
	QuoteQty     string    `json:"quoteQty"`
	Time         time.Time `json:"time"`
	IsBuyerMaker bool      `json:"isBuyerMaker"`
}

// Ticker represents 24-hour rolling statistics.
type Ticker struct {
	Symbol    string `json:"symbol"`
	Open      string `json:"openPrice"`
	High      string `json:"highPrice"`
	Low       string `json:"lowPrice"`
	Last      string `json:"lastPrice"`
	Volume    string `json:"volume"`
	Change    string `json:"priceChange"`
	ChangePct string `json:"priceChangePercent"`
	Count     string `json:"count"`
}

// Candlestick represents a single K-line/candlestick.
type Candlestick struct {
	OpenTime  int64  `json:"openTime"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	CloseTime int64  `json:"closeTime"`
	QuoteVol  string `json:"quoteAssetVolume"`
	Trades    int    `json:"trades"`
}
