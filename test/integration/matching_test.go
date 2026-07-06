package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
)

// TestEndToEndMatchingFlow tests the complete order flow:
// Place resting orders → submit taker → verify matches and book state
func TestEndToEndMatchingFlow(t *testing.T) {
	eventBus := events.NewMemoryEventBus()
	engine := matching.NewEngine(eventBus)

	err := engine.AddSymbol("ETH-USDT")
	if err != nil {
		t.Fatalf("add symbol: %v", err)
	}

	ctx := context.Background()

	// Step 1: Place resting sell orders (makers)
	sell1 := makeEngineOrder("s1", "alice", common.SideSell, common.OrderTypeLimit,
		common.TIF_GTC, "2000.00", "1.5")
	_, remainder1, err := engine.PlaceOrder(ctx, sell1)
	if err != nil {
		t.Fatalf("place sell1: %v", err)
	}
	if remainder1 == nil {
		t.Fatal("sell1 should remain in book")
	}

	sell2 := makeEngineOrder("s2", "bob", common.SideSell, common.OrderTypeLimit,
		common.TIF_GTC, "2005.00", "2.0")
	_, remainder2, err := engine.PlaceOrder(ctx, sell2)
	if err != nil {
		t.Fatalf("place sell2: %v", err)
	}
	if remainder2 == nil {
		t.Fatal("sell2 should remain in book")
	}

	// Step 2: Verify order book state
	snap := mustGetSnapshot(t, engine, "ETH-USDT")
	if len(snap.Asks) != 2 {
		t.Fatalf("expected 2 ask levels, got %d", len(snap.Asks))
	}
	if snap.Asks[0].Price != "2000" {
		t.Errorf("best ask should be 2000, got %s", snap.Asks[0].Price)
	}

	// Step 3: Place buy order (taker) - should match both partially
	buy1 := makeEngineOrder("b1", "charlie", common.SideBuy, common.OrderTypeLimit,
		common.TIF_GTC, "2005.00", "2.5")
	matches, remainder, err := engine.PlaceOrder(ctx, buy1)
	if err != nil {
		t.Fatalf("place buy1: %v", err)
	}

	// Should have 2 matches
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// Match 1: 1.5 at 2000 (alice's price)
	if matches[0].Quantity.String() != "1.5" {
		t.Errorf("match1 qty: expected 1.5, got %s", matches[0].Quantity.String())
	}
	if matches[0].Price.String() != "2000" {
		t.Errorf("match1 price: expected 2000, got %s", matches[0].Price.String())
	}

	// Match 2: 1.0 at 2005 (bob's price)
	if matches[1].Quantity.String() != "1" {
		t.Errorf("match2 qty: expected 1.0, got %s", matches[1].Quantity.String())
	}
	if matches[1].Price.String() != "2005" {
		t.Errorf("match2 price: expected 2005, got %s", matches[1].Price.String())
	}

	// Charlie's buy should be fully filled (2.5 total)
	if remainder != nil && !remainder.IsFilled() {
		t.Errorf("buy should be filled, got remaining: %s", remainder.FilledQty.String())
	}

	// Step 4: Verify final book state - bob still has 1.0 at 2005
	snap = mustGetSnapshot(t, engine, "ETH-USDT")
	if len(snap.Asks) != 1 {
		t.Fatalf("expected 1 ask level remaining, got %d", len(snap.Asks))
	}
	if snap.Asks[0].Volume != "1" {
		t.Errorf("expected 1.0 volume at 2005, got %s", snap.Asks[0].Volume)
	}

	// Step 5: Cancel bob's remaining order
	cancelled, err := engine.CancelOrder(ctx, "ETH-USDT", "s2")
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if cancelled == nil || cancelled.OrderID != "s2" {
		t.Error("should have cancelled s2")
	}

	// Verify book is now empty
	snap = mustGetSnapshot(t, engine, "ETH-USDT")
	if len(snap.Asks) != 0 {
		t.Errorf("expected 0 ask levels, got %d", len(snap.Asks))
	}
}

