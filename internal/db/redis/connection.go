package redis

import (
	"context"
	"fmt"

	"github.com/exchange/internal/config"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
)

// NewClient creates a Redis client.
func NewClient(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Info().
		Str("addr", cfg.Addr()).
		Int("db", cfg.DB).
		Msg("redis client created")

	return rdb, nil
}
