# 点播弹幕系统 Plan

## 架构概览

新增领域包 `internal/danmaku`（对标 B 站点播弹幕），复用既有 Kafka/Redis/限流：

- **发送链路**：`POST /videos/:id/danmaku`（auth + 限流）→ 敏感词（AC 自动机）→ 写 Redis ZSet（实时可见）→ 投 Kafka（异步持久化）。
- **拉取链路**：`GET /videos/:id/danmaku?start=&end=` 按时间范围分段拉取，走「本地 LRU → Redis ZSet → PostgreSQL」三级缓存，响应带 `Cache-Control`。
- **落库链路**：`danmaku` topic → worker 消费 → 写 PostgreSQL（按弹幕 ID 幂等）。

弹幕以 `time_offset` 为 score 存于 Redis ZSet，天然按时间排序，查询用 `ZRANGEBYSCORE`。

## 核心数据结构

### 弹幕实体（`internal/model/entity/danmaku/danmaku.go`）

```go
type Danmaku struct {
    ID         int64     `gorm:"primaryKey;column:id"`   // snowflake
    VideoID    int64     `gorm:"not null;index;column:video_id"`
    UserID     int64     `gorm:"not null;column:user_id"`
    Content    string    `gorm:"type:text;not null;column:content"`
    TimeOffset float64   `gorm:"column:time_offset"`     // 视频时间轴秒
    Color      string    `gorm:"size:20;column:color"`
    Mode       int       `gorm:"column:mode"`            // 0滚动 1顶部 2底部
    CreatedAt  time.Time `gorm:"column:created_at"`
}
```

### 敏感词实体（`internal/model/entity/danmaku/sensitive_word.go`）

```go
type SensitiveWord struct {
    ID        int64     `gorm:"primaryKey;column:id"`
    Word      string    `gorm:"size:100;not null;uniqueIndex;column:word"`
    CreatedAt time.Time `gorm:"column:created_at"`
}
func (SensitiveWord) TableName() string { return "sensitive_words" }
```

### danmaku.Service

```go
type Options struct {
    LocalCacheSize int           // 本地 LRU 容量（分段数）
    LocalCacheTTL  time.Duration // 本地缓存 TTL
    Logger         *zap.Logger
}

type Service struct {
    rdb    *redis.Client
    db     *gorm.DB
    filter *SensitiveFilter
    local  *LocalCache
    opts   Options
}

func NewService(rdb *redis.Client, db *gorm.DB, opts Options) *Service

// Send 发送弹幕：敏感词过滤 → 写 Redis ZSet + 投 Kafka。返回 ErrSensitive 表示命中敏感词。
func (s *Service) Send(ctx, videoID, userID int64, content string, timeOffset float64, color string, mode int) (*Danmaku, error)

// Fetch 按时间范围拉取弹幕（按 time_offset 升序）。
func (s *Service) Fetch(ctx, videoID int64, start, end float64) ([]Danmaku, error)

// 敏感词管理（增删后实时重建自动机）
func (s *Service) LoadSensitiveWords(ctx) error          // 启动时从 DB 加载并构建
func (s *Service) ListSensitiveWords(ctx) ([]SensitiveWord, error)
func (s *Service) AddSensitiveWord(ctx, word string) error    // 写 DB + 重建
func (s *Service) DeleteSensitiveWord(ctx, id int64) error    // 删 DB + 重建
```

### 敏感词过滤器（AC 自动机）

```go
type SensitiveFilter struct { root atomic.Pointer[acNode] }
func NewSensitiveFilter(words []string) *SensitiveFilter
func (f *SensitiveFilter) Reload(words []string)   // 重建 trie 并原子替换（并发安全）
func (f *SensitiveFilter) Contains(text string) bool
```

### 本地 LRU 缓存

```go
type LocalCache struct { /* map + container/list + TTL */ }
func (c *LocalCache) Get(key string) ([]Danmaku, bool)
func (c *LocalCache) Set(key string, v []Danmaku)
```

## 模块设计

### 1. 发送（danmaku.go）

- 敏感词命中 → 返回 `ErrSensitive`（不入库、不缓存）。
- 生成弹幕（snowflake ID）→ `ZADD vistack:danmaku:<videoID> <time_offset> <json>` → `SendKafkaMessage(topic=danmaku, key=videoID, value=json)`。
- Redis 写入为实时源；Kafka 投递失败不阻断返回（弹幕已实时可见，落库由 worker 兜底）。

### 2. 拉取（danmaku.go）

1. 本地 LRU 命中 → 返回。
2. `ZRANGEBYSCORE vistack:danmaku:<videoID> start end` 命中 → 回填本地缓存 → 返回。
3. Redis 空/错误 → 查 DB（`WHERE video_id=? AND time_offset>=? AND time_offset<? ORDER BY time_offset`）→ 回填 Redis + 本地缓存 → 返回。

