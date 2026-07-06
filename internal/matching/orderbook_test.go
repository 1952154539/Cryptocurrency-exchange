package matching

import (
	"fmt"
	"testing"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
)

func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func makeOrder(id, userID string, side common.Side, orderType common.OrderType, tif common.TimeInForce, price, qty string) *Order {
	return &Order{
		OrderID:     id,
		UserID:      userID,
		Symbol:      "ETH-USDT",
		Side:        side,
		Type:        orderType,
		TimeInForce: tif,
		Price:       mustDecimal(price),
		Quantity:    mustDecimal(qty),
		Timestamp:   time.Now().UnixNano(),
		Status:      common.OrderStatusCreated,
	}
}

func TestOrderBook_AddBid(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addBid(makeOrder("o1", "u1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.5"))
	ob.addBid(makeOrder("o2", "u2", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2005", "2.0"))
	ob.addBid(makeOrder("o3", "u3", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "1995", "3.0"))

	bids, asks := ob.Size()
	if bids != 3 {
		t.Fatalf("expected 3 bid levels, got %d", bids)
	}
	if asks != 0 {
		t.Fatalf("expected 0 ask levels, got %d", asks)
	}

	// Bids should be descending: 2005, 2000, 1995
	if ob.bids[0].price.String() != "2005" {
		t.Errorf("expected best bid 2005, got %s", ob.bids[0].price.String())
	}
	if ob.bids[1].price.String() != "2000" {
		t.Errorf("expected bid 2000, got %s", ob.bids[1].price.String())
	}
	if ob.bids[2].price.String() != "1995" {
		t.Errorf("expected bid 1995, got %s", ob.bids[2].price.String())
	}

	if ob.BestBid().String() != "2005" {
		t.Errorf("BestBid should be 2005, got %s", ob.BestBid().String())
	}
}

func TestOrderBook_AddAsk(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addAsk(makeOrder("o1", "u1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.5"))
	ob.addAsk(makeOrder("o2", "u2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "1995", "2.0"))
	ob.addAsk(makeOrder("o3", "u3", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2005", "3.0"))

	bids, asks := ob.Size()
	if bids != 0 {
		t.Fatalf("expected 0 bid levels, got %d", bids)
	}
	if asks != 3 {
		t.Fatalf("expected 3 ask levels, got %d", asks)
	}

	// Asks should be ascending: 1995, 2000, 2005
	if ob.asks[0].price.String() != "1995" {
		t.Errorf("expected best ask 1995, got %s", ob.asks[0].price.String())
	}
	if ob.asks[2].price.String() != "2005" {
		t.Errorf("expected ask 2005, got %s", ob.asks[2].price.String())
	}

	if ob.BestAsk().String() != "1995" {
		t.Errorf("BestAsk should be 1995, got %s", ob.BestAsk().String())
	}
}

func TestOrderBook_SamePriceFIFO(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	// Two asks at the same price - first should be matched first
	ob.addAsk(makeOrder("o1", "u1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("o2", "u2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "2.0"))

	// Should have 1 price level with 2 orders
	if len(ob.asks) != 1 {
		t.Fatalf("expected 1 price level, got %d", len(ob.asks))
	}
	if ob.asks[0].orders.len != 2 {
		t.Fatalf("expected 2 orders at level, got %d", ob.asks[0].orders.len)
	}

	// First order in queue should be o1
	first := ob.asks[0].orders.head.order
	if first.OrderID != "o1" {
		t.Errorf("expected o1 first, got %s", first.OrderID)
	}

	// Volume should be 3.0
	if ob.asks[0].orders.volume.String() != "3" {
		t.Errorf("expected volume 3, got %s", ob.asks[0].orders.volume.String())
	}
}

func TestMatch_LimitBuyFullyFilled(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	// Add resting sell orders
	ob.addAsk(makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("s2", "seller2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2005", "2.0"))

	// Submit a buy limit order at 2005 (should match both)
	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2005", "2.0")

	matches, remaining := ob.MatchOrder(buyOrder)

	// Should have 2 matches
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// Match 1: buy 1.0 at 2000 (s1's price)
	if matches[0].Quantity.String() != "1" {
		t.Errorf("match 1 qty: expected 1, got %s", matches[0].Quantity.String())
	}
	if matches[0].Price.String() != "2000" {
		t.Errorf("match 1 price: expected 2000, got %s", matches[0].Price.String())
	}
	if matches[0].MakerOrderID != "s1" {
		t.Errorf("match 1 maker: expected s1, got %s", matches[0].MakerOrderID)
	}

	// Match 2: buy 1.0 at 2005 (s2's price)
	if matches[1].Quantity.String() != "1" {
		t.Errorf("match 2 qty: expected 1, got %s", matches[1].Quantity.String())
	}
	if matches[1].Price.String() != "2005" {
		t.Errorf("match 2 price: expected 2005, got %s", matches[1].Price.String())
	}

	// Buy order should be fully filled, no remaining
	if remaining != nil {
		t.Errorf("expected no remaining order, got %s", remaining.Status)
	}
	if buyOrder.Status != common.OrderStatusFilled {
		t.Errorf("expected filled status, got %s", buyOrder.Status)
	}

	// s1 should be removed (fully filled), s2 should have 1.0 remaining
	if len(ob.asks) != 1 {
		t.Fatalf("expected 1 ask level remaining, got %d", len(ob.asks))
	}
}

func TestMatch_LimitBuyPartialFill(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	// Add resting sell order
	ob.addAsk(makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))

	// Submit a buy limit order for 3.0 at 2000 (only 1.0 available)
	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "3.0")

	matches, remaining := ob.MatchOrder(buyOrder)

	// Should have 1 match
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}

	// s1 should be filled and removed
	if len(ob.asks) != 0 {
		t.Errorf("expected 0 asks, got %d", len(ob.asks))
	}

	// Remaining should be available; manually add to book (engine does this)
	if remaining == nil {
		t.Fatal("expected remaining order")
	}
	if remaining.Status == common.OrderStatusCancelled {
		t.Error("GTC order should not be cancelled")
	}
	if remaining.Remaining().String() != "2" {
		t.Errorf("expected 2.0 remaining, got %s", remaining.Remaining().String())
	}
	if remaining.Price.String() != "2000" {
		t.Errorf("expected remaining at 2000, got %s", remaining.Price.String())
	}

	// Manually add to book (this is what the engine does)
	ob.addBid(remaining)
	bids, _ := ob.Size()
	if bids != 1 {
		t.Fatalf("expected 1 bid level, got %d", bids)
	}
	if ob.bids[0].price.String() != "2000" {
		t.Errorf("expected bid at 2000, got %s", ob.bids[0].price.String())
	}
}

func TestMatch_MarketBuy(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addAsk(makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("s2", "seller2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2010", "2.0"))

	// Market buy for 1.5
	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeMarket, "", "", "1.5")

	matches, remaining := ob.MatchOrder(buyOrder)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// First match: 1.0 at 2000
	if matches[0].Quantity.String() != "1" {
		t.Errorf("match 1 qty: expected 1, got %s", matches[0].Quantity.String())
	}
	// Second match: 0.5 at 2010
	if matches[1].Quantity.String() != "0.5" {
		t.Errorf("match 2 qty: expected 0.5, got %s", matches[1].Quantity.String())
	}

	if remaining != nil {
		t.Errorf("market order should not have remaining")
	}

	// s2 should have 1.5 remaining
	if len(ob.asks) != 1 {
		t.Fatalf("expected 1 ask level remaining, got %d", len(ob.asks))
	}
}

func TestMatch_LimitBuyNoCross(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	// Add ask at 2000
	ob.addAsk(makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))

	// Buy limit at 1990 (below best ask) - should not match
	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "1990", "2.0")

	matches, remaining := ob.MatchOrder(buyOrder)

	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}

	// Remaining should be available (not yet added to book by MatchOrder alone)
	if remaining == nil {
		t.Fatal("expected remaining order")
	}
	if remaining.Remaining().String() != "2" {
		t.Errorf("expected 2.0 remaining, got %s", remaining.Remaining().String())
	}

	// Manually add to book (engine does this in processOrder)
	ob.addBid(remaining)
	bids, _ := ob.Size()
	if bids != 1 {
		t.Fatalf("expected 1 bid level, got %d", bids)
	}
	if ob.bids[0].price.String() != "1990" {
		t.Errorf("expected bid at 1990, got %s", ob.bids[0].price.String())
	}
}

