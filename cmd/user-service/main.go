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
	"github.com/exchange/internal/user"
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

	eventBus := createEventBus(cfg, rdb, "user-service")
	defer eventBus.Close()

	authSvc := loadAuthService(cfg)
	userSvc := user.NewService(pool, authSvc, cfg.Env == "production")

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	pb.RegisterUserServiceServer(grpcServer, internalgrpc.NewUserServer(userSvc, authSvc))

	addr := fmt.Sprintf(":%d", cfg.Server.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err).Str("addr", addr).Msg("failed to listen")
	}

	go func() {
		log.Info().Str("addr", addr).Msg("user-service gRPC listening")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("gRPC server failed")
		}
	}()

	log.Info().Msg("user service running")
	<-ctx.Done()
	log.Info().Msg("user service shutting down")
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

func loadAuthService(cfg *config.Config) *user.AuthService {
	accessSecret := os.Getenv("JWT_ACCESS_SECRET")
	refreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if privKeyPath := os.Getenv("JWT_PRIVATE_KEY_PATH"); privKeyPath != "" {
		pubKeyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
		if pubKeyPath == "" {
			log.Fatal().Msg("JWT_PUBLIC_KEY_PATH required")
		}
		privKey, pubKey, err := user.LoadRSAKeyPair(privKeyPath, pubKeyPath)
		if err != nil {
			log.Fatal().Err(err).Msg("load RSA keys")
		}
		return user.NewAuthService(user.JWTConfig{PrivateKey: privKey, PublicKey: pubKey})
	} else if accessSecret != "" {
		return user.NewAuthServiceWithSecrets(accessSecret, refreshSecret)
	} else if cfg.Env == "production" {
		log.Fatal().Msg("JWT keys required in production")
	}
	log.Warn().Msg("using dev JWT secrets")
	return user.NewAuthServiceWithSecrets("dev-access-secret", "dev-refresh-secret")
}
