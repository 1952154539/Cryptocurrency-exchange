package handler

import (
	"encoding/json"
	"net/http"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/order"
)

// OrderHandler handles order-related HTTP requests.
type OrderHandler struct {
	orderSvc *order.Service
}

// NewOrderHandler creates an order handler.
func NewOrderHandler(orderSvc *order.Service) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc}
}

// PlaceOrder handles POST /api/v1/order
func (h *OrderHandler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Type          string `json:"type"`
		TimeInForce   string `json:"timeInForce,omitempty"`
		Price         string `json:"price,omitempty"`
		StopPrice     string `json:"stopPrice,omitempty"`
		Quantity      string `json:"quantity"`
		ClientOrderID string `json:"clientOrderId,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	price, _ := decimal.NewFromString(req.Price)
	stopPrice, _ := decimal.NewFromString(req.StopPrice)
	quantity, err := decimal.NewFromString(req.Quantity)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid quantity")
		return
	}

	placeReq := &order.PlaceOrderRequest{
		UserID:        userID,
		Symbol:        common.Symbol(req.Symbol),
		Side:          common.Side(req.Side),
		Type:          common.OrderType(req.Type),
		TimeInForce:   common.TimeInForce(req.TimeInForce),
		Price:         price,
		StopPrice:     stopPrice,
		Quantity:      quantity,
		ClientOrderID: req.ClientOrderID,
	}

	resp, err := h.orderSvc.PlaceOrder(r.Context(), placeReq)
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"orderId":       resp.OrderID,
		"clientOrderId": resp.ClientOrderID,
		"status":        resp.Status,
		"filledQty":     resp.FilledQty.String(),
	})
}

// CancelOrder handles DELETE /api/v1/order
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Symbol  string `json:"symbol"`
		OrderID string `json:"orderId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.orderSvc.CancelOrder(r.Context(), userID, common.Symbol(req.Symbol), req.OrderID); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetOpenOrders handles GET /api/v1/open-orders
func (h *OrderHandler) GetOpenOrders(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	symbol := r.URL.Query().Get("symbol")
	orders, err := h.orderSvc.GetOpenOrders(r.Context(), userID, symbol)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, orders)
}

// GetOrder handles GET /api/v1/order
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orderID := r.URL.Query().Get("orderId")
	order, err := h.orderSvc.GetOrder(r.Context(), orderID)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if order.UserID != userID {
		WriteError(w, http.StatusForbidden, "not your order")
		return
	}

	WriteJSON(w, http.StatusOK, order)
}
