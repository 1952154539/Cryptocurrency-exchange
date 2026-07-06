package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/exchange/internal/config"
	"github.com/exchange/internal/db/postgres"
	"github.com/exchange/internal/telemetry"
	"github.com/exchange/internal/user"
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

	_ = user.NewService(pool, authSvc, cfg.Env == "production")

	log.Info().Msg("user service running")
	<-ctx.Done()
	log.Info().Msg("user service shutting down")
	os.Exit(0)
}
