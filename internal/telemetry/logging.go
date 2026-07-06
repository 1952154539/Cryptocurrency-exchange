package telemetry

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func InitLogger(env string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	if env == "development" || env == "test" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: "15:04:05.000",
		})
	} else {
		log.Logger = log.With().Caller().Logger()
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if env == "development" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

type contextKey string

const (
	traceIDKey contextKey = "trace_id"
	userIDKey  contextKey = "user_id"
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func Logger(ctx context.Context) *zerolog.Logger {
	logger := log.Logger
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		logger = logger.With().Str("trace_id", traceID).Logger()
	}
	if userID, ok := ctx.Value(userIDKey).(string); ok {
		logger = logger.With().Str("user_id", userID).Logger()
	}
	return &logger
}
