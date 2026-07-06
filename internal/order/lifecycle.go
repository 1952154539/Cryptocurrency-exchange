package order

import (
	"github.com/exchange/internal/common"
)

// orderTransitions defines all valid order state transitions.
var orderTransitions = map[common.OrderStatus][]common.OrderStatus{
	common.OrderStatusCreated: {
		common.OrderStatusOpen,
		common.OrderStatusPartiallyFilled,
		common.OrderStatusRejected,
		common.OrderStatusCancelled,
		common.OrderStatusFilled,
	},
	common.OrderStatusOpen: {
		common.OrderStatusPartiallyFilled,
		common.OrderStatusFilled,
		common.OrderStatusCancelled,
		common.OrderStatusExpired,
	},
	common.OrderStatusPartiallyFilled: {
		common.OrderStatusPartiallyFilled,
		common.OrderStatusFilled,
		common.OrderStatusCancelled,
		common.OrderStatusExpired,
	},
}

// StateMachine manages order lifecycle transitions.
type StateMachine struct{}

// NewStateMachine creates an order state machine.
func NewStateMachine() *StateMachine {
	return &StateMachine{}
}

// CanTransition checks if an order can move from current status to new status.
func (sm *StateMachine) CanTransition(current, next common.OrderStatus) bool {
	allowed, ok := orderTransitions[current]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == next {
			return true
		}
	}
	return false
}

// ValidTransitions from a given state.
func (sm *StateMachine) ValidTransitions(current common.OrderStatus) []common.OrderStatus {
	return orderTransitions[current]
}
