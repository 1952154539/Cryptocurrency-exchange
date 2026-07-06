package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/settlement"
	"github.com/exchange/internal/telemetry"
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

	eventBus := events.NewMemoryEventBus()
	feeSvc := settlement.NewFeeService(nil)
	settleSvc := settlement.NewService(pool, feeSvc, eventBus)

	if err := settleSvc.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start settlement service")
	}

	log.Info().Msg("settlement service running")
	<-ctx.Done()
	log.Info().Msg("settlement service shutting down")
	os.Exit(0)
}
