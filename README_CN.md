# 加密货币交易所

<div align="center">

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Tests](https://img.shields.io/badge/tests-31%2F31%20PASS-brightgreen)]()
[![Race](https://img.shields.io/badge/race%20detector-clean-brightgreen)]()
[![Phase](https://img.shields.io/badge/phase-production%20ready-blue)]()

**Go 语言构建的生产级中心化加密货币交易所**

[English Documentation](README.md)

</div>

---

## 目录

1. [概述](#1-概述)
2. [架构](#2-架构)
3. [快速开始](#3-快速开始)
4. [API 参考](#4-api-参考)
5. [配置](#5-配置)
6. [部署](#6-部署)
7. [测试](#7-测试)
8. [监控](#8-监控)
9. [安全](#9-安全)
10. [项目结构](#10-项目结构)
11. [技术栈](#11-技术栈)
12. [路线图](#12-路线图)
13. [贡献](#13-贡献)
14. [许可证](#14-许可证)

---

## 1. 概述

高性能中心化加密货币交易所。包含内存撮合引擎、gRPC 微服务、Kafka 事件总线、多链区块链集成、风控系统、KYC/AML 合规、杠杆交易、FIX 协议支持和冷钱包多签。

### 关键指标

| 指标 | 数值 |
|------|------|
| 撮合吞吐 | 1,208,160 ops/s |
| 撮合延迟 | 843 ns/op |
| 订单类型 | 市价/限价/止损/止盈 |
| 有效期 | GTC / IOC / FOK |
| 最大杠杆 | 125x |
| 支持链 | Ethereum / BSC / Arbitrum |
| 服务数 | 6 个 (网关/撮合/结算/钱包/用户/区块链) |

---

## 2. 架构

### 系统架构图

```
                              ┌──────────────────────────────────────────┐
                              │              负载均衡                    │
                              └────────────────────┬─────────────────────┘
                                                   │
                              ┌────────────────────┴─────────────────────┐
                              │          API 网关 (:8080)                │
                              │   chi 路由 | JWT RS256 | 限流            │
                              │   /health  /ready  /metrics  /api/v1/*   │
                              └──┬──────────┬──────────┬─────────────────┘
                                 │ REST     │ gRPC     │
          ┌──────────────────────┤          │          ├──────────────────┐
          ▼                      ▼          ▼          ▼                  ▼
   ┌────────────┐   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐
   │  用户服务   │   │  订单服务   │  │  钱包服务   │  │  行情服务   │  │  FIX     │
   │  :50051    │   │  :50051    │  │  :50051     │  │            │  │  :9880   │
   └─────┬──────┘   └─────┬──────┘  └──────┬──────┘  └─────┬──────┘  └────┬─────┘
         │                │                │                │              │
         └────────────────┼────────────────┼────────────────┼──────────────┘
                          │                │                │
                          │    ┌───────────┴───────────┐    │
                          │    │   Kafka / Redis 总线  │    │
                          │    └───────────┬───────────┘    │
                          ▼                ▼                ▼
                   ┌────────────┐  ┌────────────┐  ┌────────────────┐
                   │  撮合引擎   │  │  结算服务   │  │  区块链监控    │
                   └─────┬──────┘  └─────┬──────┘  └───────┬────────┘
                         │               │                  │
                         ▼               ▼                  ▼
                   ┌─────────────────────────────────────────────┐
                   │ PostgreSQL │ Redis │ Kafka │ ClickHouse │ ETH│
                   └─────────────────────────────────────────────┘
```

### 数据流

```
下单流程：
  客户端 ──REST──▶ API 网关 ──▶ 订单服务 ──▶ 撮合引擎
                                                  │
                    ┌─────────────────────────────┤
                    ▼                             ▼
              成交事件 ◀─── Kafka ──── 撮合完成
                    │
                    ▼
            结算服务 ──▶ PostgreSQL (余额更新)
                    │
                    ├──▶ ClickHouse (成交记录)
                    ├──▶ Redis (行情缓存)
                    └──▶ Kafka (通知事件)

充值流程：
  区块链 ──▶ 区块监控 ──▶ Kafka ──▶ 钱包服务 ──▶ PostgreSQL
                                                     │
                                                审计日志
```

---

## 3. 快速开始

### 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.25+ | 编译 |
| Docker | 24+ | 基础设施 |
| Make | 4+ | 构建自动化 |

### 5 分钟本地启动

```bash
# 1. 克隆
git clone https://github.com/1952154539/Cryptocurrency-exchange.git
cd Cryptocurrency-exchange

# 2. 启动基础设施 (PostgreSQL + Redis + Kafka + ClickHouse + Anvil)
docker compose up -d

# 3. 初始化数据库
make migrate

# 4. 编译所有服务
make build

# 5. 启动服务
export KAFKA_BROKERS=localhost:9092
./bin/matching-engine &
./bin/settlement-service &
./bin/wallet-service &
./bin/user-service &
./bin/api-gateway &
./bin/blockchain-monitor &

# 6. 验证
curl http://localhost:8080/health
# {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}

curl http://localhost:8080/api/v1/ping
# {"status":"ok"}
```

---

## 4. API 参考

### 公开接口 (无需认证)

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 (DB/Redis/Kafka) |
| `GET` | `/ready` | K8s 就绪探针 |
| `GET` | `/metrics` | Prometheus 指标 |
| `GET` | `/api/v1/ping` | 心跳 |
| `GET` | `/api/v1/time` | 服务器时间 (毫秒) |
| `GET` | `/api/v1/depth?symbol=ETH-USDT&limit=100` | 订单簿深度 |
| `GET` | `/api/v1/trades?symbol=ETH-USDT&limit=500` | 最近成交 |
| `GET` | `/api/v1/klines?symbol=ETH-USDT&interval=1h` | K 线数据 |
| `GET` | `/api/v1/ticker/24hr?symbol=ETH-USDT` | 24h 行情 |

### 私有接口 (JWT Bearer Token)

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/v1/account` | 账户信息 |
| `POST` | `/api/v1/order` | 下单 |
| `DELETE` | `/api/v1/order` | 撤单 |
| `GET` | `/api/v1/order?orderId=<id>` | 查订单 |
| `GET` | `/api/v1/open-orders` | 当前挂单 |
| `GET` | `/api/v1/wallet/balances` | 钱包余额 |
| `POST` | `/api/v1/wallet/deposit-address` | 充值地址 |
| `POST` | `/api/v1/wallet/withdraw` | 提现 |

### 下单示例

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

响应：
```json
{"orderId":"ord_4H7XK2M9P1X","clientOrderId":"","status":"open","filledQty":"0"}
```

### gRPC 服务

| 服务 | 方法 |
|------|------|
| `UserService` | Register, Login, GetUser, UpdateKYC |
| `WalletService` | GetDepositAddress, RequestWithdrawal, GetBalances |
| `OrderService` | PlaceOrder, CancelOrder, GetOrder, GetOpenOrders |
| `MarketDataService` | GetDepth, GetTrades, GetTicker, GetKlines |

### 错误码

| HTTP | 含义 |
|------|------|
| 200 | 成功 |
| 400 | 无效请求 (价格/数量/交易对错误) |
| 401 | 未认证 (JWT 缺失或无效) |
| 403 | 无权访问 (订单不属于该用户) |
| 404 | 未找到 |
| 429 | 限流 |
| 500 | 服务器错误 |
| 503 | 服务降级 (健康检查失败) |

---

## 5. 配置

### 生产环境必填

```bash
export ENV=production
export JWT_PRIVATE_KEY_PATH=/etc/keys/jwt-private.pem
export JWT_PUBLIC_KEY_PATH=/etc/keys/jwt-public.pem
export WALLET_MASTER_SEED_HEX=<64位16进制种子>
export KAFKA_BROKERS=kafka-0:9092,kafka-1:9092,kafka-2:9092
```

### 可选配置 (含默认值)

| 变量 | 默认值 | 说明 |
|------|--------|------|
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
| `GRPC_PORT` | `50051` | gRPC 服务器端口 |
| `ETH_RPC_URL` | `http://localhost:8545` | 以太坊 RPC 地址 |

---

## 6. 部署

### Docker Compose (开发环境)

```bash
docker compose up -d       # 启动全部基础设施
docker compose down        # 停止全部
```

### 服务启动顺序

1. **基础设施**: PostgreSQL → Redis → Kafka → ClickHouse
2. **数据库迁移**: `make migrate`
3. **撮合引擎**: 核心订单撮合
4. **结算服务**: 成交结算 (Kafka 消费者)
5. **钱包服务**: 充值提现处理
6. **用户服务**: 认证
7. **API 网关**: 对外接口入口
8. **区块链监控**: 链上充值扫描

### 健康检查

```bash
curl http://localhost:8080/health
# {"status":"ok","services":{"postgres":"healthy","redis":"healthy"}}

curl http://localhost:8080/ready    # K8s 就绪探针
curl http://localhost:8080/metrics  # Prometheus 指标
```

---

## 7. 测试

### 命令

```bash
make test             # 全部测试 + 竞态检测
make test-matching    # 撮合引擎
make test-integration # 集成测试
make bench            # 性能基准
make vet              # 静态分析
```

### 生产级测试报告

| 测试项 | 结果 |
|--------|------|
| `go build ./cmd/...` (6 个二进制) | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./internal/... -race` (27 测试) | ✅ PASS |
| `go test ./test/... -race` (4 集成) | ✅ PASS |
| `go test -bench=. -benchmem` | ✅ 843 ns/op, 19 allocs/op |
| 20 并发订单压力测试 | ✅ 无竞态 |

### 测试套件

| 套件 | 数量 | 覆盖 |
|------|------|------|
| `decimal` | 9 | 定点数运算/解析/精度/四舍五入 |
| `matching` | 18 | 订单簿/FIFO/GTC/IOC/FOK/部分成交/撤单/快照 |
| `integration` | 4 | 端到端流程/市价单/FOK/并发 |

### 项目统计

| 指标 | 数值 |
|------|------|
| Go 源文件 | 75 |
| 代码行数 | 8,690 |
| Package 数 | 35 |
| 直接依赖 | 55 |
| 二进制大小 | 11M - 30M |

---

## 8. 监控

### Prometheus 指标

```
http_requests_total        HTTP 请求总数
orders_matched_total       撮合订单数
orders_rejected_total      拒绝订单数
trades_settled_total       成交结算数
settlement_errors_total    结算错误数
deposits_confirmed_total   充值确认数
withdrawals_requested_total 提现请求数
```

### 日志格式

所有服务使用结构化 JSON 日志 (zerolog)。关键字段：`level`, `time`, `message`, `user_id`, `order_id`, `symbol`, `error`。

```json
{"level":"info","time":"2026-07-06T11:52:23Z","user_id":"abc123",
 "order_id":"ord_4H7X","symbol":"ETH-USDT","message":"order placed"}
```

---

## 9. 安全

| 层面 | 方案 |
|------|------|
| **认证** | JWT RS256 (15分钟) + HMAC HS256 降级 |
| **API 签名** | HMAC-SHA256 (5秒窗口)，已实现未默认启用 |
| **密码** | bcrypt (cost=12) |
| **限流** | 令牌桶: IP 100r/s, 用户 50r/s, 下单 20r/s |
| **HTTP 头** | CSP + HSTS + X-Frame-Options + X-Content-Type-Options |
| **请求体** | 1MB MaxBytesReader |
| **SQL 注入** | 100% 参数化查询 (pgx) |
| **防双花** | `UNIQUE(tx_hash, to_address)` + 结算幂等 |
| **乐观锁** | accounts 表 `version` 列 |
| **钱包** | BIP44/secp256k1 + 用户隔离地址 + 事务提现 |
| **密钥** | 环境变量，生产强校验，无硬编码 |
| **关机** | 所有服务 30s 优雅关闭 |

---

## 10. 项目结构

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
│   ├── blockchain/                # ETH 扫描/提现 + 多链抽象
│   ├── events/                    # Kafka + Redis + Memory 事件总线
│   ├── gateway/                   # chi 路由 + 处理器 + 中间件
│   ├── grpc/                      # gRPC 服务端 + 客户端
│   ├── risk/                      # 熔断 + 限额 + 黑名单
│   ├── kyc/                       # KYC 审核 + AML 筛查
│   ├── marketdata/                # 行情数据 (Redis + 事件)
│   ├── db/                        # Postgres/Redis/ClickHouse + 迁移
│   ├── telemetry/                 # 日志 + Prometheus + 健康检查
│   ├── common/                    # 定点数/类型/错误/ID
│   └── config/                    # 环境配置
├── migrations/                    # 迁移 SQL
├── test/integration/              # 集成测试
├── docker-compose.yml             # 开发基础设施
├── Makefile
└── go.mod
```

---

## 11. 技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.25 |
| HTTP 路由 | go-chi/chi | v5 |
| 数据库 | pgx (PostgreSQL 16) | v5 |
| 缓存 | go-redis | v8 |
| 消息队列 | segmentio/kafka-go | v0.4 |
| gRPC | google.golang.org/grpc | v1.79 |
| 认证 | golang-jwt | v5 |
| 区块链 | go-ethereum + btcec/secp256k1 | v1.17 / v2.5 |
| 数据库迁移 | golang-migrate | v4 |
| OLAP | clickhouse-go | v2.47 |
| FIX | quickfixgo | v0.9 |
| 日志 | zerolog | v1.32 |
| ID 生成 | ulid | v2 |

---

## 12. 路线图

### ✅ 第一阶段 — 核心 MVP
撮合引擎 · 订单/结算/用户/钱包 · API 网关 · JWT 认证 · 事件总线 · 31 项测试

### ✅ 第二阶段 — 生产加固
gRPC · Kafka · 区块链监控 · 风控 · KYC/AML · 健康检查 + Prometheus · 安全加固

### ✅ 第三阶段 — 规模化
ClickHouse OLAP · 多链 (ETH/BSC/ARB) · 杠杆/永续合约 · FIX 4.4 · 冷钱包多签

---

## 13. 贡献

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feat/新功能`)
3. 运行测试 (`make test`)
4. 提交 (`git commit -m 'feat: 新功能描述'`)
5. 推送 (`git push origin feat/新功能`)
6. 发起 Pull Request

### 提交规范

```
feat:     新功能
fix:      修复
docs:     文档
test:     测试
refactor: 重构
perf:     性能优化
```

---

## 14. 许可证

MIT License

---

<div align="center">
  <sub>75 文件 · 8,690 行代码 · 35 包</sub>
</div>
