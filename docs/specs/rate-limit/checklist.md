# 分布式限流 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] `Limiter` 接口与 `Result` 已实现（验证：`go build ./internal/middlewares/ratelimit/...`）
- [ ] 内存令牌桶已实现 `NewTokenBucket`/`Allow`（验证：编译 + 单测）
- [ ] Redis 滑动窗口已实现 `NewSlidingWindow`/`Allow`（含 Lua 脚本）（验证：编译 + 单测）
- [ ] `RateLimit` 中间件与 `BuildLimiter` 已实现（验证：编译 + 中间件单测）

## 集成

- [ ] `RegisterRoutes` 签名含 `limiter`，且 `AuthApiGroup` 在 `AuthMiddleware` 之后挂 `RateLimit`（验证：grep 调用点）
- [ ] `role/api.go` 用 `BuildLimiter(cfg.RateLimit, core.Redis)` 构造并注入（验证：grep 调用点）
- [ ] 配置文件含 `[ratelimit]` 段（验证：grep 两处 toml）

## 编译与测试

- [ ] `go build ./...` 全量编译通过
- [ ] `go test ./internal/middlewares/ratelimit/...` 全部通过
- [ ] `go test ./internal/middlewares/...`（含中间件 httptest）全部通过
- [ ] `go vet ./internal/middlewares/...` 无告警

## 端到端场景

- [ ] **场景 1（令牌桶突发）**：`rate=1, burst=2` 时，同一用户突发 2 个请求放行、第 3 个 429；等待 ~1s 后恢复放行（验证：单测 `TestTokenBucketBurst`）
- [ ] **场景 2（滑动窗口超限）**：`window=1s, limit=3` 时，同一用户第 4 个请求 429；窗口滑过后恢复（验证：单测 `TestSlidingWindowLimit` + `TestSlidingWindowSlide`）
- [ ] **场景 3（中间件 429 + 响应头）**：构造带 `claims` 的 gin 请求，超过阈值后返回 429，且含 `Retry-After` + `X-RateLimit-Limit/Remaining/Reset`（验证：中间件 httptest）
- [ ] **场景 4（Redis 降级 fail-open）**：滑动窗口指向不可用 Redis 时，`Allow` 返回 error，中间件放行不返回 429（验证：单测指向死端口 Redis）
