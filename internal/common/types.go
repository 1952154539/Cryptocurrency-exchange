package common

// Side represents buy or sell.
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

func (s Side) Opposite() Side {
	if s == SideBuy {
		return SideSell
	}
	return SideBuy
}

// OrderType represents the type of an order.
type OrderType string

const (
	OrderTypeMarket    OrderType = "market"
	OrderTypeLimit     OrderType = "limit"
	OrderTypeStopLoss  OrderType = "stop_loss"
	OrderTypeStopLimit OrderType = "stop_limit"
)

// TimeInForce specifies how long an order remains active.
type TimeInForce string

const (
	TIF_GTC TimeInForce = "GTC" // Good Till Cancelled
	TIF_IOC TimeInForce = "IOC" // Immediate Or Cancel
	TIF_FOK TimeInForce = "FOK" // Fill Or Kill
)

// OrderStatus represents the lifecycle status of an order.
type OrderStatus string

const (
	OrderStatusCreated        OrderStatus = "created"
	OrderStatusOpen           OrderStatus = "open"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled         OrderStatus = "filled"
	OrderStatusCancelled      OrderStatus = "cancelled"
	OrderStatusRejected       OrderStatus = "rejected"
	OrderStatusExpired        OrderStatus = "expired"
)

// IsFinal returns true if the order is in a terminal state.
func (s OrderStatus) IsFinal() bool {
	return s == OrderStatusFilled || s == OrderStatusCancelled ||
		s == OrderStatusRejected || s == OrderStatusExpired
}

// DepositStatus represents the status of a deposit.
type DepositStatus string

const (
	DepositStatusPending    DepositStatus = "pending"
	DepositStatusConfirming DepositStatus = "confirming"
	DepositStatusConfirmed  DepositStatus = "confirmed"
	DepositStatusOrphaned   DepositStatus = "orphaned"
)

// WithdrawalStatus represents the status of a withdrawal.
type WithdrawalStatus string

const (
	WithdrawalStatusPending    WithdrawalStatus = "pending"
	WithdrawalStatusApproved   WithdrawalStatus = "approved"
	WithdrawalStatusProcessing WithdrawalStatus = "processing"
	WithdrawalStatusBroadcast  WithdrawalStatus = "broadcast"
	WithdrawalStatusCompleted  WithdrawalStatus = "completed"
	WithdrawalStatusFailed     WithdrawalStatus = "failed"
	WithdrawalStatusRejected   WithdrawalStatus = "rejected"
)

// UserStatus represents the account status.
type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusFrozen  UserStatus = "frozen"
)

// Currency represents a cryptocurrency or fiat symbol.
type Currency string

// Symbol represents a trading pair symbol (e.g., "ETH-USDT").
type Symbol string

// Base returns the base currency of a symbol.
func (s Symbol) Base() Currency {
	for i, c := range s {
		if c == '-' {
			return Currency(s[:i])
		}
	}
	return Currency(s)
}

// Quote returns the quote currency of a symbol.
func (s Symbol) Quote() Currency {
	for i, c := range s {
		if c == '-' {
			return Currency(s[i+1:])
		}
	}
	return ""
}

// Chain identifies a blockchain network.
type Chain string

const (
	ChainEthereum Chain = "ethereum"
	ChainBSC      Chain = "bsc"
	ChainArbitrum Chain = "arbitrum"
)
