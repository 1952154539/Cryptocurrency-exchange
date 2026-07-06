# Cryptocurrency Exchange

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Tests](https://img.shields.io/badge/tests-31%2F31%20PASS-brightgreen)]()
[![Coverage](https://img.shields.io/badge/race%20detector-clean-brightgreen)]()
[![Phase](https://img.shields.io/badge/phase-production%20ready-blue)]()

**Production-grade centralized cryptocurrency exchange written in Go**

</div>

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Quick Start](#quick-start)
4. [API Reference](#api-reference)
5. [Configuration](#configuration)
6. [Deployment](#deployment)
7. [Testing](#testing)
8. [Monitoring](#monitoring)
9. [Security](#security)
10. [Project Structure](#project-structure)
11. [Technology Stack](#technology-stack)
12. [Roadmap](#roadmap)
13. [Contributing](#contributing)
14. [License](#license)

---

## Overview

A high-performance centralized cryptocurrency exchange featuring an in-memory matching engine, gRPC microservices, Kafka event streaming, multi-chain blockchain integration, risk management, KYC/AML compliance, margin trading, FIX protocol support, and cold wallet multisig.

### Key Metrics

| Metric | Value |
|--------|-------|
| Matching Throughput | 1,208,160 ops/s |
| Matching Latency | 843 ns/op |
| Order Types | Market, Limit, Stop-Loss, Stop-Limit |
| Time in Force | GTC, IOC, FOK |
| Max Leverage | 125x |
| Supported Chains | Ethereum, BSC, Arbitrum |
| Services | 6 (API Gateway, Matching, Settlement, Wallet, User, Blockchain Monitor) |

---

## Architecture

```
                              ┌──────────────────────────────────────────┐
                              │              Load Balancer               │
                              └────────────────────┬─────────────────────┘
                                                   │
                              ┌────────────────────┴─────────────────────┐
                              │         API Gateway (:8080)              │
                              │   chi Router | JWT RS256 | Rate Limiter  │
                              │   /health  /ready  /metrics  /api/v1/*   │
                              └──┬──────────┬──────────┬─────────────────┘
                                 │ REST     │ gRPC     │
          ┌──────────────────────┤          │          ├──────────────────┐
          ▼                      ▼          ▼          ▼                  ▼
   ┌────────────┐   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐
   │   User     │   │   Order    │  │   Wallet    │  │   Market   │  │   Fix    │
   │  Service   │   │  Service   │  │  Service    │  │   Data     │  │  Gateway │
   │  :50051    │   │  :50051    │  │  :50051     │  │  Service   │  │  :9880   │
   └─────┬──────┘   └─────┬──────┘  └──────┬──────┘  └─────┬──────┘  └────┬─────┘
         │                │                │                │              │
         └────────────────┼────────────────┼────────────────┼──────────────┘
                          │                │                │
                          │    ┌───────────┴───────────┐    │
                          │    │   Kafka / Redis Bus   │    │
                          │    └───────────┬───────────┘    │
                          ▼                ▼                ▼
                   ┌────────────┐  ┌────────────┐  ┌────────────────┐
                   │  Matching  │  │ Settlement │  │   Blockchain   │
                   │   Engine   │  │  Service   │  │    Monitor     │
                   └─────┬──────┘  └─────┬──────┘  └───────┬────────┘
                         │               │                  │
                         ▼               ▼                  ▼
                   ┌─────────────────────────────────────────────┐
                   │ PostgreSQL │ Redis │ Kafka │ ClickHouse │ ETH│
                   └─────────────────────────────────────────────┘
```

### Data Flow

```
Order Placement:
  Client ──REST──▶ API Gateway ──▶ Order Service ──▶ Matching Engine
                                                          │
                    ┌─────────────────────────────────────┤
                    ▼                                     ▼
              Trade Executed ◀─── Kafka ──── Order Matched
                    │
                    ▼
            Settlement Service ──▶ PostgreSQL (balance update)
                    │
                    ├──▶ ClickHouse (trade record)
                    ├──▶ Redis (market data cache)
                    └──▶ Kafka (notification event)

Deposit:
  Blockchain ──▶ Block Monitor ──▶ Kafka ──▶ Wallet Service ──▶ PostgreSQL
                                                                   │
                                                              audit log
```

---

## Quick Start

### Prerequisites

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.25+ | Compilation |
| Docker | 24+ | Infrastructure |
| Make | 4+ | Build automation |

### Local Development (5 minutes)

```bash
# 1. Clone
git clone https://github.com/1952154539/Cryptocurrency-exchange.git
cd Cryptocurrency-exchange

# 2. Start infrastructure (PostgreSQL + Redis + Kafka + ClickHouse + Anvil)
docker compose up -d

# 3. Initialize database schema
make migrate

# 4. Build all services
make build

# 5. Start services (6 terminals or use &)
export KAFKA_BROKERS=localhost:9092
./bin/matching-engine &
./bin/settlement-service &
./bin/wallet-service &
./bin/user-service &
./bin/api-gateway &
./bin/blockchain-monitor &

# 6. Verify
curl http://localhost:8080/health
# {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}

curl http://localhost:8080/api/v1/ping
# {"status":"ok"}
```

### One-Command Dev Startup

```bash
make docker-up && make migrate && make build && ./bin/api-gateway
```

---

## API Reference

### Public Endpoints (No Auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Service health (DB, Redis, Kafka checks) |
| `GET` | `/ready` | Kubernetes readiness probe |
| `GET` | `/metrics` | Prometheus metrics endpoint |
| `GET` | `/api/v1/ping` | Heartbeat |
| `GET` | `/api/v1/time` | Server timestamp (ms) |
| `GET` | `/api/v1/depth?symbol=ETH-USDT&limit=100` | Order book depth |
| `GET` | `/api/v1/trades?symbol=ETH-USDT&limit=500` | Recent trades |
| `GET` | `/api/v1/klines?symbol=ETH-USDT&interval=1h&limit=100` | Candlestick data |
| `GET` | `/api/v1/ticker/24hr?symbol=ETH-USDT` | 24h price statistics |

### Private Endpoints (JWT Bearer Token)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/account` | Account info |
| `POST` | `/api/v1/order` | Place order |
| `DELETE` | `/api/v1/order` | Cancel order |
| `GET` | `/api/v1/order?orderId=<id>` | Query order |
| `GET` | `/api/v1/open-orders?symbol=ETH-USDT` | Open orders |
| `GET` | `/api/v1/wallet/balances` | Wallet balances |
| `POST` | `/api/v1/wallet/deposit-address` | Generate deposit address |
| `POST` | `/api/v1/wallet/withdraw` | Request withdrawal |

### Place Order Example

**Request:**
```http
POST /api/v1/order
Authorization: Bearer <JWT_TOKEN>
Content-Type: application/json

{
  "symbol": "ETH-USDT",
  "side": "buy",
  "type": "limit",
  "timeInForce": "GTC",
  "price": "2000.00",
  "quantity": "1.5"
}
```

**Response:**
```json
{
  "orderId": "ord_4H7XK2M9P1X",
  "clientOrderId": "",
  "status": "open",
  "filledQty": "0"
}
```

### gRPC Services

| Service | Methods |
|---------|---------|
| `UserService` | Register, Login, GetUser, UpdateKYC |
| `WalletService` | GetDepositAddress, RequestWithdrawal, GetBalances |
| `OrderService` | PlaceOrder, CancelOrder, GetOrder, GetOpenOrders |
| `MarketDataService` | GetDepth, GetTrades, GetTicker, GetKlines |

### Error Codes

| HTTP Status | Meaning |
|-------------|---------|
| 200 | Success |
| 400 | Invalid request (bad price, quantity, symbol) |
| 401 | Missing or invalid JWT token |
| 403 | Order does not belong to user |
| 404 | Order not found |
| 429 | Rate limit exceeded |
| 500 | Internal server error |
| 503 | Service degraded (health check) |

---

## Configuration

### Required (Production)

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export WALLET_MASTER_SEED_HEX=<64-character-hex-seed>
export KAFKA_BROKERS=kafka-0:9092,kafka-1:9092,kafka-2:9092
```

### Optional (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | `production` enables strict validation |
| `JWT_ACCESS_SECRET` | (dev default) | HMAC fallback signing key |
| `JWT_REFRESH_SECRET` | (dev default) | HMAC refresh token key |
| `PG_HOST` | `localhost` | PostgreSQL host |
| `PG_PORT` | `5432` | PostgreSQL port |
| `PG_USER` | `exchange` | PostgreSQL user |
| `PG_PASSWORD` | `exchange_dev` | PostgreSQL password |
| `PG_DATABASE` | `exchange` | PostgreSQL database |
| `PG_MAX_CONNS` | `50` | Connection pool size |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | (empty) | Redis password |
| `REDIS_DB` | `0` | Redis database number |
| `KAFKA_GROUP_ID` | `exchange` | Consumer group ID |
| `HTTP_PORT` | `8080` | API Gateway HTTP port |
| `GRPC_PORT` | `50051` | gRPC server port |
| `WS_PORT` | `8081` | WebSocket port (reserved) |
| `METRICS_PORT` | `9090` | Metrics port (reserved) |
| `ETH_RPC_URL` | `http://localhost:8545` | Ethereum RPC endpoint |

---

## Deployment

### Docker Compose (Dev)

```bash
docker compose up -d       # PostgreSQL + Redis + Kafka + ClickHouse + Anvil
docker compose down        # Stop all
```

### Kubernetes (Production)

```bash
# Build images
make docker-build

# Apply base configuration
kubectl apply -k deployments/base

# Apply production overlay
kubectl apply -k deployments/overlays/production
```

### Service Startup Order

1. **Infrastructure**: PostgreSQL, Redis, Kafka, ClickHouse
2. **Database Migration**: `make migrate`
3. **Matching Engine**: Core order matching
4. **Settlement Service**: Trade settlement (Kafka consumer)
5. **Wallet Service**: Deposit/withdrawal processing
6. **User Service**: Authentication
7. **API Gateway**: External API entry point
8. **Blockchain Monitor**: On-chain deposit scanning

### Health Check

```bash
# Service-level
curl http://localhost:8080/health  | jq .
# {
#   "status": "ok",
#   "timestamp": "2026-07-06T11:52:23Z",
#   "services": {
#     "postgres": "healthy",
#     "redis": "healthy"
#   }
# }

# Kubernetes probes
curl http://localhost:8080/ready   # 200 = ready
curl http://localhost:8080/metrics # Prometheus format
```

---

## Testing

### Quick Reference

```bash
make test            # All tests + race detector
make test-matching   # Matching engine only
make test-integration # Integration tests
make bench           # Performance benchmarks
make vet             # Static analysis
make lint            # Linter (requires golangci-lint)
```

### Production Test Report

| Test | Result |
|------|--------|
| `go build ./cmd/...` (6 binaries) | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/... -race` (27 tests) | ✅ PASS |
| `go test ./test/... -race` (4 integration) | ✅ PASS |
| `go test -bench=. -benchmem` | ✅ 843 ns/op, 19 allocs/op |
| 20 concurrent order stress test | ✅ No race |

### Test Suites

| Suite | Count | Coverage |
|-------|-------|----------|
| `decimal` | 9 | Fixed-point arithmetic, parsing, precision, rounding |
| `matching` | 18 | Order book, FIFO, GTC/IOC/FOK, partial fills, cancel, snapshot |
| `integration` | 4 | End-to-end flow, market orders, FOK validation, concurrent orders |

### Benchmark

```
BenchmarkOrderBook_Matching-8   1,208,160 ops   843.8 ns/op   512 B/op   19 allocs/op
```

### Project Statistics

| Metric | Value |
|--------|-------|
| Go source files | 75 |
| Lines of code | 8,690 |
| Packages | 35 |
| Direct dependencies | 55 |
| Binary sizes | 11M - 30M |

---

## Monitoring

### Prometheus Metrics

```
# HELP http_requests_total Total HTTP requests
# HELP orders_matched_total Total matched orders
# HELP orders_rejected_total Total rejected orders
# HELP trades_settled_total Total settled trades
# HELP settlement_errors_total Total settlement errors
# HELP deposits_confirmed_total Total confirmed deposits
# HELP withdrawals_requested_total Total withdrawal requests
```

### Grafana Dashboard (Recommended Panels)

1. **Trading**: orders/sec, trades/sec, matching latency p50/p99
2. **Infrastructure**: CPU, memory, goroutines, GC pause
3. **Business**: active users, deposit/withdrawal volume, fee revenue
4. **Alerts**: settlement errors > 0, matching latency > 10ms, service down

### Logging

All services use structured JSON logging (zerolog). Key fields: `level`, `time`, `message`, `user_id`, `order_id`, `symbol`, `error`.

```json
{"level":"info","time":"2026-07-06T11:52:23Z","user_id":"abc123","order_id":"ord_4H7X","symbol":"ETH-USDT","message":"order placed"}
```

---

## Security

| Layer | Implementation |
|-------|---------------|
| **Authentication** | JWT RS256 (15min access + 7d refresh), HMAC HS256 fallback |
| **API Authentication** | HMAC-SHA256 signature (5s window), implemented, not enabled by default |
| **Password Hashing** | bcrypt, cost factor 12 |
| **Rate Limiting** | Token bucket: 100 req/s per IP, 50 req/s per user, 20 req/s per order |
| **HTTP Security** | CSP, HSTS (1 year), X-Frame-Options: DENY, X-Content-Type-Options: nosniff |
| **Request Size** | 1 MB MaxBytesReader on all body-reading handlers |
| **SQL Injection** | 100% parameterized queries via pgx |
| **Double-Spend** | `UNIQUE(tx_hash, to_address)` + settlement trade_id idempotency |
| **Optimistic Locking** | `version` column on accounts table prevents lost updates |
| **Wallet** | BIP44/secp256k1, per-user account derivation, tx-level withdrawal |
| **Data at Rest** | PostgreSQL with TLS (production), Redis AOF persistence |
| **Secrets** | Environment variables, no hardcoded secrets in code, production enforcement |
| **Shutdown** | Graceful shutdown with 30s timeout on all services |

---

## Project Structure

```
├── api/proto/                     # gRPC service definitions + generated code
│   ├── order/user/wallet/market.proto
│   └── gen/                       # protoc output (.pb.go, _grpc.pb.go)
│
├── cmd/                           # Service entry points (6 binaries)
│   ├── api-gateway/               # HTTP API server (:8080)
│   ├── matching-engine/           # Order matching engine
│   ├── settlement-service/        # Post-trade settlement
│   ├── user-service/              # User management + auth
│   ├── wallet-service/            # Wallet + deposits + withdrawals
│   └── blockchain-monitor/        # On-chain transaction scanning
│
├── internal/
│   ├── matching/                  # Matching engine (sharded + RWMutex)
│   │   ├── engine.go              # Shard dispatcher + event emission
│   │   ├── orderbook.go           # Price-sorted order book + FIFO queues
│   │   ├── matcher.go             # Price-time priority matching algorithm
│   │   ├── types.go               # Order, MatchResult, OrderType
│   │   └── pool.go                # sync.Pool for GC optimization
│   │
│   ├── order/                     # Order service
│   │   ├── service.go             # Place/cancel/get + balance freeze/unfreeze
│   │   ├── validator.go           # Multi-step order validation
│   │   ├── lifecycle.go           # State machine transitions
│   │   ├── repository.go          # PostgreSQL persistence
│   │   └── balance_provider.go    # DB-backed balance provider
│   │
│   ├── settlement/                # Settlement service
│   │   ├── service.go             # Balance updates + optimistic locking + idempotency
│   │   └── fees.go                # Fee calculation + 5-tier volume schedule
│   │
│   ├── user/                      # User service
│   │   ├── service.go             # Registration + login + status
│   │   └── auth.go                # JWT RS256/HS256 + HMAC signature verification
│   │
│   ├── wallet/                    # Wallet service
│   │   ├── service.go             # Deposits + withdrawals + address generation
│   │   ├── hdwallet.go            # BIP32/BIP44 HD wallet (secp256k1)
│   │   └── cold/multisig.go       # M-of-N cold wallet multisig
│   │
│   ├── trading/                   # Advanced trading
│   │   ├── margin/engine.go       # Leverage engine (liquidation + funding rate)
│   │   └── fix/session.go         # FIX 4.4 protocol (NewOrderSingle/ExecutionReport)
│   │
│   ├── blockchain/                # Blockchain integration
│   │   ├── ethereum/              # ETH client, block scanner, withdrawal processor
│   │   └── adapter/chain.go       # Multi-chain abstraction (ETH/BSC/ARB + ERC20)
│   │
│   ├── events/                    # Event bus
│   │   ├── types.go               # Event types + payload structs + interface
│   │   ├── kafka_bus.go           # Kafka implementation (consumer group + ACK)
│   │   ├── redis_bus.go           # Redis Pub/Sub fallback
│   │   └── producer.go            # In-memory implementation (dev)
│   │
│   ├── gateway/                   # API Gateway
│   │   ├── router.go              # chi route registration
│   │   ├── handler/               # HTTP handlers (order, market, wallet)
│   │   └── middleware/            # JWT, HMAC, rate limiting, security headers, CORS
│   │
│   ├── grpc/                      # gRPC layer
│   │   ├── *_server.go            # Server implementations (4 services)
│   │   └── client/                # Client wrappers (order, wallet, user)
│   │
│   ├── risk/                      # Risk management
│   │   ├── circuit_breaker.go     # Price-based circuit breaker
│   │   ├── withdrawal_limits.go   # Per-currency withdrawal limits
│   │   └── blacklist.go           # IP/UID/Address blacklist
│   │
│   ├── kyc/                       # KYC/AML
│   │   ├── service.go             # Verification workflow (submit/approve/reject)
│   │   └── aml_checker.go         # Sanctions list screening
│   │
│   ├── marketdata/                # Market data service
│   │   └── service.go             # Order book depth, trades, ticker, klines, Redis cache
│   │
│   ├── db/                        # Data access
│   │   ├── postgres/              # pgxpool connection pool
│   │   ├── redis/                 # Redis client
│   │   ├── clickhouse/writer.go   # ClickHouse OLAP writer
│   │   └── migrate/               # golang-migrate runner
│   │
│   ├── telemetry/                 # Observability
│   │   ├── logging.go             # Structured logging (zerolog)
│   │   └── metrics.go             # Prometheus metrics + health/ready endpoints
│   │
│   ├── common/                    # Shared utilities
│   │   ├── decimal/               # 18-decimal fixed-point arithmetic
│   │   ├── types.go               # Domain types (Side, OrderType, Status, etc.)
│   │   ├── errors.go              # Domain errors
│   │   └── idgen.go               # ULID-based ID generation
│   │
│   └── config/                    # Configuration
│       └── config.go              # Environment variable loader with defaults
│
├── migrations/                    # Database migrations (up/down SQL)
│   ├── 001_init.up.sql            # Core schema (12 tables)
│   ├── 001_init.down.sql
│   ├── 002_kyc.up.sql             # KYC verifications table
│   └── 002_kyc.down.sql
│
├── test/integration/              # Integration tests
├── docker-compose.yml             # Dev infrastructure (PG + Redis + Kafka + ClickHouse + Anvil)
├── Makefile                       # Build, test, deploy commands
└── go.mod                         # Go module definition
```

---

## Technology Stack

| Category | Technology | Version |
|----------|-----------|---------|
| Language | Go | 1.25 |
| HTTP Router | go-chi/chi | v5.0.12 |
| Database Driver | pgx | v5.5.5 |
| Cache | go-redis | v8.11.5 |
| Message Queue | segmentio/kafka-go | v0.4.51 |
| gRPC | google.golang.org/grpc | v1.79.1 |
| JWT | golang-jwt | v5.2.1 |
| Blockchain | go-ethereum | v1.17.4 |
| ECC | btcec/secp256k1 | v2.5.0 |
| Database Migration | golang-migrate | v4.19.1 |
| OLAP | clickhouse-go | v2.47.0 |
| FIX Protocol | quickfixgo | v0.9.10 |
| Logging | zerolog | v1.32.0 |
| ID Generation | ulid | v2.1.0 |
| Password Hashing | x/crypto (bcrypt) | v0.53.0 |

---

## Roadmap

### ✅ Phase 1 — Core MVP
Matching engine · Order/Settlement/User/Wallet services · API Gateway · JWT auth · Memory/Redis event bus · 27 unit tests · 4 integration tests

### ✅ Phase 2 — Production Hardening
gRPC microservices · Kafka event bus · Blockchain monitor · Risk management (circuit breaker, limits, blacklist) · KYC/AML · Health checks + Prometheus metrics · Security headers + CORS · Database migration framework

### ✅ Phase 3 — Scale
ClickHouse OLAP pipeline · Multi-chain support (ETH/BSC/ARB + ERC20) · Margin/perpetual trading · FIX 4.4 protocol · Cold wallet M-of-N multisig

---

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/amazing-feature`)
3. Run tests (`make test`)
4. Commit changes (`git commit -m 'feat: amazing feature'`)
5. Push (`git push origin feat/amazing-feature`)
6. Open a Pull Request

### Commit Convention

```
feat:     New feature
fix:      Bug fix
docs:     Documentation
test:     Tests
refactor: Code restructuring
perf:     Performance improvement
```

---

## License

MIT License

---

<div align="center">
  <sub>Built with ❤️ in Go</sub>
</div>
