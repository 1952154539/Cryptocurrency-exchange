package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/settlement"
	"github.com/exchange/internal/telemetry"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	telemetry.InitLogger(cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Postgres)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr()})
	eventBus := events.NewRedisEventBus(rdb)
	defer eventBus.Close()

	feeSvc := settlement.NewFeeService(nil)
	settleSvc := settlement.NewService(pool, feeSvc, eventBus)

	if err := settleSvc.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start settlement service")
	}

	log.Info().Msg("settlement service running")
	<-ctx.Done()

	log.Info().Msg("settlement service shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = shutdownCtx
	eventBus.Close()
	os.Exit(0)
}
