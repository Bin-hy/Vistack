# Redis 缓存层 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/config/config.go` | 新增 `Cache` 配置结构体 |
| 新建 | `internal/core/cache/bloom.go` | 布隆过滤器（Redis bitmap + FNV 双哈希 + ready 标志） |
| 新建 | `internal/core/cache/cache.go` | `Cache` 组件：Cache-Aside + 三件套 + 锁 Lua |
| 新建 | `internal/core/cache/cache_test.go` | 单元测试（哈希/随机 TTL/单飞合并/空值/删除） |
| 新建 | `internal/core/cache.go` | `InitCache`：config → Options → 全局 `core.Cache`/`core.VideoBloom` |
| 新建 | `internal/api/v1/video_cache.go` | 业务 key 常量 + `BuildVideoBloom` |
| 修改 | `internal/api/v1/Video.go` | 5 个方法接入缓存/失效/布隆新增 |
| 修改 | `internal/role/api.go` | 启动装配：`InitCache` + 异步构建布隆 |
| 修改 | `conf/app.toml` / `conf/app.docker.toml` | 新增 `[cache]` 段 |

> 测试依赖：新增 `github.com/alicebob/miniredis/v2`（**仅测试**，用于在单测中模拟 Redis，不进入运行时依赖）。

---

## T1: 新增 Cache 配置结构体

**文件：** `internal/config/config.go`
**依赖：** 无

**步骤：**
1. 在 `Redis` 结构体之后新增 `Cache` 结构体，字段（`mapstructure` 与 toml 键一致）：
   - `Enabled bool` → `enabled`
   - `DefaultTTLMin int` → `default_ttl_min`
   - `DefaultTTLMax int` → `default_ttl_max`
   - `NullTTL int` → `null_ttl`
   - `LockTTL int` → `lock_ttl`
   - `LockWaitMS int` → `lock_wait_ms`
   - `RecommendTTL int` → `recommend_ttl`
   - `BloomEnabled bool` → `bloom_enabled`
   - `BloomBits int64` → `bloom_bits`
   - `BloomHashes int` → `bloom_hashes`
2. 所有时间单位用注释标注（`default_ttl_min/max`、`null_ttl`、`lock_ttl`、`recommend_ttl` 为秒；`lock_wait_ms` 为毫秒）。

**验证：** `go build ./internal/config/...` 编译通过。

---

## T2: 实现布隆过滤器 Bloom

**文件：** `internal/core/cache/bloom.go`
**依赖：** 无

**步骤：**
1. 声明包 `cache`，定义 `Bloom` 结构体（`client *redis.Client; key string; bits, hashes uint64`）与 `NewBloom(client, key, bits, hashes) *Bloom`。
2. 实现 FNV-1a 双哈希：`hash1(item) = fnv1a64(item)`、`hash2(item) = fnv1a64(item + "\x00")`；位置 `pos(i) = (hash1 + i*hash2) % bits`。用 `hash/fnv` 标准库实现，写成包内函数 `positions(item string) []uint64`。
3. 实现 `Build(ctx, items)`：`DEL key` + `DEL key+":ready"` → Pipeline 批量 `SETBIT` 所有位置 → `SET key+":ready" "1"`（`ready` 标志，防止空 bitmap 假阴性）。
4. 实现 `Add(ctx, item)`：Pipeline 批量 `SETBIT`。
5. 实现 `Exists(ctx, item)`：先 `GET key+":ready"`，非 "1" 返回 `(true, nil)`（未就绪降级为「可能存在」）；就绪则 Pipeline 批量 `GETBIT`，任一为 0 返回 `(false, nil)`，否则 `(true, nil)`。
6. 所有 Redis 写操作用 Pipeline 减少往返，错误向上返回并带 `zap` 日志（用包内注入的 logger，见 T3）。

**验证：** `go build ./internal/core/cache/...` 编译通过。

---

## T3: 实现 Cache 组件

