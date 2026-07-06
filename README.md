# 加密货币交易所

基于 Go 语言构建的中心化加密货币交易所，涵盖撮合引擎、gRPC 微服务、Kafka 事件总线、区块链集成、风控、KYC/AML、杠杆交易、FIX 协议和冷钱包多签。

---

## 系统架构

```
                        ┌─────────────────────────────────────┐
                        │      API Gateway (chi) :8080         │
                        │  JWT/RS256 | RateLimit | Security    │
                        │  /health  /ready  /metrics           │
                        └──────┬──────┬──────┬─────────────────┘
                               │REST  │ gRPC │
               ┌───────────────┤      │      ├───────────────┐
               ▼               ▼      ▼      ▼               ▼
        ┌──────────┐  ┌──────────┐ ┌──────────┐ ┌──────────────┐
        │  User    │  │  Order   │ │ Market   │ │   Wallet     │
        │  Service │  │  Service │ │  Data    │ │   Service    │
        └────┬─────┘  └────┬─────┘ └────┬─────┘ └──────┬───────┘
             │              │            │              │
             └──────────────┼────────────┼──────────────┘
                            │  Kafka / Redis Event Bus
                            ▼            ▼
                     ┌──────────┐ ┌──────────┐ ┌──────────────┐
                     │ Matching │ │Settlement│ │  Blockchain  │
                     │  Engine  │ │ Service  │ │   Monitor    │
                     └──────────┘ └──────────┘ └──────────────┘
                            │            │              │
                            ▼            ▼              ▼
                     ┌──────────────────────────────────────────┐
                     │ PostgreSQL | Redis | Kafka | ClickHouse   │
                     └──────────────────────────────────────────┘
```

---

## 项目结构

```
├── api/proto/                     # Protobuf 服务定义 + 生成代码
│   ├── order/wallet/user/market.proto
│   └── gen/                       # protoc 生成 (*.pb.go + *_grpc.pb.go)
├── cmd/                           # 6 个服务启动入口
│   ├── api-gateway/               # REST :8080 + /health /ready /metrics
│   ├── matching-engine/           # 撮合引擎 (Kafka 生产者)
│   ├── user-service/              # 用户服务 (gRPC)
│   ├── wallet-service/            # 钱包服务 (gRPC)
│   ├── settlement-service/        # 结算服务 (Kafka 消费者)
│   └── blockchain-monitor/        # 区块链扫描 (ETH 充值检测)
├── internal/
│   ├── matching/                  # 撮合引擎 (分片 + RWMutex)
│   ├── order/                     # 订单 (校验/冻结/状态机/持久化)
│   ├── user/                      # 用户 (JWT RS256/HS256 + bcrypt)
│   ├── wallet/                    # 钱包 (BIP44/secp256k1 + 提现事务)
│   ├── wallet/cold/               # 冷钱包 M-of-N 多签
│   ├── settlement/                # 结算 (乐观锁 + 幂等 + 冻结同步)
│   ├── marketdata/                # 行情 (Redis + 事件订阅)
│   ├── gateway/                   # API 网关 (chi + 中间件)
│   ├── grpc/                      # gRPC Server 实现 + Client 封装
│   ├── blockchain/
│   │   ├── ethereum/              # ETH 客户端/扫描/提现
│   │   └── adapter/               # 多链抽象 (ETH/BSC/ARB + ERC20)
│   ├── trading/
│   │   ├── margin/                # 杠杆 (强平引擎 + 资金费率)
│   │   └── fix/                   # FIX 4.4 (NewOrderSingle/ExecutionReport)
│   ├── risk/                      # 风控 (熔断/限额/黑名单)
│   ├── kyc/                       # KYC/AML (审核/制裁筛查)
│   ├── events/                    # 事件总线 (Kafka + Redis + Memory)
│   ├── common/decimal/            # 18 位定点数
│   ├── db/                        # Postgres/Redis/ClickHouse + 迁移
│   ├── config/                    # 环境变量配置
│   └── telemetry/                 # 日志 + Prometheus + 健康检查
├── migrations/                    # 数据库迁移 (up/down SQL)
├── test/integration/              # 集成测试
├── docker-compose.yml             # 本地开发环境 (PG+Redis+Kafka+ClickHouse+Anvil)
├── Makefile
└── go.mod
```

