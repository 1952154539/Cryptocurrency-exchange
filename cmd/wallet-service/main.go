package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
	"github.com/exchange/internal/telemetry"
	"github.com/exchange/internal/wallet"
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

	walletSvc := wallet.NewService(pool, eventBus)

	// Load master seed for HD wallet
	if seedHex := os.Getenv("WALLET_MASTER_SEED_HEX"); seedHex != "" {
		if err := walletSvc.LoadMasterSeed(seedHex); err != nil {
			log.Fatal().Err(err).Msg("failed to load wallet master seed")
		}
	} else if cfg.Env == "production" {
		log.Fatal().Msg("WALLET_MASTER_SEED_HEX must be set in production")
	} else {
		log.Warn().Msg("WALLET_MASTER_SEED_HEX not set, using random keys (dev only)")
	}

	// Subscribe to deposit events from blockchain monitor
	_ = eventBus.Subscribe(ctx, "deposit.detected", "wallet", func(ctx context.Context, evt *events.Event) error {
		var deposit wallet.DepositEvent
		if err := evt.GetPayload(&deposit); err != nil {
			log.Warn().Err(err).Msg("failed to unmarshal deposit event")
			return err
		}
		return walletSvc.ProcessDeposit(ctx, &deposit)
	})

	log.Info().Msg("wallet service running")
	<-ctx.Done()

	log.Info().Msg("wallet service shutting down")
	eventBus.Close()
	os.Exit(0)
}
