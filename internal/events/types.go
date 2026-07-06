package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/exchange/internal/common"
)

// EventType identifies the type of domain event.
type EventType string

const (
	EventOrderPlaced       EventType = "order.placed"
	EventOrderMatched      EventType = "order.matched"
	EventOrderCancelled    EventType = "order.cancelled"
	EventTradeExecuted     EventType = "trade.executed"
	EventBalanceChanged    EventType = "balance.changed"
	EventDepositDetected   EventType = "deposit.detected"
	EventDepositConfirmed  EventType = "deposit.confirmed"
	EventWithdrawalReq     EventType = "withdrawal.requested"
	EventWithdrawalSent    EventType = "withdrawal.sent"
	EventWithdrawalFailed  EventType = "withdrawal.failed"
)

// Event represents a domain event in the system.
type Event struct {
	ID        string          `json:"id"`
	Type      EventType       `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	Version   int             `json:"version"`
}

// SetPayload marshals a payload struct into the event.
func (e *Event) SetPayload(payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	e.Payload = data
	return nil
}

// GetPayload unmarshals the payload into the given struct.
func (e *Event) GetPayload(v interface{}) error {
	return json.Unmarshal(e.Payload, v)
}

// EventBus is the interface for publishing and subscribing to events.
type EventBus interface {
	Publish(ctx context.Context, topic string, events ...*Event) error
	Subscribe(ctx context.Context, topic string, group string, handler EventHandler) error
	Close() error
}

// Compile-time interface compliance checks.
var _ EventBus = (*MemoryEventBus)(nil)
var _ EventBus = (*RedisEventBus)(nil)

// EventHandler is a function that processes events.
type EventHandler func(ctx context.Context, event *Event) error

// Event payloads

type OrderPlacedPayload struct {
	OrderID     string `json:"order_id"`
	UserID      string `json:"user_id"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	Price       string `json:"price"`
	StopPrice   string `json:"stop_price,omitempty"`
	Quantity    string `json:"quantity"`
	FilledQty   string `json:"filled_qty"`
	TimeInForce string `json:"time_in_force"`
	Status      string `json:"status"`
}

type OrderMatchedPayload struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Symbol  string `json:"symbol"`
	Side    string `json:"side"`
	Status  string `json:"status"`
}

type OrderCancelledPayload struct {
	OrderID string `json:"order_id"`
	UserID  string `json:"user_id"`
	Symbol  string `json:"symbol"`
	Reason  string `json:"reason,omitempty"`
}

type TradeExecutedPayload struct {
	TradeID      string `json:"trade_id"`
	TakerOrderID string `json:"taker_order_id"`
	MakerOrderID string `json:"maker_order_id"`
	TakerUserID  string `json:"taker_user_id"`
	MakerUserID  string `json:"maker_user_id"`
	Symbol       string `json:"symbol"`
	Price        string `json:"price"`
	Quantity     string `json:"quantity"`
	QuoteQty     string `json:"quote_qty"`
	TakerSide    string `json:"taker_side"`
	Timestamp    int64  `json:"timestamp"`
}

type BalanceChangedPayload struct {
	UserID    string `json:"user_id"`
	Currency  string `json:"currency"`
	Amount    string `json:"amount"`
	NewBalance string `json:"new_balance"`
	Reason    string `json:"reason"` // trade, deposit, withdrawal, fee
	Reference string `json:"reference,omitempty"`
}

type DepositDetectedPayload struct {
	TxHash        string `json:"tx_hash"`
	Currency      string `json:"currency"`
	Chain         string `json:"chain"`
	FromAddress   string `json:"from_address"`
	ToAddress     string `json:"to_address"`
	Amount        string `json:"amount"`
	BlockNumber   uint64 `json:"block_number"`
	Confirmations uint64 `json:"confirmations"`
		RequiredConfirmations uint64 `json:"required_confirmations"`
	UserID        string `json:"user_id"`
}

type WithdrawalRequestedPayload struct {
	WithdrawalID string `json:"withdrawal_id"`
	UserID       string `json:"user_id"`
	Currency     string `json:"currency"`
	Chain        string `json:"chain"`
	ToAddress    string `json:"to_address"`
	Amount       string `json:"amount"`
	Fee          string `json:"fee"`
}

// NewEventID generates a unique event ID.
func NewEventID() string {
	return common.NewEventID()
}
