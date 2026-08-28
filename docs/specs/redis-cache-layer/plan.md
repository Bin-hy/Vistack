# Redis 缓存层 Plan

## 架构概览

新增通用缓存组件包 `internal/core/cache`，内含两个正交结构：

- **`Cache`**：Cache-Aside 读组件，内聚三件套——穿透（空值缓存）、击穿（singleflight + Redis 互斥锁）、雪崩（随机 TTL）。
- **`Bloom`**：基于 Redis bitmap 的布隆过滤器，用于在回源前拦截「一定不存在」的 key。

`internal/core` 新增 `InitCache`，从 `[cache]` 配置构造全局实例 `core.Cache` 与 `core.VideoBloom`。api 角色启动时初始化，并异步从 `videos` 表构建布隆过滤器。

两条读路径（`GetVideoInfo`、`GetVideoRecommend`）改为调用 `Cache.GetOrLoad`；写路径（`PutVideoInfo`、`DeleteVideo`）调用 `Cache.Delete` 失效；视频创建点向布隆过滤器 `Add` 新 ID。

## 核心数据结构

### cache.Options（构造参数，来自配置）

```go
type Options struct {
    KeyPrefix   string        // key 前缀，默认 "vistack"
    DefaultTTL  [2]time.Duration // 随机 TTL 范围 [min, max]
    NullTTL     time.Duration // 空值缓存 TTL
    LockTTL     time.Duration // 互斥锁 TTL
    LockWait    time.Duration // 未抢到锁时的最大等待
    Bloom       *BloomOptions // 非 nil 表示启用布隆过滤
    Logger      *zap.Logger   // 注入日志（nil 则不打印）
}

type BloomOptions struct {
    Key    string // bitmap key，如 "vistack:video:bloom"
    Bits   uint64 // bitmap 位数 m
    Hashes uint64 // hash 函数个数 k
}
```

### cache.Loader（回源函数）

```go
// 回源：value 为要缓存的对象（JSON 序列化）；found=false 表示源中不存在（穿透）。
type Loader func(ctx context.Context) (value any, found bool, err error)
```

### cache.Cache

```go
type Cache struct {
    client *redis.Client
    sf     singleflight.Group
    opts   Options
}

func New(client *redis.Client, opts Options) *Cache

// GetOrLoad 读缓存；未命中回源并写缓存。dst 为反序列化目标。
// opts 可选 WithTTL(min,max) 覆盖本次调用的随机 TTL 范围。
func (c *Cache) GetOrLoad(ctx context.Context, key string, dst any, loader Loader, opts ...Option) (found bool, err error)

// Delete 删除缓存 key（用于写路径失效）。
func (c *Cache) Delete(ctx context.Context, keys ...string) error

type Option func(*callOpts)
func WithTTL(min, max time.Duration) Option
```

### cache.Bloom

```go
type Bloom struct {
    client *redis.Client
    key    string
    bits   uint64
    hashes uint64
}

func NewBloom(client *redis.Client, key string, bits, hashes uint64) *Bloom
func (b *Bloom) Build(ctx context.Context, items []string) error // 清空并重建，完成后置 ready 标志
func (b *Bloom) Add(ctx context.Context, item string) error      // 新增单个元素
func (b *Bloom) Exists(ctx context.Context, item string) (bool, error) // false=一定不存在
```

### 内部类型（不对外）

- `const nullSentinel = "\x00NULL\x00"`：空值缓存哨兵。
- `type loadResult struct { found bool; raw string }`：singleflight 内部传递的结果。
- `type callOpts struct { ttlMin, ttlMax time.Duration }`：每调用覆盖项。

## 模块设计

### 1. `cache.Cache`（cache.go）

**职责**：实现「读缓存 → 未命中回源 → 写缓存」全流程，内聚三件套。

**对外接口**：`New`、`GetOrLoad`、`Delete`、`WithTTL`。

**依赖**：go-redis v9、`golang.org/x/sync/singleflight`、`zap`（经 Options 注入，不反向依赖 `core`）。

**读流程（`GetOrLoad`）**：

1. `readCache(key)`：`GET key`；命中空值哨兵 → `(found=false, hit=true)`；命中正常值 → 反序列化 → `(found=true, hit=true)`；未命中或 Redis 错误 → `hit=false`（Redis 错误降级为未命中，走回源）。
2. 若启用布隆且过滤器已就绪：`Bloom.Exists(key)` 为 `false` → 直接返回 `found=false`（不打 DB）。
3. `singleflight.Do(key, loadAndCache)`（进程内合并同 key 并发）：
   - `loadAndCache` 内先 double-check `readCache`（可能已被其他实例填上）。
   - 抢 Redis 互斥锁：`SetNX(key+":lock", token, LockTTL)`。
     - **抢到**：再次 double-check → 调 `loader` 回源 → 写缓存（`found=true` 用随机 TTL；`found=false` 写 `nullSentinel` 用 NullTTL）→ 释放锁（Lua `if GET==token then DEL`，防误删他人锁）。
     - **没抢到**（跨实例正在回源）：在 LockWait 内每 50ms 轮询 `readCache`，命中即返回；超时则 `directLoad` 兜底（直接调 loader 回源、不写缓存）。
4. 将 singleflight 结果反序列化到 `dst`，返回 `found`。

**写失效（`Delete`）**：直接 `DEL keys`。

### 2. `cache.Bloom`（bloom.go）

**职责**：Redis bitmap 布隆过滤器，拦截一定不存在的 key。

**哈希**：FNV-1a 双哈希——`h1 = fnv1a64(item)`、`h2 = fnv1a64(item + "\x00")`，第 i 个位置 `(h1 + i*h2) % bits`（无第三方依赖）。