func TestMatch_SellSide(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	// Add resting buy orders
	ob.addBid(makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addBid(makeOrder("b2", "buyer2", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "1995", "2.0"))

	// Sell limit at 1990 (should match best bid at 2000 and 1995)
	sellOrder := makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "1990", "2.5")

	matches, _ := ob.MatchOrder(sellOrder)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}

	// Match 1: 1.0 at 2000 (best bid price)
	if matches[0].Price.String() != "2000" {
		t.Errorf("match 1 price: expected 2000, got %s", matches[0].Price.String())
	}
	// Match 2: 1.5 at 1995
	if matches[1].Price.String() != "1995" {
		t.Errorf("match 2 price: expected 1995, got %s", matches[1].Price.String())
	}
	if matches[1].Quantity.String() != "1.5" {
		t.Errorf("match 2 qty: expected 1.5, got %s", matches[1].Quantity.String())
	}

	// b2 should have 0.5 remaining
	if ob.bids[0].orders.volume.String() != "0.5" {
		t.Errorf("expected remaining bid volume 0.5, got %s", ob.bids[0].orders.volume.String())
	}
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addBid(makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addBid(makeOrder("b2", "buyer2", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "1990", "2.0"))

	cancelled := ob.CancelOrder("b1")
	if cancelled == nil {
		t.Fatal("expected cancelled order, got nil")
	}
	if cancelled.OrderID != "b1" {
		t.Errorf("expected b1, got %s", cancelled.OrderID)
	}
	if cancelled.Status != common.OrderStatusCancelled {
		t.Errorf("expected cancelled status, got %s", cancelled.Status)
	}

	bids, _ := ob.Size()
	if bids != 1 {
		t.Errorf("expected 1 bid remaining, got %d", bids)
	}
}

