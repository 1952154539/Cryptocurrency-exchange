package gateway

import (
	"net/http"
	"time"

	"github.com/exchange/internal/gateway/handler"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/user"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the HTTP router with all routes.
func NewRouter(
	authSvc *user.AuthService,
	orderHandler *handler.OrderHandler,
	marketHandler *handler.MarketHandler,
	walletHandler *handler.WalletHandler,
	rateLimiter *middleware.RateLimiter,
) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(rateLimiter.Middleware)

	// Public market data endpoints
	r.Route("/api/v1", func(r chi.Router) {
		// Public - no auth required
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"status":"ok"}`))
		})
		r.Get("/time", func(w http.ResponseWriter, r *http.Request) {
			handler.WriteJSON(w, http.StatusOK, map[string]int64{
				"serverTime": time.Now().UnixMilli(),
			})
		})
		r.Get("/depth", marketHandler.GetDepth)
		r.Get("/trades", marketHandler.GetTrades)
		r.Get("/klines", marketHandler.GetKlines)
		r.Get("/ticker/24hr", marketHandler.GetTicker)

		// Private - JWT auth required
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(authSvc))

			r.Get("/account", marketHandler.GetAccount)
			r.Post("/order", orderHandler.PlaceOrder)
			r.Delete("/order", orderHandler.CancelOrder)
			r.Get("/order", orderHandler.GetOrder)
			r.Get("/open-orders", orderHandler.GetOpenOrders)

			if walletHandler != nil {
				r.Get("/wallet/balances", walletHandler.GetBalances)
				r.Post("/wallet/deposit-address", walletHandler.GetDepositAddress)
				r.Post("/wallet/withdraw", walletHandler.Withdraw)
			}
		})

		// HMAC auth: use middleware.HMACAuth(keyGetter) to enable API-key based auth
	})

	return r
}
