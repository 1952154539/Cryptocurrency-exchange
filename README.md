# 加密货币交易所

基于 Go 语言构建的中心化加密货币交易所，采用链下撮合引擎 + 链上结算的混合架构。支持 gRPC 微服务通信、Kafka 异步事件总线、Ethereum 区块链集成、风控系统和 KYC/AML。

---

## 1. 系统架构

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
        │  :50051  │  │  :50053  │ │          │ │   :50052     │
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
                     │  PostgreSQL | Redis | Kafka | ClickHouse │
                     └──────────────────────────────────────────┘
```

---

## 2. 项目结构

```
├── api/proto/                     # Protobuf 服务定义
│   ├── order.proto                # 订单服务
│   ├── wallet.proto               # 钱包服务
│   ├── user.proto                 # 用户服务
│   ├── market.proto               # 行情服务
│   └── gen/                       # protoc 生成代码 (.pb.go + _grpc.pb.go)
│
├── cmd/                           # 服务启动入口
│   ├── api-gateway/               # API 网关 (:8080)
│   ├── matching-engine/           # 撮合引擎
│   ├── user-service/              # 用户服务 (:50051 gRPC)
│   ├── wallet-service/            # 钱包服务 (:50052 gRPC)
│   ├── settlement-service/        # 结算服务
│   └── blockchain-monitor/        # 区块链监控
│
├── internal/
│   ├── matching/                  # 撮合引擎 (分片 + 读写锁)
│   ├── order/                     # 订单服务 (校验/状态机/冻结/持久化)
│   ├── user/                      # 用户服务 (JWT RS256/HS256 + bcrypt)
│   ├── wallet/                    # 钱包服务 (BIP44/secp256k1 + 提现事务)
│   ├── settlement/                # 结算服务 (乐观锁 + 幂等 + 冻结同步)
│   ├── marketdata/                # 行情数据 (Redis 存储 + 事件订阅)
│   ├── gateway/                   # API 网关 (chi 路由 + 中间件)
│   ├── grpc/                      # gRPC 服务端实现 + 客户端封装
│   ├── blockchain/ethereum/       # 区块链集成 (扫描/提现/客户端)
│   ├── risk/                      # 风控 (熔断/限额/黑名单)
│   ├── kyc/                       # KYC/AML (审核流程/制裁筛查)
│   ├── events/                    # 事件总线 (Kafka + Redis + Memory)
│   ├── common/decimal/            # 18位定点数 (金融精度)
│   ├── db/                        # Postgres/Redis 连接 + 迁移
│   ├── config/                    # 环境变量配置
│   └── telemetry/                 # 日志 + Prometheus 指标 + 健康检查
│
├── migrations/                    # 数据库迁移 (up/down)
├── test/integration/              # 集成测试
├── docker-compose.yml             # 本地开发环境
├── Makefile                       # 构建/测试/部署
└── go.mod
```

---

## 3. 快速开始

### 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.25+ (CI: 1.23) | 编译运行 |
| Docker | 24+ | 基础设施 |
| PostgreSQL | 16 | 业务数据 |
| Redis | 7 | 缓存/事件总线 |
| Kafka | 3.5+ | 生产事件总线 |

### 本地启动

```bash
# 1. 启动基础设施
docker compose up -d

# 2. 数据库迁移
make migrate

# 3. 构建
make build

# 4. 启动服务（按顺序）
export KAFKA_BROKERS=localhost:9092
./bin/matching-engine &
./bin/settlement-service &
./bin/wallet-service &
./bin/user-service &
./bin/api-gateway &
./bin/blockchain-monitor &

# 5. 验证
curl http://localhost:8080/health     # {"status":"ok",...}
curl http://localhost:8080/ready      # {"status":"ready"}
curl http://localhost:8080/metrics    # Prometheus metrics
curl http://localhost:8080/api/v1/ping # {"status":"ok"}
```

### 生产环境变量

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export WALLET_MASTER_SEED_HEX=<64-char-hex-seed>
export KAFKA_BROKERS=kafka1:9092,kafka2:9092,kafka3:9092
export PG_HOST=postgres.internal
export REDIS_HOST=redis.internal
export HTTP_PORT=8080
export GRPC_PORT=50051
# 所有环境变量见 internal/config/config.go
```

---

## 4. API 接口

### 公开接口

```
GET  /health                                       服务健康检查 (DB/Redis/Kafka)
GET  /ready                                        就绪探针
GET  /metrics                                      Prometheus 指标
GET  /api/v1/ping                                  服务心跳
GET  /api/v1/time                                  服务器时间
GET  /api/v1/depth?symbol=ETH-USDT&limit=100        订单簿深度
GET  /api/v1/trades?symbol=ETH-USDT&limit=500       最近成交
GET  /api/v1/klines?symbol=ETH-USDT&interval=1h     K线数据
GET  /api/v1/ticker/24hr?symbol=ETH-USDT            24h行情
```

### 私有接口 (JWT Bearer Token)

```
GET    /api/v1/account                              账户信息
POST   /api/v1/order                                下单
DELETE /api/v1/order                                撤单
GET    /api/v1/order?orderId=xxx                     查询订单
GET    /api/v1/open-orders?symbol=ETH-USDT           当前挂单
GET    /api/v1/wallet/balances                       钱包余额
POST   /api/v1/wallet/deposit-address                获取充值地址
POST   /api/v1/wallet/withdraw                       提现
```

