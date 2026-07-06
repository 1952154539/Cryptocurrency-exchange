package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exchange/internal/common/decimal"
	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/gateway"
	"github.com/exchange/internal/gateway/handler"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/marketdata"
	"github.com/exchange/internal/matching"
	"github.com/exchange/internal/order"
	"github.com/exchange/internal/telemetry"
	"github.com/exchange/internal/user"
	"github.com/exchange/internal/wallet"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	telemetry.InitLogger(cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Infrastructure
	eventBus := events.NewMemoryEventBus()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Warn().Err(err).Msg("postgres not available, running without persistence")
	} else {
		defer pool.Close()
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr()})

	// Matching engine
	engine := matching.NewEngine(eventBus)
	engine.AddSymbol("ETH-USDT")
	engine.Start(ctx)

	// JWT auth setup
	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")

	var authSvc *user.AuthService
	if privKeyPath := os.Getenv("JWT_PRIVATE_KEY_PATH"); privKeyPath != "" {
		pubKeyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
		if pubKeyPath == "" {
			log.Fatal().Msg("JWT_PUBLIC_KEY_PATH must be set when JWT_PRIVATE_KEY_PATH is set")
		}
		privKey, pubKey, err := user.LoadRSAKeyPair(privKeyPath, pubKeyPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load RSA keys")
		}
		authSvc = user.NewAuthService(user.JWTConfig{PrivateKey: privKey, PublicKey: pubKey})
	} else if accessSecret != "" {
		authSvc = user.NewAuthServiceWithSecrets(accessSecret, refreshSecret)
	} else if cfg.Env == "production" {
		log.Fatal().Msg("JWT_PRIVATE_KEY_PATH or JWT_ACCESS_SECRET must be set in production")
	} else {
		authSvc = user.NewAuthServiceWithSecrets("dev-access-secret", "dev-refresh-secret")
		log.Warn().Msg("using default JWT secrets - not for production use")
	}

	// Order service with real dependencies
	var orderSvc *order.Service
	if pool != nil {
		markets := defaultMarkets()
		balanceProvider := order.NewDbBalanceProvider(pool)
		orderRepo := order.NewRepository(pool)
		orderSvc = order.NewService(
			order.NewValidator(markets, balanceProvider, nil),
			orderRepo,
			engine,
			eventBus,
		)
	} else {
		orderSvc = order.NewService(
			order.NewValidator(nil, nil, nil),
			nil,
			engine,
			eventBus,
		)
	}

	marketSvc := marketdata.NewService(rdb, engine)
	if err := marketSvc.Start(ctx, eventBus); err != nil {
		log.Error().Err(err).Msg("failed to start market data subscription")
	}

	// Handlers
	orderHandler := handler.NewOrderHandler(orderSvc)
	marketHandler := handler.NewMarketHandler(marketSvc)

	var walletHandler *handler.WalletHandler
	if pool != nil {
		walletSvc := wallet.NewService(pool, eventBus)
		if seedHex := os.Getenv("WALLET_MASTER_SEED_HEX"); seedHex != "" {
			if err := walletSvc.LoadMasterSeed(seedHex); err != nil {
				log.Fatal().Err(err).Msg("failed to load wallet master seed")
			}
		} else if cfg.Env == "production" {
			log.Fatal().Msg("WALLET_MASTER_SEED_HEX must be set in production")
		} else {
			log.Warn().Msg("WALLET_MASTER_SEED_HEX not set, using random keys (dev only)")
		}
		walletHandler = handler.NewWalletHandler(walletSvc)
	}

	// Rate limiter with cleanup goroutine
	rateLimiter := middleware.NewRateLimiter(100, 50)
	go rateLimiter.StartCleanup(ctx, 1*time.Minute, 5*time.Minute)

	// Health checks
	var healthChecks map[string]func() error
	if pool != nil {
		healthChecks = make(map[string]func() error)
		healthChecks["postgres"] = func() error { return pool.Ping(ctx) }
		healthChecks["redis"] = func() error { return rdb.Ping(ctx).Err() }
	}
	router := gateway.NewRouter(authSvc, orderHandler, marketHandler, walletHandler, rateLimiter, healthChecks)

	// HTTP Server
	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.ReadTimeout,
	}

	go func() {
		log.Info().Str("addr", addr).Msg("API gateway listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutting down API gateway")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	engine.Stop(shutdownCtx)
	eventBus.Close()
	os.Exit(0)
}

func defaultMarkets() []*order.MarketConfig {
	return []*order.MarketConfig{
		{
			Symbol:        "ETH-USDT",
			BaseCurrency:  "ETH",
			QuoteCurrency: "USDT",
			PriceTick:     decimalMust("0.01"),
			QtyStep:       decimalMust("0.0001"),
			MinOrderQty:   decimalMust("0.001"),
			MaxOrderQty:   decimalMust("100"),
			MinNotional:   decimalMust("10"),
			Status:        "active",
		},
	}
}

func decimalMust(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
