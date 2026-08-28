# Redis 缓存层 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] 通用缓存组件 `cache.Cache` 已实现（验证：`go build ./internal/core/cache/...` 编译通过）
- [ ] `GetVideoInfo` 改为调用 `Cache.GetOrLoad`（验证：grep 不再出现手写 `videoInfo:` key 拼接与裸 `SetNX` 锁）
- [ ] `GetVideoRecommend` 改为调用 `Cache.GetOrLoad` 且带 `WithTTL`（验证：grep 调用点存在）
- [ ] 布隆过滤器 `cache.Bloom` 已实现 `Build`/`Add`/`Exists`（验证：`go test` 覆盖哈希与就绪标志）
- [ ] 锁释放使用 Lua owner-check（验证：代码审查 `releaseLock` 为 Lua 脚本，非裸 `DEL`）

## 集成

- [ ] `core.InitCache` 在 `RunAPI` 中于 `InitRedis` 之后被调用（验证：`go build ./...` + 启动日志含缓存初始化）
- [ ] 启动时异步 `BuildVideoBloom` 执行，布隆 `ready` 标志置 1（验证：启动后 `GET vistack:video:bloom:ready` 返回 `"1"`）
- [ ] `PutVideoInfo` 写成功后失效详情 + 推荐列表缓存（验证：grep `Cache.Delete` 调用含两个 key）
- [ ] `DeleteVideo` 软删后失效详情 + 推荐列表缓存（验证：grep `Cache.Delete` 调用含两个 key）
- [ ] 视频创建（`CompleteVideoUpload` 与秒传分支）后 `VideoBloom.Add` 新 ID（验证：grep 两处 `VideoBloom.Add` 调用）

## 编译与测试

- [ ] `go build ./...` 全量编译通过
- [ ] `go test ./internal/core/cache/...` 全部通过（含单飞合并、空值缓存、删除、随机 TTL）
- [ ] `go vet ./internal/core/cache/...` 无告警

## 端到端场景

- [ ] **场景 1（详情缓存命中）**：请求 `GET /api/v1/videos/{id}/info` 两次 → 第二次命中 Redis（验证：`TTL vistack:video:info:{id}` 存在且为随机值；服务日志第二次无 DB 查询/回源日志）
- [ ] **场景 2（穿透拦截）**：请求一个不存在的 video id 两次 → 首次回源后写空值缓存，第二次命中空值不再打 DB（验证：`GET vistack:video:info:{fakeId}` 返回 `__NULL__` 哨兵；DB 查询日志仅一次）
- [ ] **场景 3（写失效）**：`PUT /api/v1/videos/{id}` 修改标题后立即 `GET .../info` → 返回新标题（验证：响应 title 为最新值，缓存 key 已重建）
- [ ] **场景 4（Redis 降级）**：停掉 Redis 后请求详情 → 仍返回正确数据（验证：接口 200 且数据来自 DB 兜底，不因 Redis 不可用而 5xx）
