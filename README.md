# 加密货币交易所

基于 Go 语言构建的生产级中心化加密货币交易所，参考币安(Binance)架构设计。采用链下高性能撮合引擎 + 链上资产结算的混合架构，目标支持 EVM 兼容链（Ethereum、Arbitrum、BSC 等）。

---

## 1. 系统架构

```
                        ┌─────────────────────────────────┐
                        │        API 网关 (chi)            │
                        │   JWT + HMAC 认证 | 令牌桶限流    │
                        └──────┬──────┬──────┬─────────────┘
                               │REST  │  WS  │
               ┌───────────────┤      │      ├───────────────┐
               ▼               ▼      ▼      ▼               ▼
        ┌──────────┐  ┌──────────┐ ┌──────────┐ ┌──────────────┐
        │  用户    │  │  订单    │ │  行情    │ │   钱包       │
        │  服务    │  │  服务    │ │  数据    │ │   服务       │
        └────┬─────┘  └────┬─────┘ └────┬─────┘ └──────┬───────┘
             │              │            │              │
             └──────────────┼────────────┼──────────────┘
                            │  事件总线   │
                            ▼            ▼
                     ┌──────────┐ ┌──────────┐ ┌──────────┐
                     │  撮合    │ │  结算    │ │  风控    │
                     │  引擎    │ │  服务    │ │  服务    │
                     └──────────┘ └──────────┘ └──────────┘

数据层: PostgreSQL | Redis | ClickHouse | Kafka
```

### 服务职责矩阵

| 服务 | 核心职责 | 通信协议 |
|------|---------|---------|
| **API 网关** | 统一入口、JWT/HMAC 双通道认证、令牌桶限流、WebSocket 升级 | REST / WebSocket |
| **用户服务** | 注册登录、KYC/AML 集成、API Key 生命周期管理、2FA | gRPC（内部） |
| **订单服务** | 订单校验（余额/精度/频率）、订单状态机管理、幂等性保障 | gRPC + Kafka |
| **撮合引擎** | 内存订单簿、价格-时间优先撮合、分片无锁架构 | gRPC（管理）+ Kafka |
| **行情数据** | 深度快照、24h Ticker、K 线聚合、实时 WebSocket 推送 | Kafka 消费 → WebSocket |
| **结算服务** | 成交后余额更新（乐观锁）、手续费计算、费率阶梯、审计追踪 | Kafka 消费 + gRPC |
| **钱包服务** | HD 钱包派生（BIP32/44）、冷热钱包分离、充值扫描、提现审批 | gRPC + Kafka |
| **风控服务** | 熔断机制、提现速率管控、异常交易检测、AML 地址筛查 | Kafka 消费 + gRPC 拦截 |
| **通知服务** | 邮件/短信/Push 通知（成交、充提、风险告警） | Kafka 消费 |
| **审计服务** | 全链路不可变审计日志（ClickHouse 存储） | Kafka → ClickHouse |

---

## 2. 核心数据流

### 2.1 下单撮合流程

```
用户 ──REST/WS──▶ API 网关 ──▶ 订单服务
                                  │
                    ┌─────────────┼─────────────┐
                    ▼             ▼             ▼
               余额校验       精度校验       频率限制
                    │
              （通过）▼
         发布 OrderPlaced 事件 → Kafka → 撮合引擎
                                            │
                                     ┌──────┴──────┐
                                     ▼             ▼
                              OrderMatched    写入订单簿
                               事件            等待后续匹配
                                     │
                                     ▼
                              结算服务
                    （更新余额 + 计算手续费 + 升级费率等级）
                                     │
                        ┌────────────┼────────────┐
                        ▼            ▼            ▼
                 TradeExecuted   行情数据      通知服务
                 → ClickHouse    → Redis/WS    → Push
```

### 2.2 充值流程

```
区块链节点 ──新区块──▶ 区块扫描器
                         │
                  逐块解析交易日志
                         │
              ┌──────────┼──────────┐
              ▼                     ▼
        匹配已知充值地址         忽略其他
              │
       等待 N 个确认
       (ETH=12, Arbitrum=1)
              │
              ▼
     发布 DepositDetected → Kafka → 钱包服务
                                        │
                                  更新用户余额
                                        │
                              ┌─────────┼─────────┐
                              ▼                   ▼
                        审计日志              通知用户
```

