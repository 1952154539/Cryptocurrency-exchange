package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/exchange/internal/common"
	"github.com/exchange/internal/config"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/matching"
	"github.com/exchange/internal/telemetry"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.Load()
	telemetry.InitLogger(cfg.Env)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr()})

	eventBus := createEventBus(cfg, rdb, "matching-engine")
	defer eventBus.Close()

	engine := matching.NewEngine(eventBus)

	symbols := []common.Symbol{"ETH-USDT"}
	for _, sym := range symbols {
		if err := engine.AddSymbol(sym); err != nil {
			log.Fatal().Err(err).Str("symbol", string(sym)).Msg("failed to add symbol")
		}
	}

	if err := engine.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start matching engine")
	}

	log.Info().Msg("matching engine running")
	<-ctx.Done()

	log.Info().Msg("shutting down matching engine")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := engine.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("error during shutdown")
	}
	eventBus.Close()
	os.Exit(0)
}

func createEventBus(cfg *config.Config, rdb *redis.Client, groupID string) events.EventBus {
	if len(cfg.Kafka.Brokers) > 0 && cfg.Kafka.Brokers[0] != "" {
		return events.NewKafkaEventBus(events.KafkaConfig{
			Brokers: cfg.Kafka.Brokers,
			GroupID: groupID,
		})
	}
	return events.NewRedisEventBus(rdb)
}
