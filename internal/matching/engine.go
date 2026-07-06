package matching

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/rs/zerolog/log"
)

// Engine is the sharded matching engine. Each trading pair gets its own shard
// (single goroutine), so there is no lock contention within a shard.
type Engine struct {
	shards   map[common.Symbol]*shard
	mu       sync.RWMutex
	eventBus events.EventBus
}

// shard represents a single trading pair's order book and processing loop.
type shard struct {
	symbol    common.Symbol
	book      *OrderBook
	ordersIn  chan *shardOrder
	stopCh    chan struct{}
	eventBus  events.EventBus
}

type shardOrder struct {
	order     *Order
	resultCh  chan shardResult
}

type shardResult struct {
	matches   []*MatchResult
	remaining *Order
	cancelled *Order
	err       error
}

// NewEngine creates a new matching engine.
func NewEngine(eventBus events.EventBus) *Engine {
	return &Engine{
		shards:   make(map[common.Symbol]*shard),
		eventBus: eventBus,
	}
}

// AddSymbol registers a new trading pair and starts its shard goroutine.
func (e *Engine) AddSymbol(symbol common.Symbol) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.shards[symbol]; exists {
		return fmt.Errorf("symbol %s already registered", symbol)
	}

	s := &shard{
		symbol:   symbol,
		book:     NewOrderBook(symbol),
		ordersIn: make(chan *shardOrder, 1024),
		stopCh:   make(chan struct{}),
		eventBus: e.eventBus,
	}

	e.shards[symbol] = s
	go s.run()
	log.Info().Str("symbol", string(symbol)).Msg("matching engine shard started")
	return nil
}

// RemoveSymbol stops and removes a trading pair.
func (e *Engine) RemoveSymbol(symbol common.Symbol) error {
	e.mu.Lock()
	s, exists := e.shards[symbol]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("symbol %s not found", symbol)
	}
	delete(e.shards, symbol)
	e.mu.Unlock()

	close(s.stopCh)
	log.Info().Str("symbol", string(symbol)).Msg("matching engine shard stopped")
	return nil
}