---

## 快速开始

```bash
# 1. 启动基础设施
docker compose up -d

# 2. 数据库迁移
make migrate

# 3. 构建
make build

# 4. 启动 (6 个服务)
export KAFKA_BROKERS=localhost:9092
./bin/matching-engine &
./bin/settlement-service &
./bin/wallet-service &
./bin/user-service &
./bin/api-gateway &
./bin/blockchain-monitor &

# 5. 验证
curl http://localhost:8080/health      # 健康检查
curl http://localhost:8080/api/v1/ping # {"status":"ok"}
```

### 生产环境变量

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export JWT_ACCESS_SECRET=<secret>       # HMAC 降级模式
export WALLET_MASTER_SEED_HEX=<64-char-hex>
export KAFKA_BROKERS=kafka1:9092,kafka2:9092,kafka3:9092
export PG_HOST=postgres.internal
export REDIS_HOST=redis.internal
```

---

## API 接口

### 公开

```
GET  /health                     DB/Redis/Kafka 连通性
GET  /ready                      就绪探针
GET  /metrics                    Prometheus 指标
GET  /api/v1/ping                心跳
GET  /api/v1/time                服务器时间
GET  /api/v1/depth?symbol=       订单簿深度
GET  /api/v1/trades?symbol=      最近成交
GET  /api/v1/klines?symbol=      K 线
GET  /api/v1/ticker/24hr?symbol= 24h 行情
```

### 私有 (JWT Bearer)

```
GET    /api/v1/account                   账户
POST   /api/v1/order                     下单
DELETE /api/v1/order                     撤单
GET    /api/v1/order?orderId=            查订单
GET    /api/v1/open-orders?symbol=       挂单
GET    /api/v1/wallet/balances           余额
POST   /api/v1/wallet/deposit-address    充值地址
POST   /api/v1/wallet/withdraw           提现
```

### gRPC 服务

```
WalletService   GetDepositAddress / RequestWithdrawal / GetBalances
UserService     Register / Login / GetUser / UpdateKYC
OrderService    PlaceOrder / CancelOrder / GetOrder / GetOpenOrders
MarketService   GetDepth / GetTrades / GetTicker / GetKlines
```

---

## 撮合引擎

| 特性 | 值 |
|------|-----|
| 数据结构 | 内存订单簿，买盘降序/卖盘升序，FIFO 链表 |
| 算法 | 价格-时间优先，Maker 价格成交 |
| 并发 | 每交易对独立 goroutine 分片 + RWMutex |
| 性能 | ~820 ns/op, ~1,470,000 ops/s |
| 订单类型 | Market / Limit (Stop-Loss/Stop-Limit 已定义) |
| 有效期 | GTC / IOC / FOK |

---

## 模块矩阵

| 模块 | 功能 | 文件 |
|------|------|------|
| 撮合引擎 | 分片内存订单簿 | `internal/matching/` |
| 订单服务 | 校验/冻结/状态机 | `internal/order/` |
| 结算服务 | 乐观锁/幂等/冻结同步 | `internal/settlement/` |
| 用户服务 | JWT RS256/HS256 | `internal/user/` |
| 钱包服务 | BIP44/secp256k1 | `internal/wallet/` |
| API 网关 | chi + 中间件 | `internal/gateway/` |
| 行情数据 | Redis + 事件 | `internal/marketdata/` |
| 事件总线 | Kafka + Redis + Memory | `internal/events/` |
| gRPC | Server + Client | `internal/grpc/` |
| 区块链 | ETH 扫描/提现/多链 | `internal/blockchain/` |
| 风控 | 熔断/限额/黑名单 | `internal/risk/` |
| KYC/AML | 审核/制裁筛查 | `internal/kyc/` |
| 杠杆交易 | 强平/资金费率 | `internal/trading/margin/` |
| FIX 协议 | 4.4 NewOrderSingle | `internal/trading/fix/` |
| 冷钱包 | M-of-N 多签 | `internal/wallet/cold/` |
| ClickHouse | OLAP 管道 | `internal/db/clickhouse/` |
| 可观测性 | Prometheus + Health | `internal/telemetry/` |
| 数据库迁移 | golang-migrate | `internal/db/migrate/` |

---

## 安全

| 层面 | 方案 |
|------|------|
| Web 认证 | JWT RS256 (15min) + HMAC HS256 降级 |
| API 认证 | HMAC-SHA256 签名 (5s 窗口)，已实现未默认启用 |
| 限流 | 令牌桶 IP 100r/s + 用户 50r/s |
| HTTP 头 | CSP / HSTS / X-Frame-Options / X-Content-Type-Options |
| 密码 | bcrypt cost=12 |
| SQL | 100% 参数化查询 (pgx) |
| 防双花 | UNIQUE (tx_hash, to_address) + 结算幂等 |
| 请求体 | 1MB MaxBytesReader |

---

## 测试

```bash
make test            # 全部 + race
make test-matching   # 撮合引擎
make bench           # 性能基准
```

### 生产级测试报告 (最新)

| 测试项 | 结果 |
|--------|------|
| `go build ./cmd/...` (6 binaries) | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/... -race` (27 tests) | ✅ PASS |
| `go test ./test/... -race` (4 integration) | ✅ PASS |
| `go test -bench=.` (matching engine) | ✅ 843 ns/op, 1,208,160 ops/s |
| 20 并发订单测试 | ✅ 无竞态 |