**就绪标志**：额外维护 `<key>:ready` 标志。`Build` 完成后置 1；`Exists` 在 ready 未置时返回 `true`（降级，不误拦截）。避免「空 bitmap 把一切判为不存在」的假阴性。

**构建**：`Build` 先 `DEL key` + `DEL ready`，用 Pipeline 批量 `SETBIT`，完成后 `SET ready 1`。`Add`/`Exists` 用 Pipeline 批量 `SETBIT`/`GETBIT`，单次往返。

**假阴性防护**：新视频创建时同步 `Add(id)`；删除靠未来定期重建兜底（删除后遗留是假阳性，安全）。

### 3. `core.InitCache`（internal/core/cache.go）

**职责**：从 `cfg.Cache` 读参数、对零值套用默认值、构造并保存全局实例 `core.Cache`、`core.VideoBloom`，注入 `core.Logger`。

### 4. api 层接入（internal/api/v1）

- 新增 `video_cache.go`：业务 key 常量/构造函数 + `BuildVideoBloom(ctx)`（`SELECT id FROM videos` → `Bloom.Build`）。
- 改造 `Video.go`：
  - `GetVideoInfo` → `GetOrLoad(infoKey, &resp, loader)`；
  - `GetVideoRecommend` → `GetOrLoad(recommendKey, &resp, loader, WithTTL(recommendTTL, recommendTTL))`；
  - `PutVideoInfo` / `DeleteVideo` → 保存后 `Cache.Delete(infoKey, recommendKey)`；
  - `CompleteVideoUpload` / `InitVideoUpload`（秒传分支）→ 创建视频后 `VideoBloom.Add(id)`。

### 5. `role.RunAPI`（internal/role/api.go）

**职责**：`core.InitRedis` 后调用 `core.InitCache`；随后 goroutine 异步调用 `v1.BuildVideoBloom(ctx)`。

## 模块交互

### 读流程（Cache-Aside）

```
请求 → GetOrLoad
        ├─ readCache 命中 ────────────────────────────→ 返回
        ├─ Bloom.Exists=false（一定不存在）──────────→ 返回 not found
        └─ 未命中 → singleflight(loadAndCache)
                      ├─ 抢到锁 → loader(DB+RPC) → 写缓存 → 释放锁 → 返回
                      └─ 未抢到 → 轮询缓存 → 命中返回 / 超时直查 DB 兜底
```

### 写失效

```
PutVideoInfo / DeleteVideo → DB 写成功 → Cache.Delete(infoKey, recommendKey)
```

### 布隆构建/新增

```
启动: RunAPI → goroutine BuildVideoBloom → SELECT id FROM videos → Bloom.Build
新增: CompleteVideoUpload / InitVideoUpload → Video 创建成功 → Bloom.Add(id)
```

## 文件组织

```
internal/core/cache/
├── cache.go        — Options/Cache/Loader/New/GetOrLoad/Delete/WithTTL + 读流程 + 锁释放 Lua + nullSentinel
├── bloom.go        — Bloom/NewBloom/Build/Add/Exists + FNV 双哈希 + ready 标志
└── cache_test.go   — 单元测试：穿透（空值缓存）、击穿（singleflight 合并 + 锁）、随机 TTL

internal/core/
└── cache.go        — InitCache：config → cache.Options（默认值兜底）→ 全局 core.Cache / core.VideoBloom

internal/config/
└── config.go       — 新增 Cache 配置结构体

internal/api/v1/
├── video_cache.go  — 业务 key 常量 + BuildVideoBloom
└── Video.go        — 改造 5 个方法接入缓存/失效/布隆新增

internal/role/
└── api.go          — 启动装配：core.InitCache + 异步 BuildVideoBloom

conf/
├── app.toml        — 新增 [cache] 段
└── app.docker.toml — 新增 [cache] 段
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 缓存值序列化 | JSON 字符串 | 与现有 `VideoInfoResponse` 直接兼容，`GetOrLoad` 用 `any` + `json.Marshal` 统一处理 |
| 击穿-进程内 | `golang.org/x/sync/singleflight` | 已在依赖中，零新增依赖；单实例内合并并发回源 |
| 击穿-跨实例 | Redis `SetNX` 互斥锁 + Lua owner-check 释放 | 项目可水平扩容，singleflight 只覆盖单进程，必须叠加分布式锁 |
| 未抢到锁兜底 | 轮询缓存 LockWait 后直查 DB 不写缓存 | 避免锁竞争下重复写放大；由持锁者写缓存，超时兜底保证可用性 |
| 穿透-空值缓存 | `nullSentinel` 哨兵 + 短 TTL | 简单有效，直接解决不存在 ID 穿透 |
| 穿透-布隆 | Redis bitmap + FNV 双哈希，自实现 | 不引入第三方库；ready 标志避免空 filter 假阴性 |
| 布隆假阴性 | 启动全量 Build + 创建时 Add | 布隆不能删元素，删除靠定期重建（未来）；新增必须 Add 否则误拦真实视频 |
| 雪崩 | 随机 TTL（复用 `timeutil.RandomRangeExpire`） | 已有工具，作为组件内置默认策略 |
| 一致性 | 更新后删缓存（最终一致） | spec 已定，不实现延迟双删 |
| 日志 | `zap.Logger` 经 Options 注入 | 避免 cache 包反向 import core 造成循环依赖 |
| 配置 | 新增 `[cache]` 段 + 代码默认值兜底 | 与现有 Viper 机制一致，零值用默认值避免配置缺失报错 |
| 推荐列表 TTL | `WithTTL` 覆盖为独立 recommend_ttl | 推荐列表含作者信息，需更短时效 |