### 2.3 提现流程

```
用户 ──REST──▶ API 网关 ──▶ 钱包服务
                                │
                      ┌─────────┼─────────┐
                      ▼                   ▼
                 余额校验              风控检查
                                  （速率/AML/黑白名单）
                      │                   │
                      └─────────┬─────────┘
                                ▼
                     ┌──────────────────┐
                     │  金额 > 热钱包阈值？ │
                     └────┬─────────┬────┘
                          ▼         ▼
                       冷钱包     热钱包
                      多签审批   在线签名
                          │         │
                          └────┬────┘
                               ▼
                       广播交易到区块链
                               │
                               ▼
                    发布 WithdrawalCompleted
                    → 审计日志 + 通知用户
```

---

## 3. 功能特性

### 3.1 撮合引擎

| 特性 | 说明 |
|------|------|
| **数据结构** | 内存订单簿，买盘降序/卖盘升序，同价位 FIFO 链表 |
| **撮合算法** | 价格-时间优先（Price-Time Priority），Maker 价格成交 |
| **并发模型** | 每交易对独立 goroutine 分片，单写者无锁设计 |
| **内存管理** | sync.Pool 复用 OrderNode，降低 GC 压力 |
| **性能基准** | 单交易对约 88 万笔/秒，单次撮合约 1.2μs（i5 笔记本） |
| **故障恢复** | 定期快照订单簿至 Redis，重启后从快照 + 事件回放恢复 |

**支持的订单类型**：

| 类型 | 说明 | 有效时间 |
|------|------|---------|
| Market（市价单） | 以当前最优价格立即成交 | — |
| Limit（限价单） | 指定价格或更优成交 | GTC / IOC / FOK |
| Stop-Loss（止损单） | 触发价到达后转为市价单 | GTC |
| Stop-Limit（止盈止损单） | 触发价到达后转为限价单 | GTC |
| Iceberg（冰山单） | 仅展示部分数量，隐藏真实意图 | GTC |
| Post-Only（只做 Maker） | 确保为被动成交，不提取流动性 | GTC |

### 3.2 钱包系统

| 模块 | 实现方案 |
|------|---------|
| **密钥派生** | BIP32 / BIP44 层级确定性钱包，路径 `m/44'/60'/0'/0/{index}` |
| **热钱包** | 在线签名，余额上限管控，超限自动归集至冷钱包，每日密钥轮换 |
| **冷钱包** | M-of-N 多签（3/5 或 5/7），HSM 多地分布，离线签名仪式 |
| **充值检测** | 逐块扫描 EVM 事件日志，确认数达标后入账，防双花唯一约束 |
| **提现审批** | 分级审批：小额自动热钱包 → 大额人工+多签 → 超大额冷钱包签章 |
| **Gas 管理** | 动态 Gas Price 缓存，慢/正常/快三档倍率，EIP-1559 兼容 |
| **重组处理** | 滑动窗口维持最后 N 个区块，parentHash 断裂检测 → 回滚 → 重扫 |

### 3.3 结算系统

- **乐观锁**：`version` 字段保证余额并发更新的正确性
- **手续费阶梯**：根据 30 日滚动交易量自动升级费率等级
- **审计追踪**：每笔余额变更写入 `balance_transactions` 表，含变更前后快照
- **手续费等级**：

| 等级 | 30日交易量 (USD) | Maker 费率 | Taker 费率 |
|------|-----------------|-----------|-----------|
| 0 | $0 - $50,000 | 0.100% | 0.100% |
| 1 | $50,000 - $500,000 | 0.080% | 0.090% |
| 2 | $500,000 - $5,000,000 | 0.050% | 0.075% |
| 3 | $5,000,000 - $50,000,000 | 0.020% | 0.060% |
| 4 | $50,000,000 以上 | 0.000% | 0.050% |

### 3.4 安全体系