**文件：** `internal/core/cache/cache.go`
**依赖：** T2

**步骤：**
1. 定义 `Options`（`KeyPrefix`、`DefaultTTL [2]time.Duration`、`NullTTL`、`LockTTL`、`LockWait`、`Bloom *BloomOptions`、`Logger *zap.Logger`）与 `BloomOptions`（`Key`、`Bits`、`Hashes`），与 plan.md 一致。
2. 定义 `Loader`、`Option`/`callOpts`、`WithTTL(min, max time.Duration) Option`、`nullSentinel` 常量、`loadResult{found bool; raw string}`。
3. 实现 `New(client, opts) *Cache`（存 client、singleflight.Group、opts；记录包内 `logger`）。
4. 实现 `readCache(ctx, key) (found bool, raw string, hit bool)`：`GET`，`redis.Nil` 或错误 → `hit=false`；等于 `nullSentinel` → `(false, "", true)`；否则 `(true, val, true)`。
5. 实现 `loadAndWrite(ctx, key, loader) (loadResult, error)`：调 loader → err 返回；`found=false` 写 `nullSentinel` 用 `NullTTL`；`found=true` `json.Marshal(value)` 写随机 TTL。
6. 实现 `directLoad(ctx, loader)`：兜底直查，返回结果但不写缓存。
7. 实现 `waitAndRead(ctx, key, loader, wait) (loadResult, error)`：每 50ms 轮询 `readCache`，命中返回；超时走 `directLoad`。
8. 实现锁释放 Lua：`if GET(key)==token then DEL(key)`；`acquireLock` 用 `SetNX(key+":lock", token, LockTTL)`，token 用 `github.com/google/uuid`。
9. 实现 `loadAndCache(ctx, key, loader)`：double-check → 抢锁 → 抢到 re-check + `loadAndWrite` + 释放锁；没抢到走 `waitAndRead`。
10. 实现 `GetOrLoad(ctx, key, dst, loader, opts...)`：`readCache` 命中即反序列化返回；否则若启用 Bloom 且 `Exists=false` 直接 `found=false`；否则 `singleflight.Do(key, loadAndCache)` 取 `loadResult`，反序列化 `raw` 到 `dst`。
11. 实现 `Delete(ctx, keys...)`：`DEL`。

**验证：** `go build ./internal/core/cache/...` 编译通过。

---

## T4: 单元测试

**文件：** `internal/core/cache/cache_test.go`
**依赖：** T2、T3（引入 `miniredis` 测试依赖）

**步骤：**
1. `go get github.com/alicebob/miniredis/v2`（仅测试）。
2. 写测试：`TestBloomHashPositions`（positions 稳定且 < bits）；`TestCacheNullValue`（loader 返回 found=false 后，第二次请求不再调 loader）；`TestCacheSingleflight`（并发 20 请求同一 key，loader 调用次数 = 1）；`TestCacheDelete`（Delete 后再次请求触发 loader）；`TestWithTTL`（覆盖生效）。

**验证：** `go test ./internal/core/cache/...` 全部通过。

---

## T5: 实现 InitCache 装配

**文件：** `internal/core/cache.go`
**依赖：** T1、T2、T3

**步骤：**
1. 声明全局 `var Cache *cache.Cache`、`var VideoBloom *cache.Bloom`。
2. 实现 `InitCache(cfg *config.AppConfig)`：
   - `if !cfg.Cache.Enabled { return }`；
   - 对零值套默认值：`DefaultTTLMin/Max` 默认 300/600、`NullTTL` 60、`LockTTL` 5、`LockWaitMS` 2000、`RecommendTTL` 300、`BloomBits` 10_000_000、`BloomHashes` 7；
   - 构造 `cache.Options`（`KeyPrefix="vistack"`、`Logger=Logger`），`Cache = cache.New(Redis, opts)`；
   - `if cfg.Cache.BloomEnabled`：`VideoBloom = cache.NewBloom(Redis, "vistack:video:bloom", bits, hashes)`。

