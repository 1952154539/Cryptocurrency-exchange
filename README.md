# Cryptocurrency Exchange

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Tests](https://img.shields.io/badge/tests-31%2F31%20PASS-brightgreen)]()
[![Race](https://img.shields.io/badge/race%20detector-clean-brightgreen)]()
[![Phase](https://img.shields.io/badge/phase-production%20ready-blue)]()

**Production-grade centralized cryptocurrency exchange written in Go**

[中文文档 / Chinese Documentation](README_CN.md)

</div>

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [Quick Start](#3-quick-start)
4. [API Reference](#4-api-reference)
5. [Configuration](#5-configuration)
6. [Deployment](#6-deployment)
7. [Testing](#7-testing)
8. [Monitoring](#8-monitoring)
9. [Security](#9-security)
10. [Project Structure](#10-project-structure)
11. [Technology Stack](#11-technology-stack)
12. [Roadmap](#12-roadmap)
13. [Contributing](#13-contributing)
14. [License](#14-license)

---

## 1. Overview

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
| Services | 6 (Gateway, Matching, Settlement, Wallet, User, Blockchain Monitor) |

---

## 2. Architecture

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
   │   User     │   │   Order    │  │   Wallet    │  │   Market   │  │   FIX    │
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

## 3. Quick Start

### Prerequisites

| Dependency | Version | Purpose |
|------------|---------|---------|
| Go | 1.25+ | Compilation |
| Docker | 24+ | Infrastructure |
| Make | 4+ | Build automation |

### 5-Minute Setup

```bash
# 1. Clone
git clone https://github.com/1952154539/Cryptocurrency-exchange.git
cd Cryptocurrency-exchange

# 2. Start infrastructure (PostgreSQL + Redis + Kafka + ClickHouse + Anvil)
docker compose up -d

# 3. Initialize database
make migrate

# 4. Build all services
make build

# 5. Start services
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

---

## 4. API Reference

### Public Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Service health (DB, Redis, Kafka) |
| `GET` | `/ready` | Kubernetes readiness probe |
| `GET` | `/metrics` | Prometheus metrics |
| `GET` | `/api/v1/ping` | Heartbeat |
| `GET` | `/api/v1/time` | Server timestamp (ms) |
| `GET` | `/api/v1/depth?symbol=ETH-USDT&limit=100` | Order book depth |
| `GET` | `/api/v1/trades?symbol=ETH-USDT&limit=500` | Recent trades |
| `GET` | `/api/v1/klines?symbol=ETH-USDT&interval=1h` | Candlestick data |
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
| `POST` | `/api/v1/wallet/deposit-address` | Deposit address |
| `POST` | `/api/v1/wallet/withdraw` | Request withdrawal |

### Place Order Example

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
{"orderId":"ord_4H7XK2M9P1X","clientOrderId":"","status":"open","filledQty":"0"}
```

### gRPC Services

| Service | Methods |
|---------|---------|
| `UserService` | Register, Login, GetUser, UpdateKYC |
| `WalletService` | GetDepositAddress, RequestWithdrawal, GetBalances |
| `OrderService` | PlaceOrder, CancelOrder, GetOrder, GetOpenOrders |
| `MarketDataService` | GetDepth, GetTrades, GetTicker, GetKlines |

### Error Codes

| HTTP | Meaning |
|------|---------|
| 200 | Success |
| 400 | Invalid request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not found |
| 429 | Rate limited |
| 500 | Server error |
| 503 | Service degraded |

---

## 5. Configuration

### Required (Production)

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export WALLET_MASTER_SEED_HEX=<64-character-hex-seed>
export KAFKA_BROKERS=kafka-0:9092,kafka-1:9092,kafka-2:9092
```

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `ENV` | `development` | `production` enables strict validation |
| `PG_HOST` | `localhost` | PostgreSQL host |
| `PG_PORT` | `5432` | PostgreSQL port |
| `PG_USER` | `exchange` | PostgreSQL user |
| `PG_PASSWORD` | `exchange_dev` | PostgreSQL password |
| `PG_DATABASE` | `exchange` | PostgreSQL database |
| `PG_MAX_CONNS` | `50` | Connection pool size |
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_DB` | `0` | Redis database |
| `KAFKA_GROUP_ID` | `exchange` | Consumer group ID |
| `HTTP_PORT` | `8080` | API Gateway port |
| `GRPC_PORT` | `50051` | gRPC server port |
| `ETH_RPC_URL` | `http://localhost:8545` | Ethereum RPC endpoint |

---

## 6. Deployment

### Docker Compose (Dev)

```bash
docker compose up -d       # PostgreSQL + Redis + Kafka + ClickHouse + Anvil
docker compose down        # Stop all
```

### Startup Order

1. **Infrastructure**: PostgreSQL, Redis, Kafka, ClickHouse
2. **Migration**: `make migrate`
3. **Matching Engine**: Core order matching
4. **Settlement Service**: Trade settlement (Kafka consumer)
5. **Wallet Service**: Deposit/withdrawal processing
6. **User Service**: Authentication
7. **API Gateway**: External API entry point
8. **Blockchain Monitor**: On-chain deposit scanning

### Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}

curl http://localhost:8080/ready    # K8s readiness probe
curl http://localhost:8080/metrics  # Prometheus metrics
```

---

## 7. Testing

### Commands

```bash
make test             # All tests + race detector
make test-matching    # Matching engine only
make test-integration # Integration tests
make bench            # Performance benchmarks
make vet              # Static analysis
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
| `integration` | 4 | End-to-end flow, market orders, FOK validation, concurrency |

### Project Statistics

| Metric | Value |
|--------|-------|
| Go source files | 75 |
| Lines of code | 8,690 |
| Packages | 35 |
| Direct dependencies | 55 |
| Binary sizes | 11M - 30M |

---

## 8. Monitoring

### Prometheus Metrics

```
http_requests_total       Total HTTP requests
orders_matched_total      Total matched orders
orders_rejected_total     Total rejected orders
trades_settled_total      Total settled trades
settlement_errors_total   Total settlement errors
deposits_confirmed_total  Total confirmed deposits
withdrawals_requested_total Total withdrawal requests
```

### Log Format

All services use structured JSON logging (zerolog). Key fields: `level`, `time`, `message`, `user_id`, `order_id`, `symbol`, `error`.

```json
{"level":"info","time":"2026-07-06T11:52:23Z","user_id":"abc123",
 "order_id":"ord_4H7X","symbol":"ETH-USDT","message":"order placed"}
```

---

## 9. Security

| Layer | Implementation |
|-------|---------------|
| **Authentication** | JWT RS256 (15min) + HMAC HS256 fallback |
| **API Auth** | HMAC-SHA256 signature (5s window), implemented, not enabled by default |
| **Password** | bcrypt (cost=12) |
| **Rate Limiting** | Token bucket: 100 req/s per IP, 50 req/s per user, 20 req/s per order |
| **HTTP Headers** | CSP, HSTS, X-Frame-Options: DENY, X-Content-Type-Options: nosniff |
| **Request Size** | 1 MB MaxBytesReader on all body-reading handlers |
| **SQL Injection** | 100% parameterized queries via pgx |
| **Double-Spend** | `UNIQUE(tx_hash, to_address)` + settlement trade_id idempotency |
| **Optimistic Lock** | `version` column on accounts table |
| **Wallet** | BIP44/secp256k1, per-user account derivation, tx-level withdrawal |
| **Secrets** | Environment variables, production enforcement, no hardcoded values |
| **Shutdown** | Graceful shutdown with 30s timeout on all services |

---

## 10. Project Structure

```
├── api/proto/                     # gRPC definitions + generated code
├── cmd/                           # 6 service entry points
│   ├── api-gateway/               # HTTP :8080
│   ├── matching-engine/           # Order matching
│   ├── settlement-service/        # Post-trade settlement
│   ├── user-service/              # User management + auth
│   ├── wallet-service/            # Wallet + deposits + withdrawals
│   └── blockchain-monitor/        # On-chain scanning
├── internal/
│   ├── matching/                  # Matching engine (sharded + RWMutex)
│   ├── order/                     # Order service (validate/freeze/state/persist)
│   ├── settlement/                # Settlement (optimistic lock/idempotent/frozen sync)
│   ├── user/                      # User (JWT RS256/HS256 + bcrypt)
│   ├── wallet/                    # Wallet (BIP44/secp256k1)
│   │   └── cold/multisig.go       # Cold wallet M-of-N multisig
│   ├── trading/                   # Advanced trading
│   │   ├── margin/engine.go       # Leverage (liquidation + funding rate)
│   │   └── fix/session.go         # FIX 4.4 protocol
│   ├── blockchain/                # ETH client/scanner/withdrawal + multi-chain
│   ├── events/                    # Kafka + Redis + Memory event bus
│   ├── gateway/                   # chi router + handlers + middleware
│   ├── grpc/                      # gRPC servers + client wrappers
│   ├── risk/                      # Circuit breaker + limits + blacklist
│   ├── kyc/                       # KYC workflow + AML screening
│   ├── marketdata/                # Market data (Redis + events)
│   ├── db/                        # Postgres/Redis/ClickHouse + migration runner
│   ├── telemetry/                 # Logging + Prometheus + health checks
│   ├── common/                    # Fixed-point decimal, types, errors, ID gen
│   └── config/                    # Environment-based configuration
├── migrations/                    # SQL migrations (up/down)
├── test/integration/              # Integration tests
├── docker-compose.yml             # Dev infrastructure
├── Makefile
└── go.mod
```

---

## 11. Technology Stack

| Category | Technology | Version |
|----------|-----------|---------|
| Language | Go | 1.25 |
| HTTP Router | go-chi/chi | v5 |
| Database | pgx (PostgreSQL 16) | v5 |
| Cache | go-redis | v8 |
| Message Queue | segmentio/kafka-go | v0.4 |
| gRPC | google.golang.org/grpc | v1.79 |
| Auth | golang-jwt | v5 |
| Blockchain | go-ethereum + btcec/secp256k1 | v1.17 / v2.5 |
| Migration | golang-migrate | v4 |
| OLAP | clickhouse-go | v2.47 |
| FIX | quickfixgo | v0.9 |
| Logging | zerolog | v1.32 |
| ID Generation | ulid | v2 |

---

## 12. Roadmap

### ✅ Phase 1 — Core MVP
Matching engine · Order/Settlement/User/Wallet · API Gateway · JWT auth · Event bus · 31 tests

### ✅ Phase 2 — Production Hardening
gRPC · Kafka · Blockchain monitor · Risk management · KYC/AML · Health checks + Prometheus · Security

### ✅ Phase 3 — Scale
ClickHouse OLAP · Multi-chain (ETH/BSC/ARB) · Margin/perpetual trading · FIX 4.4 · Cold wallet multisig

---

## 13. Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/amazing-feature`)
3. Run tests (`make test`)
4. Commit (`git commit -m 'feat: amazing feature'`)
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

## 14. License

MIT License

---

<div align="center">
  <sub>75 files · 8,690 LOC · 35 packages</sub>
</div>