| 安全层面 | 实施方案 |
|---------|---------|
| **Web 认证** | JWT（RS256，15min 过期）+ Refresh Token（7 天），httpOnly Cookie |
| **API 认证** | HMAC-SHA256 签名（API Key + Secret），5 秒时间窗口防重放 |
| **双因素认证** | TOTP（Time-based One-Time Password） |
| **限流** | 令牌桶算法：IP 100r/s，API Key 50r/s，下单 20r/s |
| **服务间通信** | mTLS（Istio/Linkerd Service Mesh） |
| **密钥管理** | HSM/KMS → 内存加密存储 → mlock 页面锁定 → 用后清零 |
| **SQL 注入** | 100% 参数化查询，pgx 驱动 |
| **防双花** | `(tx_hash, to_address)` 数据库 UNIQUE 约束 |
| **DDoS** | Cloudflare → K8s Ingress → 服务级限流三级防护 |
| **审计** | 全量操作日志写入 ClickHouse 不可变存储 |

---

## 4. 项目结构

```
Cryptocurrency-exchange/
├── cmd/                          # 各服务启动入口
│   ├── api-gateway/              # API 网关（端口 8080）
│   ├── matching-engine/          # 撮合引擎
│   ├── user-service/             # 用户服务
│   ├── wallet-service/           # 钱包服务
│   └── settlement-service/       # 结算服务
│
├── internal/                     # 私有业务包（不可外部引用）
│   ├── matching/                 # 撮合引擎核心
│   │   ├── engine.go             # 分片引擎调度器 + 事件发布
│   │   ├── orderbook.go          # 订单簿（排序价格层 + FIFO 队列）
│   │   ├── matcher.go            # 价格-时间优先撮合算法
│   │   ├── types.go              # Order / MatchResult / Side / OrderType
│   │   └── pool.go               # sync.Pool 内存复用
│   │
│   ├── order/                    # 订单服务
│   │   ├── service.go            # 下单/撤单业务编排
│   │   ├── validator.go          # 多维度订单校验
│   │   ├── lifecycle.go          # 订单状态机
│   │   └── repository.go         # 订单持久化（pgx）
│   │
│   ├── user/                     # 用户服务
│   │   ├── service.go            # 注册/登录/状态管理
│   │   └── auth.go               # JWT + HMAC-SHA256 认证
│   │
│   ├── wallet/                   # 钱包服务
│   │   ├── service.go            # 充值/提现/地址管理
│   │   └── hdwallet.go           # BIP32/44 HD 钱包派生
│   │
│   ├── settlement/               # 结算服务
│   │   ├── service.go            # 余额更新（乐观锁）+ 审计
│   │   └── fees.go               # 手续费计算 + 费率阶梯
│   │
│   ├── marketdata/               # 行情数据服务
│   │   └── service.go            # Ticker / K线 / 深度 / 成交
│   │
│   ├── gateway/                  # API 网关
│   │   ├── router.go             # 路由注册（chi）
│   │   ├── handler/              # 请求处理器
│   │   │   ├── order.go          # 订单相关
│   │   │   └── market.go         # 行情相关
│   │   └── middleware/           # 中间件
│   │       ├── auth.go           # JWT + HMAC 认证
│   │       └── ratelimit.go      # 令牌桶限流
│   │
│   ├── events/                   # 事件总线
│   │   ├── types.go              # 13 种事件类型定义
│   │   └── producer.go           # 内存实现（开发）/ Kafka 实现（生产）
│   │
│   ├── db/                       # 数据访问层
│   │   ├── postgres/
│   │   │   ├── connection.go     # pgxpool 连接池
│   │   │   └── migrations/       # DDL 迁移脚本
│   │   └── redis/
│   │       └── connection.go     # Redis 客户端
│   │
│   ├── common/                   # 公共工具
│   │   ├── decimal/              # 18 位定点数（金融计算）
│   │   ├── types.go              # 领域类型定义
│   │   ├── errors.go             # 领域错误
│   │   └── idgen.go              # ULID 生成器
│   │
│   ├── config/                   # 配置管理
│   │   └── config.go             # 环境变量 + 默认值
│   │
│   └── telemetry/                # 可观测性
│       └── logging.go            # 结构化日志（zerolog）
│
├── api/proto/                    # Protobuf 接口定义
├── deployments/                  # Kubernetes 部署清单
│   ├── base/                     # 基础配置
│   └── overlays/                 # 环境覆盖（staging / production）
├── test/                         # 测试
│   └── integration/              # 端到端集成测试
├── scripts/                      # 运维脚本
├── docker-compose.yml            # 本地开发环境
├── Makefile                      # 构建 & 测试命令
├── go.mod                        # Go 模块定义
└── go.sum                        # 依赖校验
```

---

## 5. 快速开始

