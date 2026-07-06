package matching

import (
	"fmt"
	"sort"
	"sync"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
)

// orderNode is a node in the FIFO queue at a price level.
type orderNode struct {
	order *Order
	next  *orderNode
	prev  *orderNode
}

// orderQueue is a doubly-linked FIFO queue of orders at the same price.
type orderQueue struct {
	head   *orderNode
	tail   *orderNode
	volume decimal.Decimal
	len    int
}

// push adds an order to the back of the queue.
func (q *orderQueue) push(order *Order) {
	node := orderNodePool.Get().(*orderNode)
	node.order = order
	node.next = nil
	node.prev = nil

	if q.tail == nil {
		q.head = node
		q.tail = node
	} else {
		q.tail.next = node
		node.prev = q.tail
		q.tail = node
	}
	q.volume = q.volume.Add(order.Remaining())
	q.len++
}

// pop removes and returns the front order.
func (q *orderQueue) pop() *Order {
	if q.head == nil {
		return nil
	}
	node := q.head
	q.head = node.next
	if q.head != nil {
		q.head.prev = nil
	} else {
		q.tail = nil
	}
	q.volume = q.volume.Sub(node.order.Remaining())
	q.len--

	order := node.order
	node.order = nil
	node.next = nil
	node.prev = nil
	orderNodePool.Put(node)
	return order
}

// remove removes a specific order from the queue.
func (q *orderQueue) remove(orderID string) *Order {
	for node := q.head; node != nil; node = node.next {
		if node.order.OrderID == orderID {
			if node.prev != nil {
				node.prev.next = node.next
			} else {
				q.head = node.next
			}
			if node.next != nil {
				node.next.prev = node.prev
			} else {
				q.tail = node.prev
			}
			q.volume = q.volume.Sub(node.order.Remaining())
			q.len--

			order := node.order
			node.order = nil
			node.next = nil
			node.prev = nil
			orderNodePool.Put(node)
			return order
		}
	}
	return nil
}

// priceLevel represents all orders at a given price.
type priceLevel struct {
	price  decimal.Decimal
	orders *orderQueue
}

// OrderBook holds bids and asks for a single trading pair.
// All methods are safe for concurrent use: writers acquire Lock, readers acquire RLock.
type OrderBook struct {
	symbol common.Symbol
	bids   []*priceLevel // descending by price (highest bid at index 0)
	asks   []*priceLevel // ascending by price (lowest ask at index 0)
	seqNum uint64
	mu     sync.RWMutex
}

// NewOrderBook creates a new order book for a trading pair.
func NewOrderBook(symbol common.Symbol) *OrderBook {
	return &OrderBook{
		symbol: symbol,
		bids:   make([]*priceLevel, 0),
		asks:   make([]*priceLevel, 0),
	}
}

// Symbol returns the trading pair symbol.
func (ob *OrderBook) Symbol() common.Symbol {
	return ob.symbol
}

// addBid inserts a bid order at the correct price level (descending order).
// Must be called while ob.mu is held (by processOrder or MatchOrder).
func (ob *OrderBook) addBid(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.seqNum++
	order.Status = common.OrderStatusOpen

	// Find insertion point: bid levels are descending, want highest price at index 0
	i := sort.Search(len(ob.bids), func(i int) bool {
		return ob.bids[i].price.Cmp(order.Price) <= 0
	})

	if i < len(ob.bids) && ob.bids[i].price.Cmp(order.Price) == 0 {
		// Existing price level
		ob.bids[i].orders.push(order)
	} else {
		// New price level
		pl := &priceLevel{
			price:  order.Price,
			orders: &orderQueue{},
		}
		pl.orders.push(order)

		ob.bids = append(ob.bids, nil)
		copy(ob.bids[i+1:], ob.bids[i:])
		ob.bids[i] = pl
	}
}

// addAsk inserts an ask order at the correct price level (ascending order).
// Must be called while ob.mu is held (by processOrder or MatchOrder).
func (ob *OrderBook) addAsk(order *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.seqNum++
	order.Status = common.OrderStatusOpen

	// Find insertion point: ask levels are ascending, want lowest price at index 0
	i := sort.Search(len(ob.asks), func(i int) bool {
		return ob.asks[i].price.Cmp(order.Price) >= 0
	})

	if i < len(ob.asks) && ob.asks[i].price.Cmp(order.Price) == 0 {
		ob.asks[i].orders.push(order)
	} else {
		pl := &priceLevel{
			price:  order.Price,
			orders: &orderQueue{},
		}
		pl.orders.push(order)

		ob.asks = append(ob.asks, nil)
		copy(ob.asks[i+1:], ob.asks[i:])
		ob.asks[i] = pl
	}
}

