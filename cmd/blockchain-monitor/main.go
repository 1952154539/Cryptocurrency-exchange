package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"github.com/exchange/internal/blockchain/ethereum"
	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
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

	// Connect to Ethereum node
	var ethClient *ethereum.Client
	if len(cfg.Chains) > 0 {
		ethClient, err = ethereum.NewClient(cfg.Chains[0].RPCURLs[0])
		if err != nil {
			log.Fatal().Err(err).Msg("failed to connect to ethereum node")
		}
		defer ethClient.Close()
	}

	// Event bus for deposit events
	eventBus := events.NewRedisEventBus(rdb)
	defer eventBus.Close()

	// Scanner monitors blockchain for deposits
	scanner := ethereum.NewScanner(ethClient, cfg.Chains[0].ConfirmationDepth)

	// Load watched addresses from DB
	rows, err := pool.Query(ctx, `SELECT user_id, address FROM deposit_addresses WHERE chain = 'ethereum'`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var userID, address string
			if err := rows.Scan(&userID, &address); err == nil {
				scanner.WatchAddress(address, userID)
			}
		}
	}

	log.Info().Msg("blockchain monitor running")

	// Poll for new blocks
	ticker := time.NewTicker(cfg.Chains[0].PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			deposits, err := scanner.ScanNewBlocks(ctx)
			if err != nil {
				log.Warn().Err(err).Msg("block scan error")
				continue
			}

			for _, d := range deposits {
				evt := &events.Event{
					ID:        events.NewEventID(),
					Type:      events.EventDepositDetected,
					Timestamp: time.Now(),
				}
				evt.SetPayload(events.DepositDetectedPayload{
					TxHash:               d.TxHash,
					Currency:             d.Currency,
					Chain:                d.Chain,
					FromAddress:          d.FromAddress,
					ToAddress:            d.ToAddress,
					Amount:               d.Amount,
					BlockNumber:          d.BlockNumber,
					Confirmations:        d.Confirmations,
					RequiredConfirmations: d.RequiredConfirmations,
				})

				if err := eventBus.Publish(ctx, "deposit.detected", evt); err != nil {
					log.Warn().Err(err).Msg("failed to publish deposit event")
				}
			}

		case <-ctx.Done():
			log.Info().Msg("blockchain monitor shutting down")
			return
		}
	}
}