### 5.1 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.23+ | 编译运行 |
| Docker | 24+ | 本地基础设施 |
| PostgreSQL | 16 | 核心业务数据 |
| Redis | 7 | 缓存 & 实时数据 |
| Kafka | 3.5+ | 异步事件总线（生产环境） |

### 5.2 本地启动

```bash
# 1. 克隆仓库
git clone https://github.com/1952154539/Cryptocurrency-exchange.git
cd Cryptocurrency-exchange

# 2. 启动基础设施（PostgreSQL + Redis + Kafka + Anvil 本地链）
docker compose up -d

# 3. 初始化数据库（自动建表 + 种子数据）
make migrate

# 4. 下载 Go 依赖
go mod tidy

# 5. 启动撮合引擎
go run ./cmd/matching-engine &

# 6. 启动 API 网关
go run ./cmd/api-gateway &

# 7. 健康检查
curl http://localhost:8080/api/v1/ping
# 返回: {"status":"ok"}

# 8. 查看服务时间
curl http://localhost:8080/api/v1/time
# 返回: {"serverTime":1719900000000}
```

### 5.3 构建二进制

```bash
# 构建所有服务
make build

# 各服务二进制位于 bin/ 目录
ls bin/
# api-gateway  matching-engine  user-service  wallet-service  settlement-service
```

---

## 6. API 接口文档

### 6.1 公开接口（无需认证）

```
GET  /api/v1/ping                             服务心跳
GET  /api/v1/time                             服务器时间（毫秒时间戳）
GET  /api/v1/depth?symbol=ETH-USDT&limit=100   订单簿深度
GET  /api/v1/trades?symbol=ETH-USDT&limit=500  最近成交记录
GET  /api/v1/klines?symbol=ETH-USDT&interval=1h&limit=100  K线数据
GET  /api/v1/ticker/24hr?symbol=ETH-USDT       24小时行情统计
```

### 6.2 私有接口（需 JWT 或 HMAC 签名认证）

```
# 账户
GET    /api/v1/account                                    查询账户余额

# 订单
POST   /api/v1/order                                      下单
DELETE /api/v1/order                                      撤单
GET    /api/v1/order?symbol=ETH-USDT&orderId=xxx           查询订单
GET    /api/v1/open-orders?symbol=ETH-USDT                 查询当前挂单

# 钱包
GET    /api/v1/deposit/address?currency=ETH&chain=ethereum  获取充值地址
GET    /api/v1/deposit/history?currency=ETH&limit=100       充值历史
POST   /api/v1/withdraw                                    申请提现
GET    /api/v1/withdraw/history?currency=ETH&limit=100      提现历史
```

### 6.3 下单请求示例

```json
POST /api/v1/order
Content-Type: application/json
Authorization: Bearer <JWT_TOKEN>

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
{
  "orderId": "ord_4H7XK2M9P1",
  "clientOrderId": "",
  "status": "open",
  "filledQty": "0"
}
```

### 6.4 HMAC 签名规范

交易 API 支持 HMAC-SHA256 签名认证：

```
请求头:
  X-API-Key:     <您的 API Key>
  X-Timestamp:   <Unix 毫秒时间戳>
  X-Signature:   HMAC-SHA256(Secret, timestamp + method + path + query + body)

签名有效期为 5 秒，超出窗口将被拒绝。
```

---

## 7. 数据库设计

### 7.1 核心表结构

| 表名 | 说明 | 关键索引 |
|------|------|---------|
| `users` | 用户账户 | email(UNIQUE), status |
| `accounts` | 用户资产余额 | (user_id, currency) UNIQUE, version(乐观锁) |
| `api_keys` | API 密钥 | api_key(UNIQUE) |
| `markets` | 交易对配置 | symbol(UNIQUE) |
| `orders` | 订单记录 | (user_id, created_at), (symbol, status) |
| `trades` | 成交记录 | (symbol, executed_at), executed_at |
| `deposits` | 充值记录 | (tx_hash, to_address) UNIQUE |
| `deposit_addresses` | 充值地址簿 | (address, chain) UNIQUE |
| `withdrawals` | 提现记录 | withdrawal_id(UNIQUE), status |
| `balance_transactions` | 余额变动审计 | (user_id, created_at) |
| `fee_tiers` | 手续费等级 | tier_name(UNIQUE) |
| `user_volume_30d` | 用户30日交易量 | user_id(PK) |