### 3. 敏感词（sensitive.go）

- 标准 AC 自动机：Trie 插入词 → BFS 构建 fail 链 → 匹配时沿 fail 跳转，任一命中即 `Contains=true`。
- 关键词从 `sensitive_words` 表加载（启动时 `LoadSensitiveWords`）；增删后 `Reload` 重建 trie，用 `atomic.Pointer` 原子替换，发送侧无锁读取，并发安全。

### 4. 本地缓存（local_cache.go）

- `map[string]*list.Element` + `container/list`（LRU 顺序）+ 每项 TTL。
- `Get`：过期则删除返回 miss；命中移到队首。
- `Set`：写/更新并移到队首；超容量时淘汰队尾。

### 5. Kafka 消费者（`internal/core/message_queue/danmaku/worker.go`）

- `StartDanmakuWorker(ctx)`：`core.StartKafkaConsumer(ctx, danmaku topic, handler)`。
- handler：反序列化 → `INSERT ... ON CONFLICT (id) DO NOTHING`（幂等）。

### 6. API 层（`internal/api/v1/danmaku.go`）

| 接口 | 鉴权 | 行为 |
|------|------|------|
| `POST /videos/:id/danmaku` | 是 | 限流 → Send；命中敏感词返回 400 |
| `GET /videos/:id/danmaku?start=&end=` | 否 | Fetch，`Cache-Control: public, max-age=N` |
| `GET /admin/sensitive-words` | 是 | 查全部敏感词 |
| `POST /admin/sensitive-words` | 是 | 新增敏感词（写 DB + 重建自动机） |
| `DELETE /admin/sensitive-words/:id` | 是 | 删除敏感词（删 DB + 重建自动机） |

### 7. 装配

- `role/api.go`：构造 `danmaku.NewService(core.Redis, core.DB, opts)`，`v1.SetDanmakuService(svc)`。
- `role/worker.go`：`danmaku.StartDanmakuWorker(ctx)`。
- `migrations`：AutoMigrate 增加 `Danmaku`。
- `consts`：新增 `KafkaTopicDanmaku = "danmaku"`。

## 模块交互

```
发送: 客户端 → 限流 → 敏感词(AC) → ZADD Redis(实时) → Kafka → worker → PostgreSQL
拉取: 客户端 → 本地LRU → Redis ZRANGEBYSCORE → DB(回填) → 响应(Cache-Control)
```

## 文件组织

```
internal/danmaku/
├── danmaku.go        — Service + Send/Fetch + Redis ZSet
├── sensitive.go      — AC 自动机 + 词库
├── local_cache.go    — 进程内 LRU
├── keys.go           — Redis key
└── danmaku_test.go   — 单元测试

internal/model/entity/danmaku/
├── danmaku.go          — 弹幕实体
└── sensitive_word.go   — 敏感词实体

internal/core/message_queue/danmaku/worker.go — Kafka 消费者
internal/api/v1/danmaku.go               — 弹幕发送/拉取 handlers
internal/api/v1/sensitive_word.go        — 敏感词管理 handlers
internal/routers/api/v1/danmaku.go       — 弹幕 + 敏感词路由（+ enter.go）
internal/consts/consts.go                — KafkaTopicDanmaku
internal/config/config.go                — Danmaku 配置
internal/role/api.go                     — 构造 service
internal/role/worker.go                  — 启动 worker
migrations/migrate.go                    — AutoMigrate Danmaku
conf/app.toml / app.docker.toml          — [danmaku] 段
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 时间排序缓存 | Redis ZSet（score=time_offset） | 天然有序，`ZRANGEBYSCORE` 按范围查询 O(log n + m) |
| 实时可见 | API 同步写 Redis + Kafka 异步落库 | 与点赞系统同构；DB 仅承受异步写入 |
| 敏感词 | DB `sensitive_words` 表 + AC 自动机 + `atomic.Pointer` 动态重建 | 管理端维护、增删即生效、并发安全 |
| 多级缓存 | 本地 LRU → Redis → DB | 热门视频高频读取，本地缓存省 Redis 往返 |
| CDN | 响应 `Cache-Control` 短 TTL | 弹幕变化频繁，短 max-age 平衡新鲜度与缓存 |
| 落库幂等 | 弹幕 ID 为主键 `ON CONFLICT DO NOTHING` | Kafka at-least-once 重复消费不重复落库 |
| Kafka 投递失败 | 不阻断返回（Redis 已实时写） | 弹幕实时性优先，持久化靠 worker 重试 |
| 降级 | Redis 空/错误回退 DB | 不 5xx，保证可用性 |
