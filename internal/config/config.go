package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env      string
	Postgres PostgresConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	Server   ServerConfig
	Chains   []ChainConfig
}

type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	MaxConns int
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type KafkaConfig struct {
	Brokers []string
	GroupID string
}

type ServerConfig struct {
	GRPCPort    int
	HTTPPort    int
	WSPort      int
	MetricsPort int
	ReadTimeout time.Duration
}

type ChainConfig struct {
	Name              string
	ChainID           uint64
	RPCURLs           []string
	ConfirmationDepth uint64
	MaxReorgDepth     uint64
	PollInterval      time.Duration
}

func Load() *Config {
	return &Config{
		Env: envOrDefault("ENV", "development"),
		Postgres: PostgresConfig{
			Host:     envOrDefault("PG_HOST", "localhost"),
			Port:     envIntOrDefault("PG_PORT", 5432),
			User:     envOrDefault("PG_USER", "exchange"),
			Password: envOrDefault("PG_PASSWORD", "exchange_dev"),
			Database: envOrDefault("PG_DATABASE", "exchange"),
			MaxConns: envIntOrDefault("PG_MAX_CONNS", 50),
		},
		Redis: RedisConfig{
			Host:     envOrDefault("REDIS_HOST", "localhost"),
			Port:     envIntOrDefault("REDIS_PORT", 6379),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envIntOrDefault("REDIS_DB", 0),
		},
		Kafka: KafkaConfig{
			Brokers: []string{envOrDefault("KAFKA_BROKERS", "localhost:9092")},
			GroupID: envOrDefault("KAFKA_GROUP_ID", "exchange"),
		},
		Server: ServerConfig{
			GRPCPort:    envIntOrDefault("GRPC_PORT", 50051),
			HTTPPort:    envIntOrDefault("HTTP_PORT", 8080),
			WSPort:      envIntOrDefault("WS_PORT", 8081),
			MetricsPort: envIntOrDefault("METRICS_PORT", 9090),
			ReadTimeout: 30 * time.Second,
		},
		Chains: []ChainConfig{
			{
				Name:              "ethereum",
				ChainID:           1,
				RPCURLs:           []string{envOrDefault("ETH_RPC_URL", "http://localhost:8545")},
				ConfirmationDepth: 12,
				MaxReorgDepth:     12,
				PollInterval:      5 * time.Second,
			},
		},
	}
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