### 7.2 Redis 缓存键规范

```
book:{symbol}:bids              → Sorted Set  订单簿买盘深度
book:{symbol}:asks              → Sorted Set  订单簿卖盘深度
ticker:{symbol}                 → Hash        24h 行情统计
trades:{symbol}                 → List        最近 500 笔成交
candle:{symbol}:{interval}      → Hash        当前 K 线
ratelimit:{user_id}             → String      令牌桶计数器
session:{session_id}            → Hash        用户会话
order:{order_id}                → Hash        订单热缓存
balance:{user_id}:{currency}    → String      余额缓存
```

---

## 8. 技术栈

| 组件 | 技术选型 | 说明 |
|------|---------|------|
| **编程语言** | Go 1.23+ | 高并发、内存安全、部署简单 |
| **HTTP 框架** | go-chi/chi v5 | 轻量级、中间件友好、兼容 net/http |
| **数据库驱动** | pgx v5 | 高性能 PostgreSQL 驱动，原生连接池 |
| **缓存** | go-redis v8 | Redis 客户端，集群/哨兵模式支持 |
| **消息队列** | 内存实现（开发）/ Kafka（生产） | 异步解耦，消费者组负载均衡 |
| **认证** | golang-jwt v5 | JWT 签发与验证 |
| **密码哈希** | golang.org/x/crypto | bcrypt（cost=12） |
| **ID 生成** | oklog/ulid v2 | 时间有序、全局唯一 |
| **日志** | rs/zerolog | 结构化 JSON，零分配 |
| **定点数** | math/big（自研封装） | 18 位精度，避免浮点误差 |
| **容器编排** | Docker + Kubernetes | 本地开发 → 生产部署一致 |
| **区块链** | EVM 兼容链 | Ethereum / Arbitrum / BSC |

---

## 9. 测试

### 9.1 运行测试

```bash
# 全部测试
make test

# 仅撮合引擎
make test-matching

# 集成测试（需 Docker 环境）
make test-integration

# 性能基准
make bench
```

### 9.2 测试报告（最近一次运行）

```
=== 单元测试 ===
common/decimal:     8/8   PASS   (定点数运算)
matching:          13/13  PASS   (撮合引擎)

=== 集成测试 ===
EndToEndMatchingFlow     PASS   (完整下单→撮合→撤单)
MarketOrderEndToEnd      PASS   (市价单扫单)
FOKOrder                 PASS   (FOK 可成交性校验)
ConcurrentOrders         PASS   (20 并发订单串行化)

=== 性能基准 ===
BenchmarkOrderBook_Matching-8   878,385 ops/sec   1,224 ns/op
```

### 9.3 测试覆盖场景

| 场景 | 测试用例 |
|------|---------|
| 限价买单完全成交 | `TestMatch_LimitBuyFullyFilled` |
| 限价买单部分成交 | `TestMatch_LimitBuyPartialFill` |
| 限价买单不穿价 | `TestMatch_LimitBuyNoCross` |
| 市价买单扫单 | `TestMatch_MarketBuy` |
| 卖单撮合对称性 | `TestMatch_SellSide` |
| FOK 可成交预检 | `TestCanFillFOK` |
| 撤单簿中移除 | `TestCancelOrder` |
| 同价位 FIFO 排序 | `TestOrderBook_SamePriceFIFO` |
| 订单簿快照 | `TestSnapshot` |
| 并发安全性 | `TestConcurrentOrders` |

---

## 10. 部署架构

### 10.1 Kubernetes 拓扑

