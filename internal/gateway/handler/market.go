package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/marketdata"
)

// MarketHandler handles public market data endpoints.
type MarketHandler struct {
	marketSvc *marketdata.Service
}

// NewMarketHandler creates a market data handler.
func NewMarketHandler(marketSvc *marketdata.Service) *MarketHandler {
	return &MarketHandler{marketSvc: marketSvc}
}

// GetDepth handles GET /api/v1/depth?symbol=ETH-USDT&limit=100
func (h *MarketHandler) GetDepth(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	snap, err := h.marketSvc.GetOrderBook(r.Context(), common.Symbol(symbol), limit)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, snap)
}

// GetTrades handles GET /api/v1/trades?symbol=ETH-USDT&limit=500
func (h *MarketHandler) GetTrades(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limitStr := r.URL.Query().Get("limit")
	limit := 500
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	trades, err := h.marketSvc.GetRecentTrades(r.Context(), common.Symbol(symbol), limit)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, trades)
}

// GetTicker handles GET /api/v1/ticker/24hr?symbol=ETH-USDT
func (h *MarketHandler) GetTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	ticker, err := h.marketSvc.GetTicker(r.Context(), common.Symbol(symbol))
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, ticker)
}

// GetKlines handles GET /api/v1/klines?symbol=ETH-USDT&interval=1h&limit=100
func (h *MarketHandler) GetKlines(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	interval := r.URL.Query().Get("interval")
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	klines, err := h.marketSvc.GetKlines(r.Context(), common.Symbol(symbol), interval, limit)
	if err != nil {
		WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, klines)
}

// GetAccount handles GET /api/v1/account
func (h *MarketHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// In production, this queries the account service
	WriteJSON(w, http.StatusOK, map[string]string{"userId": userID, "message": "use wallet service for balances"})
}

// WriteJSON serializes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}