**验证：** `go build ./internal/core/...` 编译通过。

---

## T6: 业务 key 与布隆构建

**文件：** `internal/api/v1/video_cache.go`
**依赖：** T5

**步骤：**
1. 定义常量 `cacheKeyVideoRecommend = "vistack:video:recommend"`；函数 `videoInfoCacheKey(id int64) string` 返回 `fmt.Sprintf("vistack:video:info:%d", id)`。
2. 实现 `BuildVideoBloom(ctx context.Context)`：`SELECT id FROM videos` 取全部 ID → 转 `[]string` → `core.VideoBloom.Build(ctx, ids)`；空表也调用 Build（置 ready）。

**验证：** `go build ./internal/api/...` 编译通过。

---

## T7: 改造 Video.go 接入缓存

**文件：** `internal/api/v1/Video.go`
**依赖：** T6

**步骤：**
1. `GetVideoInfo`：删除现有手写 Redis/SetNX 逻辑，改为 `core.Cache.GetOrLoad(ctx, videoInfoCacheKey(videoID), &resp, loader)`；`loader` 内 `Preload("CoverFile")` 查库、拼 `VideoInfoResponse`，`record not found` 返回 `(nil, false, nil)`。
2. `GetVideoRecommend`：改为 `GetOrLoad(ctx, cacheKeyVideoRecommend, &resp, loader, cache.WithTTL(recommendTTL, recommendTTL))`；`loader` 内原查询 + `resolveAuthors`。
3. `PutVideoInfo`：`Save` 成功后 `core.Cache.Delete(ctx, videoInfoCacheKey(videoID), cacheKeyVideoRecommend)`，替换现有单 key 删除。
4. `DeleteVideo`：软删成功后 `core.Cache.Delete(ctx, videoInfoCacheKey(videoID), cacheKeyVideoRecommend)`。
5. `CompleteVideoUpload` 与 `InitVideoUpload`（秒传分支）：`tx.Commit()` 成功后 `core.VideoBloom.Add(ctx, strconv.FormatInt(video.ID, 10))`（best-effort，失败仅记日志）。

**验证：** `go build ./internal/...` 编译通过；`grep -n "videoInfo:" internal/api/v1/Video.go` 不再出现手写 key 拼接。

---

## T8: 启动装配

**文件：** `internal/role/api.go`
**依赖：** T5、T6

**步骤：**
1. 在 `core.InitRedis(cfg)` 之后调用 `core.InitCache(cfg)`。
2. 在 `core.InitKafka(cfg)` 之后、`core.NewServer()` 之前，`go v1.BuildVideoBloom(context.Background())`（异步，失败仅记日志）。

**验证：** `go build ./...` 编译通过。

---

## T9: 配置文件新增 [cache] 段

**文件：** `conf/app.toml`、`conf/app.docker.toml`
**依赖：** 无（可与 T1 并行）

**步骤：**
1. 两个文件在 `[redis]` 段之后新增：
   ```toml
   [cache]
   enabled = true
   default_ttl_min = 300
   default_ttl_max = 600
   null_ttl = 60
   lock_ttl = 5
   lock_wait_ms = 2000
   recommend_ttl = 300
   bloom_enabled = true
   bloom_bits = 10000000
   bloom_hashes = 7
   ```
2. 若存在 `conf/app.local.toml` 且含 `[cache]`，保持其覆盖能力（不强制添加）。

**验证：** `go build ./...` 通过；目视确认两文件均含 `[cache]`。

---

## 执行顺序

```
T1 ──┬──> T5 ──> T6 ──> T7 ──> T8
     │
T2 ──┼──> T3 ──> T4 ──┘
     │
T9（可并行）
```

- T1、T2、T9 无依赖，可并行开始。
- T5 依赖 T1+T2+T3；T6 依赖 T5；T7 依赖 T6；T8 依赖 T5+T6。
- T4 依赖 T2+T3，可穿插进行。