func TestMarketOrderEndToEnd(t *testing.T) {
	eventBus := events.NewMemoryEventBus()
	engine := matching.NewEngine(eventBus)
	engine.AddSymbol("ETH-USDT")

	ctx := context.Background()

	// Place resting asks
	engine.PlaceOrder(ctx, makeEngineOrder("a1", "alice", common.SideSell,
		common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	engine.PlaceOrder(ctx, makeEngineOrder("a2", "bob", common.SideSell,
		common.OrderTypeLimit, common.TIF_GTC, "2005", "2.0"))

	// Market buy - should fill best asks
	buy := makeEngineOrder("mb1", "charlie", common.SideBuy,
		common.OrderTypeMarket, "", "", "1.5")

	matches, remaining, err := engine.PlaceOrder(ctx, buy)
	if err != nil {
		t.Fatalf("place market buy: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected matches")
	}
	if remaining != nil {
		t.Error("market order should not have remaining")
	}
}

func TestFOKOrder(t *testing.T) {
	eventBus := events.NewMemoryEventBus()
	engine := matching.NewEngine(eventBus)
	engine.AddSymbol("ETH-USDT")

	ctx := context.Background()

	// Place resting ask
	engine.PlaceOrder(ctx, makeEngineOrder("a1", "alice", common.SideSell,
		common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))

	// FOK buy for 2.0 when only 1.0 available - should be rejected
	fok := makeEngineOrder("fok1", "charlie", common.SideBuy,
		common.OrderTypeLimit, common.TIF_FOK, "2000", "2.0")

	matches, remaining, err := engine.PlaceOrder(ctx, fok)
	if err != nil {
		t.Fatalf("place fok: %v", err)
	}

	// FOK that can't be filled should produce no matches
	if len(matches) > 0 {
		t.Errorf("FOK should not produce matches, got %d", len(matches))
	}
	// The remaining should be rejected
	if remaining == nil || remaining.Status != common.OrderStatusRejected {
		t.Errorf("FOK should be rejected, status=%v", func() string {
			if remaining == nil {
				return "nil"
			}
			return string(remaining.Status)
		}())
	}

	// Alice's order should still be in the book
	snap := mustGetSnapshot(t, engine, "ETH-USDT")
	if len(snap.Asks) != 1 {
		t.Errorf("alice's order should remain, got %d asks", len(snap.Asks))
	}
}

func TestConcurrentOrders(t *testing.T) {
	eventBus := events.NewMemoryEventBus()
	engine := matching.NewEngine(eventBus)
	engine.AddSymbol("ETH-USDT")

	ctx := context.Background()

	done := make(chan bool, 20)

	// Concurrent sellers
	for i := 0; i < 10; i++ {
		go func(idx int) {
			order := makeEngineOrder(
				fmt.Sprintf("s%d", idx), "seller", common.SideSell,
				common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0",
			)
			engine.PlaceOrder(ctx, order)
			done <- true
		}(i)
	}

	// Concurrent buyers
	for i := 0; i < 10; i++ {
		go func(idx int) {
			order := makeEngineOrder(
				fmt.Sprintf("b%d", idx), "buyer", common.SideBuy,
				common.OrderTypeMarket, "", "", "1.0",
			)
			engine.PlaceOrder(ctx, order)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 20; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent orders")
		}
	}

	// Verify no panic, all orders processed sequentially via shard
	snap := mustGetSnapshot(t, engine, "ETH-USDT")
	t.Logf("Final book state: %d bids, %d asks", len(snap.Bids), len(snap.Asks))
}

// Helpers

func makeEngineOrder(id, userID string, side common.Side, orderType common.OrderType, tif common.TimeInForce, price, qty string) *matching.Order {
	dPrice, _ := decimal.NewFromString(price)
	dQty, _ := decimal.NewFromString(qty)
	return &matching.Order{
		OrderID:     id,
		UserID:      userID,
		Symbol:      "ETH-USDT",
		Side:        side,
		Type:        orderType,
		TimeInForce: tif,
		Price:       dPrice,
		Quantity:    dQty,
		Timestamp:   time.Now().UnixNano(),
		Status:      common.OrderStatusCreated,
	}
}

func mustGetSnapshot(t *testing.T, engine *matching.Engine, symbol string) *matching.BookSnapshot {
	t.Helper()
	snap, err := engine.GetOrderBook(common.Symbol(symbol), 100)
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	return snap
}