func TestCancelOrder_NotFound(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	cancelled := ob.CancelOrder("nonexistent")
	if cancelled != nil {
		t.Error("expected nil for non-existent order")
	}
}

func TestOrderBook_BestBidAsk(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	if !ob.BestBid().IsZero() {
		t.Error("empty book should have zero best bid")
	}
	if !ob.BestAsk().IsZero() {
		t.Error("empty book should have zero best ask")
	}
	if !ob.Spread().IsZero() {
		t.Error("empty book should have zero spread")
	}

	ob.addBid(makeOrder("b1", "u1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("a1", "u2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2005", "1.0"))

	if ob.BestBid().String() != "2000" {
		t.Errorf("best bid: expected 2000, got %s", ob.BestBid().String())
	}
	if ob.BestAsk().String() != "2005" {
		t.Errorf("best ask: expected 2005, got %s", ob.BestAsk().String())
	}
	if ob.Spread().String() != "5" {
		t.Errorf("spread: expected 5, got %s", ob.Spread().String())
	}
}

func TestCanFillFOK(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addAsk(makeOrder("s1", "u1", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("s2", "u2", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2005", "2.0"))

	// FOK buy for 2.5 - should be fillable
	buyOrder := makeOrder("b1", "u3", common.SideBuy, common.OrderTypeLimit, common.TIF_FOK, "2005", "2.5")
	if !ob.CanFillFOK(buyOrder) {
		t.Error("FOK should be fillable for 2.5")
	}

	// FOK buy for 4.0 - should not be fillable
	buyOrder2 := makeOrder("b2", "u3", common.SideBuy, common.OrderTypeLimit, common.TIF_FOK, "2005", "4.0")
	if ob.CanFillFOK(buyOrder2) {
		t.Error("FOK should not be fillable for 4.0")
	}

	// FOK buy for 3.0 at 2000 max price - only 1.0 available at or below 2000
	buyOrder3 := makeOrder("b3", "u3", common.SideBuy, common.OrderTypeLimit, common.TIF_FOK, "2000", "3.0")
	if ob.CanFillFOK(buyOrder3) {
		t.Error("FOK at 2000 limit should not be fillable for 3.0")
	}
}

func TestSnapshot(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")

	ob.addBid(makeOrder("b1", "u1", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "2000", "1.0"))
	ob.addBid(makeOrder("b2", "u2", common.SideBuy, common.OrderTypeLimit, common.TIF_GTC, "1995", "2.0"))
	ob.addAsk(makeOrder("a1", "u3", common.SideSell, common.OrderTypeLimit, common.TIF_GTC, "2005", "1.5"))

	snap := ob.Snapshot(10)

	if snap.Symbol != "ETH-USDT" {
		t.Errorf("expected ETH-USDT, got %s", snap.Symbol)
	}
	if len(snap.Bids) != 2 {
		t.Errorf("expected 2 bids in snapshot, got %d", len(snap.Bids))
	}
	if len(snap.Asks) != 1 {
		t.Errorf("expected 1 ask in snapshot, got %d", len(snap.Asks))
	}
	if snap.Bids[0].Price != "2000" {
		t.Errorf("expected bid 2000, got %s", snap.Bids[0].Price)
	}
}

// Benchmark: matching engine throughput
func BenchmarkOrderBook_Matching(b *testing.B) {
	ob := NewOrderBook("ETH-USDT")

	// Pre-populate with resting orders
	for i := 0; i < 100; i++ {
		price := decimal.NewFromInt64(2000).Sub(
			decimal.NewFromInt64(int64(i)).Mul(mustDecimal("0.01")),
		)
		ob.addAsk(makeOrder(
			fmt.Sprintf("s%d", i),
			"seller",
			common.SideSell,
			common.OrderTypeLimit,
			common.TIF_GTC,
			price.String(),
			"1.0",
		))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buyOrder := &Order{
			OrderID:     "bench_buy",
			UserID:      "buyer",
			Symbol:      "ETH-USDT",
			Side:        common.SideBuy,
			Type:        common.OrderTypeLimit,
			TimeInForce: common.TIF_IOC,
			Price:       mustDecimal("2000"),
			Quantity:    mustDecimal("0.5"),
			Timestamp:   time.Now().UnixNano(),
		}
		ob.MatchOrder(buyOrder)
	}
}

func TestMatch_IOCPartialFill(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	ob.addAsk(makeOrder("s1", "seller1", common.SideSell, common.OrderTypeLimit,
		common.TIF_GTC, "2000", "1.0"))

	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit,
		common.TIF_IOC, "2000", "3.0")
	matches, remaining := ob.MatchOrder(buyOrder)

	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].Quantity.String() != "1" {
		t.Errorf("match qty: expected 1, got %s", matches[0].Quantity.String())
	}
	if remaining == nil {
		t.Fatal("expected remaining order")
	}
	if remaining.Status != common.OrderStatusCancelled {
		t.Errorf("IOC remaining should be cancelled, got %s", remaining.Status)
	}
	if len(ob.asks) != 0 {
		t.Errorf("asks should be empty, got %d", len(ob.asks))
	}
}

func TestCancelOrder_AskSide(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	ob.addAsk(makeOrder("a1", "seller1", common.SideSell, common.OrderTypeLimit,
		common.TIF_GTC, "2000", "1.0"))
	ob.addAsk(makeOrder("a2", "seller2", common.SideSell, common.OrderTypeLimit,
		common.TIF_GTC, "2005", "2.0"))

	cancelled := ob.CancelOrder("a1")
	if cancelled == nil {
		t.Fatal("expected cancelled order, got nil")
	}
	if cancelled.OrderID != "a1" {
		t.Errorf("expected a1, got %s", cancelled.OrderID)
	}
	_, asks := ob.Size()
	if asks != 1 {
		t.Errorf("expected 1 ask remaining, got %d", asks)
	}
}

func TestMatch_MarketSell(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	ob.addBid(makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit,
		common.TIF_GTC, "2000", "1.0"))
	ob.addBid(makeOrder("b2", "buyer2", common.SideBuy, common.OrderTypeLimit,
		common.TIF_GTC, "1995", "2.0"))

	sellOrder := makeOrder("s1", "seller1", common.SideSell, common.OrderTypeMarket,
		"", "", "2.5")
	matches, remaining := ob.MatchOrder(sellOrder)

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Price.String() != "2000" {
		t.Errorf("match 1 price: expected 2000, got %s", matches[0].Price.String())
	}
	if matches[1].Price.String() != "1995" {
		t.Errorf("match 2 price: expected 1995, got %s", matches[1].Price.String())
	}
	if remaining != nil {
		t.Error("market order should not have remaining")
	}
}

func TestMatch_IOCNoLiquidity(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	buyOrder := makeOrder("b1", "buyer1", common.SideBuy, common.OrderTypeLimit,
		common.TIF_IOC, "2000", "1.0")
	matches, remaining := ob.MatchOrder(buyOrder)

	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
	if remaining == nil || remaining.Status != common.OrderStatusCancelled {
		t.Error("IOC with no liquidity should be cancelled")
	}
}

func TestOrderBook_EmptySnapshot(t *testing.T) {
	ob := NewOrderBook("ETH-USDT")
	snap := ob.Snapshot(10)
	if snap.Symbol != "ETH-USDT" {
		t.Errorf("expected ETH-USDT, got %s", snap.Symbol)
	}
	if len(snap.Bids) != 0 || len(snap.Asks) != 0 {
		t.Error("empty book snapshot should have no levels")
	}
}