### 项目统计

| 指标 | 数值 |
|------|------|
| Go 源文件 | 75 |
| 代码行数 | 8,690 |
| Package 数 | 35 |
| 直接依赖 | 55 |

### 测试套件详情

| Suite | 测试数 | 状态 |
|-------|--------|------|
| decimal (定点数精度) | 9 | ✅ PASS |
| matching (订单簿/撮合/FOK/IOC) | 18 | ✅ PASS |
| integration (端到端/市价单/并发) | 4 | ✅ PASS |

---

## 数据库表

| 表 | 说明 |
|----|------|
| users | 用户 (bcrypt/KYC/2FA) |
| accounts | 余额 (version 乐观锁, frozen_balance) |
| orders | 订单 |
| trades | 成交 (trade_id 幂等) |
| deposits / deposit_addresses | 充值 |
| withdrawals | 提现 (冷热钱包分类) |
| balance_transactions | 审计追踪 |
| kyc_verifications | KYC 审核 |
| fee_tiers | 手续费阶梯 (5 级) |

---

## 技术栈

| 组件 | 版本 |
|------|------|
| Go | 1.25 |
| HTTP | go-chi/chi v5 |
| 数据库 | pgx v5 (PostgreSQL 16) |
| 缓存 | go-redis v8 |
| 消息 | segmentio/kafka-go + Redis PubSub |
| gRPC | google.golang.org/grpc |
| 认证 | golang-jwt v5 |
| 区块链 | go-ethereum + btcec/secp256k1 |
| 迁移 | golang-migrate v4 |
| OLAP | clickhouse-go v2 |
| FIX | quickfixgo |
| 日志 | zerolog |
| ID | ulid v2 |

---

## 路线图

### ✅ Phase 1 — 核心 MVP
撮合引擎 · 订单/结算/用户/钱包 · API 网关 · JWT 认证 · 内存/Redis 事件总线 · 单元测试 + 集成测试

### ✅ Phase 2 — 生产加固
gRPC 微服务 · Kafka 事件总线 · 区块链监控 · 风控 · KYC/AML · 健康检查 + Prometheus · 安全头/CORS · 数据库迁移框架

### ✅ Phase 3 — 规模化
ClickHouse OLAP · 多链支持 (ETH/BSC/ARB) · 杠杆/永续合约 · FIX 4.4 协议 · 冷钱包 M-of-N 多签

---

## 许可证

MIT License
