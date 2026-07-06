package handler

import (
	"encoding/json"
	"net/http"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/wallet"
)

// WalletHandler handles wallet-related HTTP requests.
type WalletHandler struct {
	walletSvc *wallet.Service
}

// NewWalletHandler creates a wallet handler.
func NewWalletHandler(walletSvc *wallet.Service) *WalletHandler {
	return &WalletHandler{walletSvc: walletSvc}
}

// GetDepositAddress handles POST /api/v1/wallet/deposit-address
func (h *WalletHandler) GetDepositAddress(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Currency string `json:"currency"`
		Chain    string `json:"chain"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	address, err := h.walletSvc.GenerateDepositAddress(r.Context(), userID, common.Currency(req.Currency), common.Chain(req.Chain))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"address": address,
		"chain":   req.Chain,
	})
}

// Withdraw handles POST /api/v1/wallet/withdraw
func (h *WalletHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Currency  string `json:"currency"`
		Chain     string `json:"chain"`
		ToAddress string `json:"toAddress"`
		Amount    string `json:"amount"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid amount")
		return
	}

	result, err := h.walletSvc.RequestWithdrawal(r.Context(), &wallet.WithdrawalRequest{
		UserID:    userID,
		Currency:  req.Currency,
		Chain:     req.Chain,
		ToAddress: req.ToAddress,
		Amount:    amount,
		Fee:       withdrawalFee(common.Currency(req.Currency)),
	})
	if err != nil {
		WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"withdrawalId": result.WithdrawalID,
		"status":       result.Status,
	})
}

// GetBalances handles GET /api/v1/wallet/balances
func (h *WalletHandler) GetBalances(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// For now, return a placeholder - full implementation queries accounts table
	WriteJSON(w, http.StatusOK, map[string]string{
		"message": "use /api/v1/account for balance info",
	})
}

// withdrawalFee returns a flat withdrawal fee per currency.
func withdrawalFee(currency common.Currency) decimal.Decimal {
	switch currency {
	case "ETH":
		d, _ := decimal.NewFromString("0.001")
		return d
	case "USDT":
		d, _ := decimal.NewFromString("1")
		return d
	default:
		return decimal.Zero
	}
}
