package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/config"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
	"github.com/exchange/internal/telemetry"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	telemetry.InitLogger(cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// In-memory event bus for MVP (use Kafka in production)
	eventBus := events.NewMemoryEventBus()

	// Create matching engine
	engine := matching.NewEngine(eventBus)

	// Register trading pairs
	symbols := []common.Symbol{"ETH-USDT"}
	for _, sym := range symbols {
		if err := engine.AddSymbol(sym); err != nil {
			log.Fatal().Err(err).Str("symbol", string(sym)).Msg("failed to add symbol")
		}
	}

	if err := engine.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start matching engine")
	}

	log.Info().Msg("matching engine running. Press Ctrl+C to stop.")
	<-ctx.Done()

	log.Info().Msg("shutting down matching engine")
	if err := engine.Stop(context.Background()); err != nil {
		log.Error().Err(err).Msg("error during shutdown")
	}
	eventBus.Close()
	os.Exit(0)
}
