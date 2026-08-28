# 分布式限流 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/config/config.go` | 新增 `RateLimit` 配置结构体 |
| 新建 | `internal/middlewares/ratelimit/limiter.go` | `Limiter` 接口 + `Result` |
| 新建 | `internal/middlewares/ratelimit/token_bucket.go` | 内存令牌桶（单机） |
| 新建 | `internal/middlewares/ratelimit/sliding_window.go` | Redis ZSet 滑动窗口 + Lua |
| 新建 | `internal/middlewares/ratelimit/ratelimit_test.go` | 单元测试 |
| 新建 | `internal/middlewares/ratelimit.go` | `RateLimit` 中间件 + `BuildLimiter` |
| 修改 | `internal/routers/router.go` | `RegisterRoutes` 签名加 limiter |
| 修改 | `internal/role/api.go` | 构造 limiter 并注入 |
| 修改 | `conf/app.toml` / `conf/app.docker.toml` | 新增 `[ratelimit]` 段 |

> 测试复用已引入的 `miniredis`（上个子项目已加，无新增依赖）。

---

## T1: 新增 RateLimit 配置结构体

**文件：** `internal/config/config.go`
**依赖：** 无

**步骤：**
1. 在 `Cache` 结构体之后新增 `RateLimit` 结构体，字段（`mapstructure` 与 toml 键一致）：
   - `Enabled bool` → `enabled`
   - `Algorithm string` → `algorithm`（`token_bucket` | `sliding_window`）
   - `TokenRate int` → `token_rate`
   - `TokenBurst int` → `token_burst`
   - `Window int` → `window`（秒）
   - `Limit int` → `limit`

**验证：** `go build ./internal/config/...` 编译通过。

---

## T2: Limiter 接口与 Result

**文件：** `internal/middlewares/ratelimit/limiter.go`
**依赖：** 无

**步骤：**
1. 声明包 `ratelimit`。
2. 定义 `Result` 结构体：`Allowed bool`、`Limit int`、`Remaining int`、`ResetAt time.Time`。
3. 定义 `Limiter` 接口：`Allow(ctx context.Context, key string) (Result, error)`。

**验证：** `go build ./internal/middlewares/ratelimit/...` 编译通过。

---

## T3: 内存令牌桶

**文件：** `internal/middlewares/ratelimit/token_bucket.go`
**依赖：** T2

**步骤：**
1. 定义 `TokenBucket`（`mu sync.Mutex; rate, burst float64; buckets map[string]*bucket`）与内部 `bucket{tokens float64; last time.Time}`。
2. 实现 `NewTokenBucket(rate, burst int) *TokenBucket`（rate/burst 转 float64，初始 buckets 空 map）。
3. 实现 `Allow(ctx, key)`：
   - 加锁；取或新建 `buckets[key]`（新建时 `tokens=burst`、`last=now`）；
   - 按 `elapsed = now-last` 补充 `tokens = min(burst, tokens + elapsed*rate)`，更新 `last=now`；
   - `tokens >= 1` 则扣 1 返回 `Allowed=true, Limit=burst, Remaining=floor(tokens), ResetAt=now+max(0,(1-tokens)/rate)`；
   - 否则返回 `Allowed=false, Remaining=0, ResetAt=now+(1-tokens)/rate`。

**验证：** `go build ./internal/middlewares/ratelimit/...` 编译通过。

---

## T4: Redis ZSet 滑动窗口

**文件：** `internal/middlewares/ratelimit/sliding_window.go`
**依赖：** T2

**步骤：**
1. 定义 `SlidingWindow{client *redis.Client; window time.Duration; limit int; script *redis.Script}`。
2. 定义 Lua 脚本（`redis.NewScript`），逻辑：
   - `ZREMRANGEBYSCORE key 0 (now-window_ms)` 清旧记录；
   - `ZCARD` 计数；`count < limit` 则 `ZADD key now_ms member` + `PEXPIRE key window_ms`，返回 `{1, limit-count-1, now_ms+window_ms}`；
   - 超限则 `ZRANGE key 0 0 WITHSCORES` 取最早 score 算 `resetAt=oldest+window_ms`，返回 `{0, 0, resetAt}`。
3. 实现 `NewSlidingWindow(client, window, limit)`：key 前缀 `vistack:ratelimit`。
4. 实现 `Allow(ctx, key)`：`member = uuid.NewString()`，`now = time.Now().UnixMilli()`，`script.Run(ctx, client, []string{"vistack:ratelimit:"+key}, now, windowMs, limit, member).Int64Slice()`；解析 3 个值构造 `Result`（`ResetAt` 用 `time.UnixMilli(resetMs)`）。
5. Run 出错（Redis 不可用）返回 err（由中间件 fail-open）。