```
                          ┌──────────────────────┐
                          │   Cloudflare WAF     │
                          └──────────┬───────────┘
                                     │
                          ┌──────────▼───────────┐
                          │  Nginx Ingress       │
                          │  (TLS Termination)   │
                          └──────────┬───────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    ▼                ▼                ▼
             ┌──────────┐    ┌──────────┐    ┌──────────┐
             │ API GW   │    │ API GW   │    │ API GW   │
             │ (Pod x3) │    │ (Pod x3) │    │ (Pod x3) │
             └────┬─────┘    └────┬─────┘    └────┬─────┘
                  └───────────────┼───────────────┘
                                  │
                   ┌──────────────┼──────────────┐
                   ▼              ▼              ▼
            ┌──────────┐  ┌──────────┐  ┌──────────┐
            │ Order    │  │ Matching │  │ Wallet   │
            │ Service  │  │ Engine   │  │ Service  │
            │ (HPA)    │  │ (STS)    │  │ (HPA)    │
            └──────────┘  └──────────┘  └──────────┘
                   │              │              │
                   └──────────────┼──────────────┘
                                  │
              ┌───────────────────┼───────────────────┐
              ▼                   ▼                   ▼
       ┌──────────┐       ┌──────────┐        ┌──────────┐
       │PostgreSQL│       │  Redis   │        │  Kafka   │
       │(HA Pair) │       │(Cluster) │        │(Cluster) │
       └──────────┘       └──────────┘        └──────────┘
```

### 10.2 可观测性

| 维度 | 工具 | 关键指标 |
|------|------|---------|
| **业务指标** | Prometheus + Grafana | 订单量/秒、成交量/秒、活跃用户数、充提金额 |
| **系统指标** | Prometheus Node Exporter | CPU、内存、Goroutine 数、GC 暂停时间 |
| **延迟** | Prometheus Histogram | API P50/P99/P999、撮合延迟 P50/P99 |
| **日志** | zerolog → Loki | 结构化 JSON，trace_id 串联全链路 |
| **链路追踪** | OpenTelemetry → Jaeger | gRPC metadata 传播，Kafka 事件携带 trace_id |
| **告警** | AlertManager → PagerDuty | 服务宕机、撮合延迟超阈值、提现失败、充值延迟 |

---

## 11. 开发路线图

### Phase 1 — MVP（已完成）

- [x] 项目脚手架 + 目录结构 + Docker Compose 开发环境
- [x] 公共包（18 位定点数、领域类型、ULID 生成器）
- [x] 撮合引擎（内存订单簿、价格-时间优先、GTC/IOC/FOK）
- [x] 订单服务（校验器、状态机、持久化）
- [x] 结算服务（乐观锁余额更新、手续费计算、审计追踪）
- [x] 用户服务（JWT + HMAC 认证、注册登录、API Key 管理）
- [x] 钱包服务（HD 派生、充值检测、提现流程、冷热分离）
- [x] API 网关（REST 路由、WebSocket、令牌桶限流）
- [x] 数据库 Schema（12 张核心表 + Redis 缓存层）
- [x] 事件总线（内存实现 + Kafka 接口）
- [x] 单元测试 25 个 + 集成测试 4 个 + 性能基准

### Phase 2 — 生产加固（规划中）

- [ ] 多链支持（Ethereum 主网 + BSC + Arbitrum + Optimism）
- [ ] 冷钱包多签系统（3-of-5 HSM 多地分布）
- [ ] 风控系统（价格熔断、提现速率限制、异常交易检测）
- [ ] AML/KYC 集成（Chainalysis / Elliptic 地址筛查）
- [ ] ClickHouse 历史数据（成交/K线/审计日志 OLAP 查询）
- [ ] 高级订单类型（止损、冰山、OCO、追踪止损）
- [ ] 多区域部署（active-active API + active-passive DB）
- [ ] 渗透测试 + SOC 2 Type I 审计

### Phase 3 — 规模化（规划中）

- [ ] 杠杆交易（逐仓 + 全仓保证金）
- [ ] 永续合约 / 交割合约
- [ ] FIX 协议支持（机构客户接入）
- [ ] Staking / 理财 / Launchpad 产品
- [ ] 做市商激励计划（API + 返佣）
- [ ] ISO 27001 / SOC 2 Type II 认证

---

## 12. 运维命令

```bash
# 构建
make build              # 编译所有服务

# 测试
make test               # 全部测试
make test-matching      # 撮合引擎测试
make bench              # 性能基准

# 代码质量
make lint               # golangci-lint 检查
make fmt                # 格式化
make vet                # 静态分析

# Docker
make docker-up          # 启动基础设施
make docker-down        # 停止基础设施
make docker-build       # 构建镜像

# 数据库
make migrate            # 执行迁移

# 清理
make clean              # 删除编译产物
```

---

## 13. 许可证

MIT License

---

> **注意**：本项目为 MVP 阶段，适合学习和二次开发。生产部署前请完成 Phase 2 安全加固，并完成独立的第三方安全审计。
