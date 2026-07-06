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

	authSvc := user.NewAuthService(user.JWTConfig{
		AccessSecret:  "change-in-production",
		RefreshSecret: "change-in-production",
	})

	_ = user.NewService(pool, authSvc)

	log.Info().Msg("user service running")
	<-ctx.Done()
	log.Info().Msg("user service shutting down")
	os.Exit(0)
}