**验证：** `go build ./internal/middlewares/ratelimit/...` 编译通过。

---

## T5: 单元测试

**文件：** `internal/middlewares/ratelimit/ratelimit_test.go`
**依赖：** T3、T4（用 miniredis）

**步骤：**
1. `TestTokenBucketBurst`：`NewTokenBucket(1, 2)`，前 2 个 Allow 放行、第 3 个拒绝；`time.Sleep(1.1s)` 后第 4 个放行。
2. `TestTokenBucketPerKey`：两个不同 key 各自独立计数。
3. `TestSlidingWindowLimit`：`NewSlidingWindow(client, 1s, 3)`，连续 3 个放行、第 4 个拒绝（用 miniredis）。
4. `TestSlidingWindowSlide`：窗口 200ms、limit 1；第 1 个放行、第 2 个拒绝；`Sleep(250ms)` 后第 3 个放行。

**验证：** `go test ./internal/middlewares/ratelimit/...` 全部通过。

---

## T6: RateLimit 中间件 + BuildLimiter

**文件：** `internal/middlewares/ratelimit.go`
**依赖：** T1、T2、T3、T4

**步骤：**
1. 实现 `BuildLimiter(cfg config.RateLimit, rdb *redis.Client) ratelimit.Limiter`：
   - `!cfg.Enabled` 返回 nil；
   - `algorithm == "token_bucket"`：rate/burst 零值默认 10/20，返回 `NewTokenBucket`；
   - 否则（`sliding_window` 或空）：window/limit 零值默认 60/100，返回 `NewSlidingWindow(rdb, ...)`。
2. 实现 `RateLimit(limiter ratelimit.Limiter) gin.HandlerFunc`：
   - `limiter == nil` 返回直通 handler；
   - `userID := auth.GetUserID(c)`；`userID == 0` 直通；
   - `res, err := limiter.Allow(c.Request.Context(), strconv.FormatInt(userID, 10))`；
   - `err != nil`：记日志（fail-open）并 `c.Next()`；
   - 写 `X-RateLimit-Limit/Remaining/Reset` 头；
   - `!res.Allowed`：写 `Retry-After`（`max(1, int(time.Until(resetAt).Seconds()))`），返回 429 JSON + `c.Abort()`；
   - 否则 `c.Next()`。

**验证：** `go build ./internal/middlewares/...` 编译通过。

---

## T7: RegisterRoutes 挂载

**文件：** `internal/routers/router.go`
**依赖：** T6

**步骤：**
1. `RegisterRoutes` 签名改为 `(r *gin.Engine, validator authpkg.TokenValidator, limiter ratelimit.Limiter)`。
2. 在 `AuthApiGroup.Use(middlewares.AuthMiddleware(validator))` 之后新增 `AuthApiGroup.Use(middlewares.RateLimit(limiter))`。
3. 引入 `internal/middlewares/ratelimit` 包。

**验证：** `go build ./internal/routers/...` 编译通过。

---

## T8: role/api.go 构造注入

**文件：** `internal/role/api.go`
**依赖：** T6、T7

**步骤：**
1. 在 `core.InitRedis(cfg)` / `core.InitCache(cfg)` 之后构造 `limiter := middlewares.BuildLimiter(cfg.RateLimit, core.Redis)`。
2. `routers.RegisterRoutes(r, verifier, limiter)`。

**验证：** `go build ./...` 编译通过。

---

## T9: 配置文件新增 [ratelimit] 段

**文件：** `conf/app.toml`、`conf/app.docker.toml`
**依赖：** 无（可与 T1 并行）

**步骤：**
1. 两个文件在 `[cache]` 段之后新增：
   ```toml
   [ratelimit]
   enabled = true
   algorithm = "sliding_window"
   token_rate = 10
   token_burst = 20
   window = 60
   limit = 100
   ```

**验证：** 目视确认两文件均含 `[ratelimit]`；`go build ./...` 通过。

---

## 执行顺序

```
T1 ──┐
T2 ──┼──> T3 ──┐
     │         ├──> T5 ──> T6 ──> T7 ──> T8
     └──> T4 ──┘
T9（可并行）
```

- T1、T2、T9 无依赖可并行；T3/T4 依赖 T2；T5 依赖 T3+T4；T6 依赖 T1+T3+T4；T7 依赖 T6；T8 依赖 T6+T7。