// PlaceOrder submits an order to the appropriate shard and waits for the result.
func (e *Engine) PlaceOrder(ctx context.Context, order *Order) ([]*MatchResult, *Order, error) {
	e.mu.RLock()
	s, exists := e.shards[order.Symbol]
	e.mu.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("symbol %s not found", order.Symbol)
	}

	so := &shardOrder{
		order:    order,
		resultCh: make(chan shardResult, 1),
	}

	select {
	case s.ordersIn <- so:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	select {
	case result := <-so.resultCh:
		return result.matches, result.remaining, result.err
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// CancelOrder removes an order from the book.
func (e *Engine) CancelOrder(ctx context.Context, symbol common.Symbol, orderID string) (*Order, error) {
	e.mu.RLock()
	s, exists := e.shards[symbol]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	// For cancel, we use a special cancel command through the order channel
	so := &shardOrder{
		order: &Order{
			OrderID: orderID,
			Symbol:  symbol,
			Side:    "cancel",
		},
		resultCh: make(chan shardResult, 1),
	}

	select {
	case s.ordersIn <- so:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case result := <-so.resultCh:
		if result.cancelled == nil {
			return nil, common.ErrOrderNotFound
		}
		return result.cancelled, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// GetOrderBook returns the order book snapshot for a symbol.
func (e *Engine) GetOrderBook(symbol common.Symbol, depth int) (*BookSnapshot, error) {
	e.mu.RLock()
	s, exists := e.shards[symbol]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("symbol %s not found", symbol)
	}

	return s.book.Snapshot(depth), nil
}

// GetOrder retrieves an order by ID from the book.
func (e *Engine) GetOrder(symbol common.Symbol, orderID string) *Order {
	e.mu.RLock()
	s, exists := e.shards[symbol]
	e.mu.RUnlock()

	if !exists {
		return nil
	}
	return s.book.GetOrder(orderID)
}

// Start is a no-op; shards start when AddSymbol is called.
func (e *Engine) Start(ctx context.Context) error {
	log.Info().Msg("matching engine starting")
	return nil
}

// Stop gracefully shuts down all shards.
func (e *Engine) Stop(ctx context.Context) error {
	log.Info().Msg("matching engine shutting down")

	e.mu.Lock()
	for symbol, s := range e.shards {
		close(s.stopCh)
		delete(e.shards, symbol)
	}
	e.mu.Unlock()

	return nil
}

// run is the main event loop for a single shard.
func (s *shard) run() {
	for {
		select {
		case so := <-s.ordersIn:
			s.processOrder(so)
		case <-s.stopCh:
			return
		}
	}
}

// processOrder handles a single order within the shard's goroutine.
func (s *shard) processOrder(so *shardOrder) {
	order := so.order

	// Handle cancellation
	if order.Side == "cancel" {
		cancelled := s.book.CancelOrder(order.OrderID)
		so.resultCh <- shardResult{cancelled: cancelled}
		if cancelled != nil {
			s.emitOrderCancelled(cancelled)
		}
		return
	}

	// FOK check: if order can't be fully filled, reject immediately
	if order.TimeInForce == common.TIF_FOK && !s.book.CanFillFOK(order) {
		order.Status = common.OrderStatusRejected
		so.resultCh <- shardResult{remaining: order}
		s.emitOrderRejected(order)
		return
	}

	// Market order with no liquidity
	if order.Type == common.OrderTypeMarket {
		if order.Side == common.SideBuy && s.book.BestAsk().IsZero() {
			order.Status = common.OrderStatusRejected
			so.resultCh <- shardResult{remaining: order}
			s.emitOrderRejected(order)
			return
		}
		if order.Side == common.SideSell && s.book.BestBid().IsZero() {
			order.Status = common.OrderStatusRejected
			so.resultCh <- shardResult{remaining: order}
			s.emitOrderRejected(order)
			return
		}
	}

	start := time.Now()
	matches, remaining := s.book.MatchOrder(order)
	latency := time.Since(start).Seconds()

	log.Debug().
		Str("symbol", string(s.symbol)).
		Str("order_id", order.OrderID).
		Int("matches", len(matches)).
		Float64("latency_us", latency*1e6).
		Msg("order matched")

	// Emit trade events
	for _, m := range matches {
		s.emitTradeExecuted(m)
	}

	// Emit order events
	if len(matches) > 0 {
		if remaining == nil || remaining.Status == common.OrderStatusFilled {
			s.emitOrderFilled(order, matches)
		} else {
			s.emitOrderPartiallyFilled(order, matches)
		}
	}

	// Add remaining to book (GTC limit orders)
	if remaining != nil && remaining.Status != common.OrderStatusFilled &&
		remaining.Status != common.OrderStatusCancelled &&
		remaining.Status != common.OrderStatusRejected {
		s.addToBook(remaining)
		s.emitOrderPlaced(remaining)
	}

	so.resultCh <- shardResult{
		matches:   matches,
		remaining: remaining,
	}
}

// addToBook adds a resting limit order to the appropriate side.
func (s *shard) addToBook(order *Order) {
	if order.Side == common.SideBuy {
		s.book.addBid(order)
	} else {
		s.book.addAsk(order)
	}
}

// Event emission methods

func (s *shard) emitOrderPlaced(order *Order) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventOrderPlaced,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.OrderPlacedPayload{
		OrderID:     order.OrderID,
		UserID:      order.UserID,
		Symbol:      string(order.Symbol),
		Side:        string(order.Side),
		Type:        string(order.Type),
		Price:       order.Price.String(),
		Quantity:    order.Quantity.String(),
		FilledQty:   order.FilledQty.String(),
		TimeInForce: string(order.TimeInForce),
		Status:      string(order.Status),
	})
	_ = s.eventBus.Publish(context.Background(), "order.placed", evt)
}

func (s *shard) emitTradeExecuted(m *MatchResult) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventTradeExecuted,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.TradeExecutedPayload{
		TradeID:      "", // filled by settlement service
		TakerOrderID: m.TakerOrderID,
		MakerOrderID: m.MakerOrderID,
		TakerUserID:  m.TakerUserID,
		MakerUserID:  m.MakerUserID,
		Symbol:       string(m.Symbol),
		Price:        m.Price.String(),
		Quantity:     m.Quantity.String(),
		QuoteQty:     m.QuoteQty.String(),
		TakerSide:    string(m.TakerSide),
	})
	_ = s.eventBus.Publish(context.Background(), "trade.executed", evt)
}

func (s *shard) emitOrderFilled(order *Order, matches []*MatchResult) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventOrderMatched,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.OrderMatchedPayload{
		OrderID:  order.OrderID,
		UserID:   order.UserID,
		Symbol:   string(order.Symbol),
		Side:     string(order.Side),
		Status:   "filled",
	})
	_ = s.eventBus.Publish(context.Background(), "order.matched", evt)
}

func (s *shard) emitOrderPartiallyFilled(order *Order, matches []*MatchResult) {
	// Same event type with partial fill status
}

func (s *shard) emitOrderCancelled(order *Order) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventOrderCancelled,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.OrderCancelledPayload{
		OrderID: order.OrderID,
		UserID:  order.UserID,
		Symbol:  string(order.Symbol),
	})
	_ = s.eventBus.Publish(context.Background(), "order.cancelled", evt)
}

func (s *shard) emitOrderRejected(order *Order) {
	if s.eventBus == nil {
		return
	}
	evt := &events.Event{
		ID:        events.NewEventID(),
		Type:      events.EventOrderPlaced,
		Timestamp: time.Now(),
	}
	evt.SetPayload(events.OrderPlacedPayload{
		OrderID:  order.OrderID,
		UserID:   order.UserID,
		Symbol:   string(order.Symbol),
		Status:   "rejected",
	})
	_ = s.eventBus.Publish(context.Background(), "order.placed", evt)
}

// Ensure decimal import is used (for future extensions)
var _ = decimal.Zero
