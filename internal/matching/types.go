package matching

import (
	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
)

// Order is the core order struct used in the matching engine.
type Order struct {
	OrderID       string
	UserID        string
	Symbol        common.Symbol
	Side          common.Side
	Type          common.OrderType
	TimeInForce   common.TimeInForce
	Price         decimal.Decimal
	StopPrice     decimal.Decimal
	Quantity      decimal.Decimal
	FilledQty     decimal.Decimal
	DisplayQty    decimal.Decimal // for iceberg orders, 0 means not iceberg
	Timestamp     int64           // nanosecond timestamp for price-time priority
	Status        common.OrderStatus
}

// Remaining returns the unfilled quantity.
func (o *Order) Remaining() decimal.Decimal {
	return o.Quantity.Sub(o.FilledQty)
}

// IsFilled returns true if the order is fully filled.
func (o *Order) IsFilled() bool {
	return o.Remaining().Cmp(decimal.Zero) <= 0
}

// MatchResult records a single trade between a maker and taker.
type MatchResult struct {
	TakerOrderID string
	MakerOrderID string
	TakerUserID  string
	MakerUserID  string
	Symbol       common.Symbol
	Price        decimal.Decimal
	Quantity     decimal.Decimal
	QuoteQty     decimal.Decimal
	TakerSide    common.Side
	Timestamp    int64
}

// BookSnapshot is a point-in-time view of the order book.
type BookSnapshot struct {
	Symbol common.Symbol
	Bids   []PriceLevelSnapshot
	Asks   []PriceLevelSnapshot
	SeqNum uint64
}

// PriceLevelSnapshot is a single price level for display.
type PriceLevelSnapshot struct {
	Price  string
	Volume string
	Orders int
}
