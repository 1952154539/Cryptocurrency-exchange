package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/exchange/api/proto/gen"
	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/events"
	internalgrpc "github.com/exchange/internal/grpc"
	"github.com/exchange/internal/telemetry"
	"github.com/exchange/internal/wallet"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	eventBus := createEventBus(cfg, rdb, "wallet-service")
	defer eventBus.Close()

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

	_ = eventBus.Subscribe(ctx, "deposit.detected", "wallet", func(ctx context.Context, evt *events.Event) error {
		var deposit wallet.DepositEvent
		if err := evt.GetPayload(&deposit); err != nil {
			return err
		}
		return walletSvc.ProcessDeposit(ctx, &deposit)
	})

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	pb.RegisterWalletServiceServer(grpcServer, internalgrpc.NewWalletServer(walletSvc))

	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("failed to listen")
	}

	go func() {
		log.Info().Str("addr", addr).Msg("wallet-service gRPC listening")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	log.Info().Msg("wallet service running")
	<-ctx.Done()
	log.Info().Msg("wallet service shutting down")
	grpcServer.GracefulStop()
	eventBus.Close()
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
