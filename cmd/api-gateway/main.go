package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/exchange/internal/config"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/gateway"
	"github.com/exchange/internal/gateway/handler"
	"github.com/exchange/internal/gateway/middleware"
	"github.com/exchange/internal/marketdata"
	"github.com/exchange/internal/matching"
	"github.com/exchange/internal/order"
	"github.com/exchange/internal/user"
	"github.com/exchange/internal/telemetry"
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
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr()})

	// Matching engine
	engine := matching.NewEngine(eventBus)
	engine.AddSymbol("ETH-USDT")
	engine.Start(ctx)

	// Services
	authSvc := user.NewAuthService(user.JWTConfig{
		AccessSecret:  "dev-access-secret-change-in-production",
		RefreshSecret: "dev-refresh-secret-change-in-production",
	})

	orderSvc := order.NewService(
		order.NewValidator(nil, nil, nil), // In production, provide real deps
		nil, // order repository
		engine,
		eventBus,
	)

	marketSvc := marketdata.NewService(rdb, engine)

	// Handlers
	orderHandler := handler.NewOrderHandler(orderSvc)
	marketHandler := handler.NewMarketHandler(marketSvc)

	// Router
	rateLimiter := middleware.NewRateLimiter(100, 50)
	router := gateway.NewRouter(authSvc, orderHandler, marketHandler, rateLimiter)

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
	srv.Shutdown(context.Background())
	engine.Stop(context.Background())
	eventBus.Close()
	os.Exit(0)
}
