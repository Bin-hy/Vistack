# 分布式限流 Plan

## 架构概览

新增纯限流子包 `internal/middlewares/ratelimit`（无 gin 依赖），内含 `Limiter` 接口与两种实现：

- **`TokenBucket`**：进程内令牌桶（单机，per-key 桶，`sync.Mutex` 保护）。
- **`SlidingWindow`**：Redis ZSet 滑动窗口（分布式，单个 Lua 脚本原子执行）。

`internal/middlewares/ratelimit.go` 提供 gin 胶水：`RateLimit(limiter)` 中间件 + `BuildLimiter(cfg, rdb)` 构造器（从配置选算法、套默认值）。

为避免 `middlewares` → `core` 循环依赖，Redis 客户端由调用方（`role/api.go`）传入 `BuildLimiter`，再经 `RegisterRoutes` 注入中间件。

## 核心数据结构

### ratelimit.Result（放行结果，用于响应头）

```go
type Result struct {
    Allowed   bool
    Limit     int       // 总配额
    Remaining int       // 剩余可用量
    ResetAt   time.Time // 重置/可再请求时间
}
```

### ratelimit.Limiter（统一接口）

```go
type Limiter interface {
    // Allow 判断 key 是否放行；返回放行结果。err 表示依赖（Redis）不可用。
    Allow(ctx context.Context, key string) (Result, error)
}
```

### ratelimit.TokenBucket（单机）

```go
type TokenBucket struct {
    mu      sync.Mutex
    rate    float64          // 每秒补充令牌数
    burst   float64          // 桶容量
    buckets map[string]*bucket // key -> 桶状态
}
type bucket struct {
    tokens float64   // 当前令牌数
    last   time.Time // 上次补充时间
}
func NewTokenBucket(rate, burst int) *TokenBucket
```

### ratelimit.SlidingWindow（分布式）

```go
type SlidingWindow struct {
    client *redis.Client
    window time.Duration
    limit  int
    script *redis.Script // 原子 Lua
}
func NewSlidingWindow(client *redis.Client, window time.Duration, limit int) *SlidingWindow
```

## 模块设计

### 1. `ratelimit.TokenBucket`（token_bucket.go）

**职责**：单机令牌桶，按 key（用户 ID）维护独立桶。

**算法**：`Allow` 加锁后——按 `now-last` 计算补充令牌 `tokens = min(burst, tokens + elapsed*rate)`；有令牌则扣 1 放行，否则拒绝。`Limit=burst`、`Remaining=floor(tokens)`、`ResetAt=now + max(0, (1-tokens)/rate)`（下一次可得令牌时间）。

**说明**：per-key 桶 map 会随用户数增长（受限于注册用户数），本次不做过期清理（记录为已知边界）。

### 2. `ratelimit.SlidingWindow`（sliding_window.go）

**职责**：分布式滑动窗口，跨实例共享 Redis ZSet。

**Lua 脚本**（原子）：`ZREMRANGEBYSCORE` 清窗口外旧记录 → `ZCARD` 计数 → 未超限则 `ZADD`（score=now_ms，member=uuid 保证全局唯一）+ `PEXPIRE` 防 key 永存 → 超限则取最早记录 score 计算 reset。返回 `{allowed, remaining, reset_ms}`。

**Key 设计**：`vistack:ratelimit:<key>`（key 为用户 ID）。

### 3. `middlewares.RateLimit`（ratelimit.go 胶水）

**职责**：从 gin 上下文取用户 ID（`auth.GetUserID`）→ 调 `limiter.Allow(ctx, userID)` → 写 `X-RateLimit-Limit/Remaining/Reset` 头 → 超限返回 429 + `Retry-After`。

**降级**：`Allow` 返回 err（Redis 不可用）→ 放行并记日志；`limiter == nil`（未启用）→ 直接 `c.Next()`。

**BuildLimiter(cfg, rdb)**：读 `config.RateLimit`，`Enabled=false` 返回 nil；`algorithm` 为 `token_bucket` 构造令牌桶、`sliding_window`（或空，默认）构造滑动窗口；零值套默认值（rate=10、burst=20、window=60s、limit=100）。

### 4. 装配（routers + role）

- `routers.RegisterRoutes(r, validator, limiter)` 签名新增 `limiter`，在 `AuthApiGroup` 的 `AuthMiddleware` 之后挂 `middlewares.RateLimit(limiter)`。
- `role/api.go`：`core.InitRedis` 后 `limiter := middlewares.BuildLimiter(cfg.RateLimit, core.Redis)`，传入 `RegisterRoutes`。

## 模块交互

```
请求 → AuthMiddleware（解析 userID，写入 ctx）
      → RateLimit：key=userID → limiter.Allow(ctx, key)
          ├─ TokenBucket：进程内桶扣减
          └─ SlidingWindow：Lua 原子 ZSet 计数
      → 放行 → handler
      → 拒绝 → 429 + Retry-After + X-RateLimit-*
```

## 文件组织

```
internal/middlewares/ratelimit/
├── limiter.go          — Limiter 接口 + Result
├── token_bucket.go     — 内存令牌桶
├── sliding_window.go   — Redis ZSet 滑动窗口 + Lua 脚本
└── *_test.go           — 单元测试（令牌桶突发、滑动窗口超限/窗口滑动、Lua 原子）

internal/middlewares/
└── ratelimit.go        — RateLimit 中间件 + BuildLimiter

internal/config/
└── config.go           — 新增 RateLimit 配置结构体

internal/routers/
└── router.go           — RegisterRoutes 签名加 limiter，挂 AuthApiGroup

internal/role/
└── api.go              — 构造 limiter 并传入 RegisterRoutes

conf/
├── app.toml            — 新增 [ratelimit] 段
└── app.docker.toml     — 新增 [ratelimit] 段
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 令牌桶 | 自实现（per-key map + mutex） | 面试可讲清令牌补充/容量/突发原理；不引 `x/time/rate` |
| 滑动窗口 | Redis ZSet + Lua 单脚本 | ZSet score=时间戳天然适配窗口；Lua 保证「清理+计数+写」原子，避免竞态 |
| member 唯一性 | `uuid.NewString()` | 同一毫秒多请求需不同 member，否则 ZSet 去重导致漏计；uuid 全局唯一且已是现有依赖 |
| key 超时 | `PEXPIRE key window` | 防止冷 key 永久占用内存 |
| 循环依赖规避 | Redis 客户端经 `BuildLimiter(cfg, rdb)` 注入 | `middlewares` 不可 import `core`（core/server.go 已 import middlewares） |
| 降级策略 | fail-open（Redis 不可用放行） | 限流器不应比其保护的接口更脆弱 |
| 算法默认 | `sliding_window` | 项目核心是「分布式/可水平扩展」，分布式限流为默认语义 |
| 429 响应 | 429 + `Retry-After` + `X-RateLimit-*` | 标准限流契约，前端/网关可识别 |
| per-route 阈值 | 本期不做，全局一套 | spec 已明确留待后续 |