// CancelOrder removes an order from the book. Returns the order if found.
// Safe for concurrent use: acquires Lock for external callers.
func (ob *OrderBook) CancelOrder(orderID string) *Order {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Search bids
	for i, pl := range ob.bids {
		if o := pl.orders.remove(orderID); o != nil {
			if pl.orders.len == 0 {
				ob.removeBidLevel(i)
			}
			o.Status = common.OrderStatusCancelled
			return o
		}
	}
	// Search asks
	for i, pl := range ob.asks {
		if o := pl.orders.remove(orderID); o != nil {
			if pl.orders.len == 0 {
				ob.removeAskLevel(i)
			}
			o.Status = common.OrderStatusCancelled
			return o
		}
	}
	return nil
}

func (ob *OrderBook) removeBidLevel(i int) {
	copy(ob.bids[i:], ob.bids[i+1:])
	ob.bids[len(ob.bids)-1] = nil
	ob.bids = ob.bids[:len(ob.bids)-1]
}

func (ob *OrderBook) removeAskLevel(i int) {
	copy(ob.asks[i:], ob.asks[i+1:])
	ob.asks[len(ob.asks)-1] = nil
	ob.asks = ob.asks[:len(ob.asks)-1]
}

// BestBid returns the highest bid price (0 if no bids).
func (ob *OrderBook) BestBid() decimal.Decimal {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.bids) == 0 {
		return decimal.Zero
	}
	return ob.bids[0].price
}

// BestAsk returns the lowest ask price (0 if no asks).
func (ob *OrderBook) BestAsk() decimal.Decimal {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.asks) == 0 {
		return decimal.Zero
	}
	return ob.asks[0].price
}

// Spread returns the difference between best ask and best bid.
func (ob *OrderBook) Spread() decimal.Decimal {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	if len(ob.bids) == 0 || len(ob.asks) == 0 {
		return decimal.Zero
	}
	return ob.asks[0].price.Sub(ob.bids[0].price)
}

// GetOrder finds an order in the book by ID.
func (ob *OrderBook) GetOrder(orderID string) *Order {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	for _, pl := range ob.bids {
		for n := pl.orders.head; n != nil; n = n.next {
			if n.order.OrderID == orderID {
				return n.order
			}
		}
	}
	for _, pl := range ob.asks {
		for n := pl.orders.head; n != nil; n = n.next {
			if n.order.OrderID == orderID {
				return n.order
			}
		}
	}
	return nil
}

// Snapshot returns a read-only view of the order book.
func (ob *OrderBook) Snapshot(depth int) *BookSnapshot {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	snap := &BookSnapshot{
		Symbol: ob.symbol,
		SeqNum: ob.seqNum,
	}

	bidsLen := len(ob.bids)
	if depth > 0 && depth < bidsLen {
		bidsLen = depth
	}
	snap.Bids = make([]PriceLevelSnapshot, bidsLen)
	for i := 0; i < bidsLen; i++ {
		snap.Bids[i] = PriceLevelSnapshot{
			Price:  ob.bids[i].price.String(),
			Volume: ob.bids[i].orders.volume.String(),
			Orders: ob.bids[i].orders.len,
		}
	}

	asksLen := len(ob.asks)
	if depth > 0 && depth < asksLen {
		asksLen = depth
	}
	snap.Asks = make([]PriceLevelSnapshot, asksLen)
	for i := 0; i < asksLen; i++ {
		snap.Asks[i] = PriceLevelSnapshot{
			Price:  ob.asks[i].price.String(),
			Volume: ob.asks[i].orders.volume.String(),
			Orders: ob.asks[i].orders.len,
		}
	}

	return snap
}

// Size returns total number of price levels (bids + asks).
func (ob *OrderBook) Size() (bids, asks int) {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	return len(ob.bids), len(ob.asks)
}

// Dump prints the current order book state for debugging.
func (ob *OrderBook) Dump() string {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	s := fmt.Sprintf("=== %s Order Book ===\n", ob.symbol)
	s += "Bids:\n"
	for _, pl := range ob.bids {
		s += fmt.Sprintf("  %s x %s (%d orders)\n",
			pl.price.String(), pl.orders.volume.String(), pl.orders.len)
	}
	s += "Asks:\n"
	for _, pl := range ob.asks {
		s += fmt.Sprintf("  %s x %s (%d orders)\n",
			pl.price.String(), pl.orders.volume.String(), pl.orders.len)
	}
	return s
}
