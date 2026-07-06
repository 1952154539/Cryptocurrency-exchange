package fix

import (
	"fmt"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/order"
	"github.com/quickfixgo/quickfix"
	"github.com/rs/zerolog/log"
)

// FIX tag constants
const (
	tagMsgType      quickfix.Tag = 35
	tagSenderCompID quickfix.Tag = 49
	tagTargetCompID quickfix.Tag = 56
	tagClOrdID      quickfix.Tag = 11
	tagOrderID      quickfix.Tag = 37
	tagSymbol       quickfix.Tag = 55
	tagSide         quickfix.Tag = 54
	tagOrdType      quickfix.Tag = 40
	tagPrice        quickfix.Tag = 44
	tagOrderQty     quickfix.Tag = 38
	tagTimeInForce  quickfix.Tag = 59
	tagOrdStatus    quickfix.Tag = 39
	tagExecType     quickfix.Tag = 150
	tagCumQty       quickfix.Tag = 14
	tagLeavesQty    quickfix.Tag = 151
	tagAvgPx        quickfix.Tag = 6
	tagTransactTime quickfix.Tag = 60
)

// SessionManager handles FIX protocol sessions.
type SessionManager struct {
	orderSvc *order.Service
}

// NewSessionManager creates a FIX session manager.
func NewSessionManager(orderSvc *order.Service) *SessionManager {
	return &SessionManager{orderSvc: orderSvc}
}

// NewOrderSingleToPlaceOrder converts a FIX 4.4 NewOrderSingle (MsgType=D) to PlaceOrderRequest.
func NewOrderSingleToPlaceOrder(msg *quickfix.Message) (*order.PlaceOrderRequest, error) {
	body := &msg.Body

	clOrdID, _ := body.GetString(tagClOrdID)
	symbol, _ := body.GetString(tagSymbol)

	sideStr, _ := body.GetString(tagSide)
	ordTypeStr, _ := body.GetString(tagOrdType)

	priceStr, _ := body.GetString(tagPrice)
	qtyStr, _ := body.GetString(tagOrderQty)

	var internalSide common.Side
	switch sideStr {
	case "1":
		internalSide = common.SideBuy
	case "2":
		internalSide = common.SideSell
	default:
		return nil, fmt.Errorf("invalid FIX side: %s", sideStr)
	}

	var internalType common.OrderType
	switch ordTypeStr {
	case "1":
		internalType = common.OrderTypeMarket
	case "2":
		internalType = common.OrderTypeLimit
	case "3":
		internalType = common.OrderTypeStopLoss
	case "4":
		internalType = common.OrderTypeStopLimit
	default:
		return nil, fmt.Errorf("invalid FIX ordType: %s", ordTypeStr)
	}

	priceDec, _ := decimal.NewFromString(priceStr)
	if priceStr == "" {
		priceDec = decimal.Zero
	}
	qtyDec, _ := decimal.NewFromString(qtyStr)

	tif := "GTC"
	if tifStr, err := body.GetString(tagTimeInForce); err == nil {
		switch tifStr {
		case "0":
			tif = "GTC" // Day
		case "3":
			tif = "IOC" // Immediate or Cancel
		case "4":
			tif = "FOK" // Fill or Kill
		}
	}

	return &order.PlaceOrderRequest{
		Symbol:        common.Symbol(symbol),
		Side:          internalSide,
		Type:          internalType,
		TimeInForce:   common.TimeInForce(tif),
		Price:         priceDec,
		Quantity:      qtyDec,
		ClientOrderID: clOrdID,
	}, nil
}

// BuildExecutionReport creates a FIX 4.4 ExecutionReport (MsgType=8).
func BuildExecutionReport(resp *order.PlaceOrderResponse, clOrdID, symbol string) *quickfix.Message {
	msg := quickfix.NewMessage()
	msg.Header.SetField(tagMsgType, quickfix.FIXString("8"))
	msg.Header.SetField(tagSenderCompID, quickfix.FIXString("EXCHANGE"))
	msg.Header.SetField(tagTargetCompID, quickfix.FIXString("CLIENT"))
	msg.Body.SetField(tagOrderID, quickfix.FIXString(resp.OrderID))
	msg.Body.SetField(tagClOrdID, quickfix.FIXString(clOrdID))
	msg.Body.SetField(tagSymbol, quickfix.FIXString(symbol))
	msg.Body.SetField(tagOrdStatus, quickfix.FIXString(fixStatus(resp.Status)))
	msg.Body.SetField(tagExecType, quickfix.FIXString(fixExecType(resp.Status)))
	msg.Body.SetField(tagCumQty, quickfix.FIXString(resp.FilledQty.String()))
	msg.Body.SetField(tagLeavesQty, quickfix.FIXString("0"))
	return msg
}

// BuildReject creates a FIX 4.4 OrderCancelReject or BusinessMessageReject.
func BuildReject(clOrdID, reason string) *quickfix.Message {
	msg := quickfix.NewMessage()
	msg.Header.SetField(tagMsgType, quickfix.FIXString("j"))
	msg.Header.SetField(tagSenderCompID, quickfix.FIXString("EXCHANGE"))
	msg.Body.SetField(tagClOrdID, quickfix.FIXString(clOrdID))
	msg.Body.SetField(tagOrdStatus, quickfix.FIXString("8")) // Rejected
	msg.Body.SetField(quickfix.Tag(58), quickfix.FIXString(reason))
	return msg
}

func fixStatus(s common.OrderStatus) string {
	switch s {
	case common.OrderStatusOpen:
		return "0"
	case common.OrderStatusPartiallyFilled:
		return "1"
	case common.OrderStatusFilled:
		return "2"
	case common.OrderStatusCancelled:
		return "4"
	case common.OrderStatusRejected:
		return "8"
	default:
		return "0"
	}
}

func fixExecType(s common.OrderStatus) string {
	switch s {
	case common.OrderStatusFilled:
		return "2"
	case common.OrderStatusPartiallyFilled:
		return "1"
	case common.OrderStatusCancelled:
		return "4"
	case common.OrderStatusRejected:
		return "8"
	default:
		return "0"
	}
}

// Log logs a FIX protocol message.
func Log(msg string) {
	log.Info().Str("protocol", "FIX").Msg(msg)
}

var _ = fmt.Sprintf
