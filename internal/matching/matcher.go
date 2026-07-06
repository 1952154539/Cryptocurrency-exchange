package matching

import (
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
)

// MatchOrder processes an incoming taker order against the resting order book.
// Returns the list of match results and the remaining order (nil if fully filled).
func (ob *OrderBook) MatchOrder(order *Order) ([]*MatchResult, *Order) {
	var matches []*MatchResult

	switch {
	case order.Side == common.SideBuy:
		matches = ob.matchBuy(order)
	case order.Side == common.SideSell:
		matches = ob.matchSell(order)
	}

	// Post-match handling
	remaining := order.Remaining()
	switch {
	case remaining.Cmp(decimal.Zero) <= 0:
		order.Status = common.OrderStatusFilled
		return matches, nil
	case order.TimeInForce == common.TIF_FOK:
		// FOK: revert all fills, cancel entire order
		// In production, this would require rollback of partial fills
		// For MVP, we check before matching if FOK can be fully filled
		order.Status = common.OrderStatusCancelled
		return nil, order
	case order.TimeInForce == common.TIF_IOC:
		// IOC: cancel remaining, keep partial fills
		order.Status = common.OrderStatusCancelled
		return matches, order
	default:
		// GTC: add remaining to order book as a resting order
		return matches, order
	}
}

// matchBuy matches a buy order against the ask side of the book.
func (ob *OrderBook) matchBuy(order *Order) []*MatchResult {
	var matches []*MatchResult

	for order.Remaining().Cmp(decimal.Zero) > 0 && len(ob.asks) > 0 {
		bestAsk := ob.asks[0]

		// Limit order: cannot buy above limit price
		if order.Type == common.OrderTypeLimit && order.Price.Cmp(bestAsk.price) < 0 {
			break
		}

		// Take from the oldest order at this price level (FIFO)
		maker := bestAsk.orders.head.order
		tradeQty := order.Remaining()
		makerRemaining := maker.Remaining()
		if tradeQty.Cmp(makerRemaining) > 0 {
			tradeQty = makerRemaining
		}

		// Trade at maker's price
		tradePrice := maker.Price
		quoteQty := tradePrice.Mul(tradeQty)

		now := time.Now().UnixNano()
		match := &MatchResult{
			TakerOrderID: order.OrderID,
			MakerOrderID: maker.OrderID,
			TakerUserID:  order.UserID,
			MakerUserID:  maker.UserID,
			Symbol:       ob.symbol,
			Price:        tradePrice,
			Quantity:     tradeQty,
			QuoteQty:     quoteQty,
			TakerSide:    common.SideBuy,
			Timestamp:    now,
		}
		matches = append(matches, match)

		// Update filled quantities
		order.FilledQty = order.FilledQty.Add(tradeQty)
		maker.FilledQty = maker.FilledQty.Add(tradeQty)

		// Update price level volume
		bestAsk.orders.volume = bestAsk.orders.volume.Sub(tradeQty)

		// Remove fully filled maker
		if maker.IsFilled() {
			maker.Status = common.OrderStatusFilled
			bestAsk.orders.pop()
			if bestAsk.orders.len == 0 {
				ob.removeAskLevel(0)
			}
		}
	}

	return matches
}

// matchSell matches a sell order against the bid side of the book.
func (ob *OrderBook) matchSell(order *Order) []*MatchResult {
	var matches []*MatchResult

	for order.Remaining().Cmp(decimal.Zero) > 0 && len(ob.bids) > 0 {
		bestBid := ob.bids[0]

		// Limit order: cannot sell below limit price
		if order.Type == common.OrderTypeLimit && order.Price.Cmp(bestBid.price) > 0 {
			break
		}

		maker := bestBid.orders.head.order
		tradeQty := order.Remaining()
		makerRemaining := maker.Remaining()
		if tradeQty.Cmp(makerRemaining) > 0 {
			tradeQty = makerRemaining
		}

		tradePrice := maker.Price
		quoteQty := tradePrice.Mul(tradeQty)

		now := time.Now().UnixNano()
		match := &MatchResult{
			TakerOrderID: order.OrderID,
			MakerOrderID: maker.OrderID,
			TakerUserID:  order.UserID,
			MakerUserID:  maker.UserID,
			Symbol:       ob.symbol,
			Price:        tradePrice,
			Quantity:     tradeQty,
			QuoteQty:     quoteQty,
			TakerSide:    common.SideSell,
			Timestamp:    now,
		}
		matches = append(matches, match)

		order.FilledQty = order.FilledQty.Add(tradeQty)
		maker.FilledQty = maker.FilledQty.Add(tradeQty)
		bestBid.orders.volume = bestBid.orders.volume.Sub(tradeQty)

		if maker.IsFilled() {
			maker.Status = common.OrderStatusFilled
			bestBid.orders.pop()
			if bestBid.orders.len == 0 {
				ob.removeBidLevel(0)
			}
		}
	}

	return matches
}

// CanFillFOK checks if a FOK order can be fully filled against the current book.
func (ob *OrderBook) CanFillFOK(order *Order) bool {
	required := order.Quantity

	if order.Side == common.SideBuy {
		cumVolume := decimal.Zero
		for _, pl := range ob.asks {
			if order.Type == common.OrderTypeLimit && order.Price.Cmp(pl.price) < 0 {
				break
			}
			cumVolume = cumVolume.Add(pl.orders.volume)
			if cumVolume.Cmp(required) >= 0 {
				return true
			}
		}
	} else {
		cumVolume := decimal.Zero
		for _, pl := range ob.bids {
			if order.Type == common.OrderTypeLimit && order.Price.Cmp(pl.price) > 0 {
				break
			}
			cumVolume = cumVolume.Add(pl.orders.volume)
			if cumVolume.Cmp(required) >= 0 {
				return true
			}
		}
	}
	return false
}

