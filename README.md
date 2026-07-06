# 加密货币交易所 / Cryptocurrency Exchange

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Tests](https://img.shields.io/badge/tests-31%2F31%20PASS-brightgreen)]()
[![Race](https://img.shields.io/badge/race%20detector-clean-brightgreen)]()
[![Phase](https://img.shields.io/badge/phase-production%20ready-blue)]()

**Go 语言构建的生产级中心化加密货币交易所 / Production-grade centralized crypto exchange in Go**

</div>

---

## 目录 / Table of Contents

[1. 概述 / Overview](#1-概述--overview) ·
[2. 架构 / Architecture](#2-架构--architecture) ·
[3. 快速开始 / Quick Start](#3-快速开始--quick-start) ·
[4. API 参考 / API Reference](#4-api-参考--api-reference) ·
[5. 配置 / Configuration](#5-配置--configuration) ·
[6. 部署 / Deployment](#6-部署--deployment) ·
[7. 测试 / Testing](#7-测试--testing) ·
[8. 监控 / Monitoring](#8-监控--monitoring) ·
[9. 安全 / Security](#9-安全--security) ·
[10. 项目结构 / Project Structure](#10-项目结构--project-structure) ·
[11. 技术栈 / Tech Stack](#11-技术栈--tech-stack) ·
[12. 路线图 / Roadmap](#12-路线图--roadmap) ·
[13. 贡献 / Contributing](#13-贡献--contributing) ·
[14. 许可证 / License](#14-许可证--license)

---

## 1. 概述 / Overview

一个高性能中心化加密货币交易所。包含内存撮合引擎、gRPC 微服务、Kafka 事件总线、多链区块链集成、风控、KYC/AML、杠杆交易、FIX 协议和冷钱包多签。

> A high-performance centralized exchange featuring in-memory matching engine, gRPC microservices, Kafka event streaming, multi-chain blockchain integration, risk management, KYC/AML, margin trading, FIX protocol, and cold wallet multisig.

### 关键指标 / Key Metrics

| 指标 Metric | 值 Value |
|-------------|----------|
| 撮合吞吐 Matching Throughput | 1,208,160 ops/s |
| 撮合延迟 Matching Latency | 843 ns/op |
| 订单类型 Order Types | 市价/限价/止损/止盈 Market/Limit/Stop-Loss/Stop-Limit |
| 有效期 Time in Force | GTC / IOC / FOK |
| 最大杠杆 Max Leverage | 125x |
| 支持链 Supported Chains | Ethereum / BSC / Arbitrum |
| 服务数 Services | 6 个 (Gateway/Matching/Settlement/Wallet/User/Blockchain) |

---

## 2. 架构 / Architecture

### 系统架构图 / System Architecture

```
                              ┌──────────────────────────────────────────┐
                              │              负载均衡 / Load Balancer      │
                              └────────────────────┬─────────────────────┘
                                                   │
                              ┌────────────────────┴─────────────────────┐
                              │          API 网关 / Gateway (:8080)       │
                              │   chi Router | JWT RS256 | 限流/RateLimit │
                              │   /health  /ready  /metrics  /api/v1/*    │
                              └──┬──────────┬──────────┬──────────────────┘
                                 │ REST     │ gRPC     │
          ┌──────────────────────┤          │          ├──────────────────┐
          ▼                      ▼          ▼          ▼                  ▼
   ┌────────────┐   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐
   │  用户服务   │   │  订单服务   │  │  钱包服务   │  │  行情服务   │  │  FIX     │
   │  User Svc  │   │  Order Svc │  │ Wallet Svc │  │ Market Svc │  │ Gateway  │
   │  :50051    │   │  :50051    │  │  :50051    │  │            │  │  :9880   │
   └─────┬──────┘   └─────┬──────┘  └──────┬──────┘  └─────┬──────┘  └────┬─────┘
         │                │                │                │              │
         └────────────────┼────────────────┼────────────────┼──────────────┘
                          │                │                │
                          │    ┌───────────┴───────────┐    │
                          │    │   Kafka / Redis Bus   │    │
                          │    └───────────┬───────────┘    │
                          ▼                ▼                ▼
                   ┌────────────┐  ┌────────────┐  ┌────────────────┐
                   │  撮合引擎   │  │  结算服务   │  │  区块链监控    │
                   │  Matching  │  │ Settlement │  │  Blockchain    │
                   └─────┬──────┘  └─────┬──────┘  └───────┬────────┘
                         │               │                  │
                         ▼               ▼                  ▼
                   ┌─────────────────────────────────────────────┐
                   │ PostgreSQL │ Redis │ Kafka │ ClickHouse │ ETH│
                   └─────────────────────────────────────────────┘
```

### 数据流 / Data Flow

```
下单 / Order:
  客户端 ──REST──▶ API 网关 ──▶ 订单服务 ──▶ 撮合引擎
      Client ──REST──▶ Gateway ──▶ Order Svc ──▶ Matching Engine
                                                     │
                    ┌────────────────────────────────┤
                    ▼                                ▼
              成交事件 ◀─── Kafka ──── 撮合完成
         Trade Executed ◀─── Kafka ──── Matched
                    │
                    ▼
            结算服务 ──▶ PostgreSQL (余额更新 / balance)
         Settlement ──▶ PostgreSQL (update)
                    │
                    ├──▶ ClickHouse (成交记录 / trade log)
                    ├──▶ Redis (行情缓存 / market cache)
                    └──▶ Kafka (通知事件 / notification)

充值 / Deposit:
  区块链 ──▶ 区块监控 ──▶ Kafka ──▶ 钱包服务 ──▶ PostgreSQL
  Chain ──▶ Block Monitor ──▶ Kafka ──▶ Wallet Svc ──▶ PostgreSQL
                                                         │
                                                   审计日志 / audit log
```

---

## 3. 快速开始 / Quick Start

### 环境要求 / Prerequisites

| 依赖 | 版本 | 用途 / Purpose |
|------|------|----------------|
| Go | 1.25+ | 编译 / Compilation |
| Docker | 24+ | 基础设施 / Infrastructure |
| Make | 4+ | 构建自动化 / Build |

### 5 分钟本地启动 / 5-Minute Setup

```bash
# 1. 克隆 / Clone
git clone https://github.com/1952154539/Cryptocurrency-exchange.git
cd Cryptocurrency-exchange

# 2. 启动基础设施 (PG + Redis + Kafka + ClickHouse + Anvil)
docker compose up -d

# 3. 建表 / Init DB
make migrate

# 4. 编译 / Build
make build

# 5. 启动 6 个服务 / Start all services
export KAFKA_BROKERS=localhost:9092
./bin/matching-engine &
./bin/settlement-service &
./bin/wallet-service &
./bin/user-service &
./bin/api-gateway &
./bin/blockchain-monitor &

# 6. 验证 / Verify
curl http://localhost:8080/health
# → {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}
curl http://localhost:8080/api/v1/ping
# → {"status":"ok"}
```

---

## 4. API 参考 / API Reference

### 公开接口 / Public (No Auth)

| 方法 | 端点 / Endpoint | 说明 / Description |
|------|-----------------|--------------------|
| `GET` | `/health` | 健康检查 (DB/Redis/Kafka) / Service health |
| `GET` | `/ready` | K8s 就绪探针 / Readiness probe |
| `GET` | `/metrics` | Prometheus 指标 / Metrics |
| `GET` | `/api/v1/ping` | 心跳 / Heartbeat |
| `GET` | `/api/v1/time` | 服务器时间 (ms) / Server time |
| `GET` | `/api/v1/depth?symbol=ETH-USDT` | 订单簿深度 / Depth |
| `GET` | `/api/v1/trades?symbol=ETH-USDT` | 最近成交 / Recent trades |
| `GET` | `/api/v1/klines?symbol=ETH-USDT&interval=1h` | K线 / Candles |
| `GET` | `/api/v1/ticker/24hr?symbol=ETH-USDT` | 24h 行情 / 24h ticker |

### 私有接口 / Private (JWT Bearer)

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/account` | 账户信息 / Account |
| `POST` | `/api/v1/order` | 下单 / Place order |
| `DELETE` | `/api/v1/order` | 撤单 / Cancel order |
| `GET` | `/api/v1/order?orderId=<id>` | 查订单 / Query order |
| `GET` | `/api/v1/open-orders` | 当前挂单 / Open orders |
| `GET` | `/api/v1/wallet/balances` | 钱包余额 / Balances |
| `POST` | `/api/v1/wallet/deposit-address` | 充值地址 / Deposit address |
| `POST` | `/api/v1/wallet/withdraw` | 提现 / Withdraw |

### 下单示例 / Place Order

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

响应 / Response:
```json
{"orderId":"ord_4H7XK2M9P1X","clientOrderId":"","status":"open","filledQty":"0"}
```

### gRPC 服务 / gRPC Services

| 服务 Service | 方法 Methods |
|-------------|-------------|
| `UserService` | Register, Login, GetUser, UpdateKYC |
| `WalletService` | GetDepositAddress, RequestWithdrawal, GetBalances |
| `OrderService` | PlaceOrder, CancelOrder, GetOrder, GetOpenOrders |
| `MarketDataService` | GetDepth, GetTrades, GetTicker, GetKlines |

### 错误码 / Error Codes

| HTTP | 含义 Meaning |
|------|-------------|
| 200 | 成功 Success |
| 400 | 无效请求 Invalid request |
| 401 | 未认证 Unauthorized |
| 403 | 无权访问 Forbidden |
| 404 | 未找到 Not found |
| 429 | 限流 Rate limited |
| 500 | 服务器错误 Server error |
| 503 | 服务降级 Degraded (health) |

---

## 5. 配置 / Configuration

### 生产必填 / Required (Production)

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export WALLET_MASTER_SEED_HEX=<64位16进制种子 / hex seed>
export KAFKA_BROKERS=kafka-0:9092,kafka-1:9092,kafka-2:9092
```

### 可选配置 / Optional (with defaults)

| 变量 Variable | 默认值 Default | 说明 |
|---------------|---------------|------|
| `ENV` | `development` | `production` 启用严格校验 |
| `PG_HOST` | `localhost` | PostgreSQL 地址 |
| `PG_PORT` | `5432` | PostgreSQL 端口 |
| `PG_USER` | `exchange` | PostgreSQL 用户 |
| `PG_PASSWORD` | `exchange_dev` | PostgreSQL 密码 |
| `PG_DATABASE` | `exchange` | PostgreSQL 库名 |
| `PG_MAX_CONNS` | `50` | 连接池大小 |
| `REDIS_HOST` | `localhost` | Redis 地址 |
| `REDIS_PORT` | `6379` | Redis 端口 |
| `REDIS_DB` | `0` | Redis 库号 |
| `KAFKA_GROUP_ID` | `exchange` | 消费者组 ID |
| `HTTP_PORT` | `8080` | API 网关端口 |
| `GRPC_PORT` | `50051` | gRPC 端口 |
| `ETH_RPC_URL` | `http://localhost:8545` | 以太坊 RPC |

---

## 6. 部署 / Deployment

### Docker Compose (开发 / Dev)

```bash
docker compose up -d       # 启动全部基础设施
docker compose down        # 停止全部
```

### 服务启动顺序 / Startup Order

1. **基础设施 Infrastructure**: PostgreSQL → Redis → Kafka → ClickHouse
2. **数据库迁移 Migration**: `make migrate`
3. **撮合引擎 Matching Engine**: 核心撮合
4. **结算服务 Settlement**: 成交结算 (Kafka 消费者)
5. **钱包服务 Wallet**: 充值提现处理
6. **用户服务 User**: 认证
7. **API 网关 Gateway**: 对外入口
8. **区块链监控 Blockchain Monitor**: 链上扫描

### 健康检查 / Health Check

```bash
curl http://localhost:8080/health
# → {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}

curl http://localhost:8080/ready    # K8s 就绪探针
curl http://localhost:8080/metrics  # Prometheus 指标
```

---

## 7. 测试 / Testing

### 命令 / Commands

```bash
make test             # 全部 + 竞态 / All + race
make test-matching    # 撮合引擎 / Matching only
make test-integration # 集成测试 / Integration
make bench            # 性能基准 / Benchmark
make vet              # 静态分析 / Static analysis
```

### 生产级测试报告 / Production Test Report

| 测试项 Test | 结果 Result |
|-------------|-------------|
| `go build ./cmd/...` (6 binaries) | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/... -race` (27 tests) | ✅ PASS |
| `go test ./test/... -race` (4 integration) | ✅ PASS |
| `go test -bench=.` | ✅ 843 ns/op, 19 allocs/op |
| 20 并发订单压力测试 / Concurrent stress | ✅ 无竞态 / No race |

### 测试套件 / Test Suites

| 套件 Suite | 数量 | 覆盖 Coverage |
|------------|------|---------------|
| `decimal` | 9 | 定点数运算/解析/精度/四舍五入 |
| `matching` | 18 | 订单簿/FIFO/GTC/IOC/FOK/部分成交/撤单/快照 |
| `integration` | 4 | 端到端/市价单/FOK/20并发 |

### 项目统计 / Project Stats

| 指标 Metric | 值 Value |
|-------------|----------|
| Go 源文件 Source files | 75 |
| 代码行数 Lines of code | 8,690 |
| Package 数 Packages | 35 |
| 依赖 Dependencies | 55 |
| 二进制大小 Binary size | 11M - 30M |

---

## 8. 监控 / Monitoring

### Prometheus 指标 / Metrics

```
http_requests_total       HTTP 请求总数 / Total requests
orders_matched_total      撮合订单数 / Matched orders
orders_rejected_total     拒绝订单数 / Rejected orders
trades_settled_total      成交结算数 / Settled trades
settlement_errors_total   结算错误数 / Settlement errors
deposits_confirmed_total  充值确认数 / Confirmed deposits
withdrawals_requested_total 提现请求数 / Withdrawal requests
```

### 日志格式 / Log Format

所有服务使用结构化 JSON 日志 (zerolog)。关键字段: `level`, `time`, `message`, `user_id`, `order_id`, `symbol`, `error`.

```json
{"level":"info","time":"2026-07-06T11:52:23Z","user_id":"abc123",
 "order_id":"ord_4H7X","symbol":"ETH-USDT","message":"order placed"}
```

---

## 9. 安全 / Security

| 层面 Layer | 方案 Implementation |
|-----------|-------------------|
| **认证** | JWT RS256 (15min) + HMAC HS256 降级 / fallback |
| **API 签名** | HMAC-SHA256 (5s 窗口/window)，已实现未默认启用 |
| **密码** | bcrypt (cost=12) |
| **限流** | 令牌桶: IP 100r/s, 用户 50r/s, 下单 20r/s |
| **HTTP 头** | CSP + HSTS + X-Frame-Options + X-Content-Type-Options |
| **请求体** | 1MB MaxBytesReader |
| **SQL 注入** | 100% 参数化查询 / parameterized (pgx) |
| **防双花** | `UNIQUE(tx_hash, to_address)` + 结算幂等 / idempotency |
| **乐观锁** | accounts 表 `version` 列防并发覆盖 |
| **钱包** | BIP44/secp256k1 + 用户隔离地址 + 事务提现 |
| **密钥** | 环境变量，无硬编码，生产强校验 |
| **关机** | 30s 优雅关闭 / graceful shutdown |

---

## 10. 项目结构 / Project Structure

```
├── api/proto/                     # gRPC 定义 + 生成代码
├── cmd/                           # 6 个服务入口
│   ├── api-gateway/               # HTTP :8080
│   ├── matching-engine/           # 撮合引擎
│   ├── settlement-service/        # 结算服务
│   ├── user-service/              # 用户服务
│   ├── wallet-service/            # 钱包服务
│   └── blockchain-monitor/        # 区块链监控
├── internal/
│   ├── matching/                  # 撮合引擎 (分片 + RWMutex)
│   ├── order/                     # 订单 (校验/冻结/状态机/持久化)
│   ├── settlement/                # 结算 (乐观锁/幂等/冻结同步)
│   ├── user/                      # 用户 (JWT RS256/HS256 + bcrypt)
│   ├── wallet/                    # 钱包 (BIP44/secp256k1)
│   │   └── cold/multisig.go       # 冷钱包 M-of-N 多签
│   ├── trading/                   # 高级交易
│   │   ├── margin/engine.go       # 杠杆 (强平 + 资金费率)
│   │   └── fix/session.go         # FIX 4.4 协议
│   ├── blockchain/                # 区块链 (ETH 扫描/提现/多链)
│   ├── events/                    # 事件总线 (Kafka + Redis + Memory)
│   ├── gateway/                   # API 网关 (chi + 中间件)
│   ├── grpc/                      # gRPC Server + Client
│   ├── risk/                      # 风控 (熔断/限额/黑名单)
│   ├── kyc/                       # KYC/AML
│   ├── marketdata/                # 行情数据
│   ├── db/                        # Postgres/Redis/ClickHouse + 迁移
│   ├── telemetry/                 # 日志 + Prometheus + 健康检查
│   ├── common/                    # 公共 (定点数/类型/错误/ID)
│   └── config/                    # 配置管理
├── migrations/                    # 迁移 SQL
├── test/integration/              # 集成测试
├── docker-compose.yml
├── Makefile
└── go.mod
```

---

## 11. 技术栈 / Tech Stack

| 类别 Category | 技术 Technology | 版本 Version |
|---------------|---------------|--------------|
| 语言 Language | Go | 1.25 |
| HTTP | go-chi/chi | v5 |
| 数据库 DB | pgx (PostgreSQL 16) | v5 |
| 缓存 Cache | go-redis | v8 |
| 消息 Message | segmentio/kafka-go | v0.4 |
| gRPC | google.golang.org/grpc | v1.79 |
| 认证 Auth | golang-jwt | v5 |
| 区块链 Chain | go-ethereum + btcec/secp256k1 | v1.17 / v2.5 |
| 迁移 Migration | golang-migrate | v4 |
| OLAP | clickhouse-go | v2.47 |
| FIX | quickfixgo | v0.9 |
| 日志 Log | zerolog | v1.32 |
| ID | ulid | v2 |

---

## 12. 路线图 / Roadmap

### ✅ Phase 1 — 核心 MVP / Core
撮合引擎 · 订单/结算/用户/钱包 · API 网关 · JWT · 事件总线 · 31 测试

### ✅ Phase 2 — 生产加固 / Production
gRPC · Kafka · 区块链 · 风控 · KYC/AML · 健康检查 + Prometheus · 安全加固

### ✅ Phase 3 — 规模化 / Scale
ClickHouse · 多链 · 杠杆/合约 · FIX 4.4 · 冷钱包多签

---

## 13. 贡献 / Contributing

1. Fork 仓库
2. 创建分支 `git checkout -b feat/功能名`
3. 运行测试 `make test`
4. 提交 `git commit -m 'feat: 功能描述'`
5. 推送 `git push origin feat/功能名`
6. 发起 Pull Request

### 提交规范 / Commit Convention

```
feat:     新功能 / New feature
fix:      修复 / Bug fix
docs:     文档 / Documentation
test:     测试 / Tests
refactor: 重构 / Refactor
perf:     性能 / Performance
```

---

## 14. 许可证 / License

MIT License

---

<div align="center">
  <sub>Built with Go • 75 files • 8,690 LOC • 35 packages</sub>
</div>