### gRPC 服务

```
OrderService            PlaceOrder / CancelOrder / GetOrder / GetOpenOrders
WalletService   :50051   GetDepositAddress / RequestWithdrawal / GetBalances
UserService     :50051   Register / Login / GetUser / UpdateKYC
MarketDataService        GetDepth / GetTrades / GetTicker / GetKlines
```

---

## 5. 撮合引擎

| 特性 | 说明 |
|------|------|
| 数据结构 | 内存订单簿，买盘降序/卖盘升序，同价位 FIFO 链表 |
| 撮合算法 | 价格-时间优先 (Price-Time Priority)，Maker 价格成交 |
| 并发模型 | 每交易对独立 goroutine 分片，RWMutex 读写安全 |
| 内存管理 | sync.Pool 复用 OrderNode |
| 性能 | ~820 ns/op, ~1,470,000 ops/s (i5-11300H) |
| 订单类型 | Market / Limit (Stop-Loss/Stop-Limit defined, engine pending) |
| 有效期 | GTC / IOC / FOK |

---

## 6. 风控系统

| 模块 | 功能 |
|------|------|
| 熔断器 | 价格波动超阈值自动暂停交易，可配置阈值和恢复时间 |
| 提现限额 | 单笔上限、日累计上限、最小提现额 |
| 黑名单 | IP / 用户ID / 钱包地址 三级黑名单 |

---

## 7. 安全体系

| 层面 | 方案 |
|------|------|
| Web 认证 | JWT RS256 (15min) + HMAC HS256 降级 |
| API 认证 | HMAC-SHA256 签名（已实现，未默认启用），5秒窗口防重放 |
| 限流 | 令牌桶：IP 100r/s，用户 50r/s |
| 安全头 | CSP / HSTS / X-Frame-Options / X-Content-Type-Options |
| SQL 注入 | 100% 参数化查询 (pgx) |
| 密码 | bcrypt (cost=12) |
| 防双花 | UNIQUE (tx_hash, to_address) + 结算幂等 |
| 钱包 | BIP44/secp256k1，用户隔离地址，提现事务 |
| 请求体 | 1MB MaxBytesReader 限制 (order handler) |

---

## 8. 数据库

| 表 | 说明 |
|----|------|
| users | 用户账户 (bcrypt密码, KYC等级, 2FA) |
| accounts | 余额 (version乐观锁, frozen_balance) |
| orders | 订单记录 |
| trades | 成交记录 (trade_id幂等) |
| deposits / deposit_addresses | 充值 |
| withdrawals | 提现 (冷热钱包分类) |
| balance_transactions | 审计追踪 |
| kyc_verifications | KYC审核 |
| fee_tiers | 手续费阶梯 (5级) |

---

## 9. 测试

```bash
make test            # 全部测试 + 竞态检测
make test-matching   # 撮合引擎
make bench           # 性能基准 (~1.47M ops/s)
```

| Suite | 状态 |
|-------|------|
| decimal | 9/9 PASS |
| matching | 18/18 PASS |
| integration | PASS (竞态检测通过) |

---

## 10. 运维

```bash
make build           # 构建 6 个二进制
make test            # 测试
make lint            # golangci-lint
make vet             # go vet
make docker-up       # 启动基础设施
make docker-down     # 停止
make migrate         # 数据库迁移
make clean           # 清理
```

---

## 11. 技术栈

| 组件 | 选型 |
|------|------|
| 语言 | Go 1.25 |
| HTTP | go-chi/chi v5 |
| 数据库 | pgx v5 (PostgreSQL 16) |
| 缓存 | go-redis v8 |
| 消息队列 | segmentio/kafka-go (生产) / Redis PubSub (降级) |
| gRPC | google.golang.org/grpc |
| 认证 | golang-jwt v5 + RSA |
| 钱包 | btcec/secp256k1 + BIP32/44 |
| 区块链 | go-ethereum |
| 迁移 | golang-migrate v4 (Makefile 使用 psql 直连) |
| 日志 | rs/zerolog |
| ID | oklog/ulid v2 |

---

## 12. 路线图

### ✅ Phase 1 — 核心 MVP
- 撮合引擎 (分片 + 锁安全)
- 订单/结算/用户/钱包 服务
- API 网关 (JWT + 限流 + 安全头)
- 内存事件总线 + Redis 事件总线
- 单元测试 + 集成测试 + 性能基准

### ✅ Phase 2 — 生产加固
- gRPC 微服务通信 (4个 proto 服务)
- Kafka 事件总线 (consumer group + ACK)
- 风控系统 (熔断/限额/黑名单)
- KYC/AML (审核流程/制裁筛查)
- 区块链监控 (ETH 扫描/提现广播)
- 健康检查 + Prometheus 指标
- 数据库迁移框架 (golang-migrate)
- 安全加固 (安全头/CORS/TLS准备)

### 🔲 Phase 3 — 规模化 (规划中)
- 杠杆/合约交易
- 多链支持 (Arbitrum/BSC/Optimism)
- FIX 协议
- 冷钱包 HSM 多签
- ClickHouse OLAP 分析
- 多区域部署
- SOC 2 审计

---

## 13. 许可证

MIT License
