# 点播弹幕系统 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/model/entity/danmaku/danmaku.go` | 弹幕实体 |
| 新建 | `internal/model/entity/danmaku/sensitive_word.go` | 敏感词实体 |
| 修改 | `internal/consts/consts.go` | 新增 `KafkaTopicDanmaku` |
| 修改 | `internal/config/config.go` | 新增 `Danmaku` 配置结构体 |
| 新建 | `internal/danmaku/keys.go` | Redis key |
| 新建 | `internal/danmaku/sensitive.go` | AC 自动机 |
| 新建 | `internal/danmaku/local_cache.go` | 进程内 LRU |
| 新建 | `internal/danmaku/danmaku.go` | Service + Send/Fetch + 敏感词 CRUD |
| 新建 | `internal/danmaku/danmaku_test.go` | 单元测试 |
| 新建 | `internal/core/message_queue/danmaku/worker.go` | Kafka 消费者 |
| 新建 | `internal/api/v1/danmaku.go` | 弹幕发送/拉取 handlers |
| 新建 | `internal/api/v1/sensitive_word.go` | 敏感词管理 handlers |
| 新建 | `internal/routers/api/v1/danmaku.go` | 弹幕 + 敏感词路由 |
| 修改 | `internal/routers/api/v1/enter.go` | 注册 DanmakuRouter |
| 修改 | `internal/role/api.go` | 构造 service |
| 修改 | `internal/role/worker.go` | 启动 danmaku worker |
| 修改 | `migrations/migrate.go` | AutoMigrate Danmaku + SensitiveWord |
| 修改 | `conf/app.toml` / `conf/app.docker.toml` | 新增 `[danmaku]` 段 |

> 测试复用 `miniredis`（已引入），无新增依赖。

---

## T1: 实体 + 常量 + 配置

**文件：** `internal/model/entity/danmaku/danmaku.go`、`sensitive_word.go`、`internal/consts/consts.go`、`internal/config/config.go`
**依赖：** 无

**步骤：**
1. `danmaku.go`：`Danmaku` 实体（ID snowflake、VideoID 带索引、UserID、Content、TimeOffset float64、Color、Mode、CreatedAt），`TableName()="danmakus"`，`BeforeCreate` 生成 ID。
2. `sensitive_word.go`：`SensitiveWord`（ID、Word uniqueIndex、CreatedAt），`TableName()="sensitive_words"`，`BeforeCreate` 生成 ID。
3. `consts.go`：`KafkaTopicDanmaku KafkaTopic = "danmaku"`。
4. `config.go`：`Danmaku` 结构体（`Enabled`、`LocalCacheSize`、`LocalCacheTTL`(秒)、`CacheControlMaxAge`(秒)）。

**验证：** `go build ./internal/model/... ./internal/config/... ./internal/consts/...` 编译通过。

---

## T2: AC 自动机

**文件：** `internal/danmaku/sensitive.go`
**依赖：** 无

**步骤：**
1. 定义 `acNode{children map[rune]*acNode; fail *acNode; isEnd bool}`。
2. `NewSensitiveFilter(words []string)`：插入词构建 Trie。
3. BFS 构建 fail 链（根的子节点 fail 指向根；其余按父 fail 的 children 找）。
4. `Contains(text string) bool`：遍历文本，沿 fail 跳转，命中 `isEnd` 返回 true。
5. `Reload(words []string)`：重建 trie 后用 `atomic.Pointer[acNode]` 原子替换根（并发安全，无锁读）。

**验证：** `go build ./internal/danmaku/...` 编译通过。

---

## T3: 本地 LRU 缓存

**文件：** `internal/danmaku/local_cache.go`
**依赖：** 无

**步骤：**
1. `LocalCache`：`map[string]*list.Element` + `container/list` + `sync.Mutex` + `size` + `ttl`。
2. `NewLocalCache(size int, ttl time.Duration)`。
3. `Get(key) ([]Danmaku, bool)`：过期删除返回 miss；命中移到队首返回。
4. `Set(key, v)`：写入/更新移到队首；超 `size` 淘汰队尾。

**验证：** `go build ./internal/danmaku/...` 编译通过。

---

## T4: Service 核心

**文件：** `internal/danmaku/keys.go`、`internal/danmaku/danmaku.go`
**依赖：** T1、T2、T3

**步骤：**
1. `keys.go`：`danmakuKey(videoID) = "vistack:danmaku:<id>"`。
2. `danmaku.go`：`Service{rdb, db, filter *SensitiveFilter, local *LocalCache, opts}` + `NewService`。
3. `Send(ctx, videoID, userID, content, timeOffset, color, mode)`：`filter.Contains(content)` 命中返回 `ErrSensitive`；否则生成弹幕 → `ZADD`（score=timeOffset, member=json）→ `core.SendKafkaMessage(danmaku, videoID, json)` → 返回。
4. `Fetch(ctx, videoID, start, end)`：本地缓存 → `ZRANGEBYSCORE` → DB 回填（`WHERE video_id AND time_offset>=start AND time_offset<end ORDER BY time_offset`）。
5. `LoadSensitiveWords`（启动从 DB 加载 → filter.Reload）、`ListSensitiveWords`、`AddSensitiveWord`（写 DB + Reload）、`DeleteSensitiveWord`（删 DB + Reload）。

**验证：** `go build ./internal/danmaku/...` 编译通过。

---

## T5: Kafka 消费者

**文件：** `internal/core/message_queue/danmaku/worker.go`
**依赖：** T1、T2（consts）

**步骤：**
1. `StartDanmakuWorker(ctx)`：`core.StartKafkaConsumer(ctx, string(consts.KafkaTopicDanmaku), handler)`。
2. handler：反序列化 `Danmaku` → `db.Clauses(clause.OnConflict{DoNothing: true}).Create(&d)`（幂等）。

**验证：** `go build ./internal/core/message_queue/danmaku/...` 编译通过。

---

## T6: 单元测试

**文件：** `internal/danmaku/danmaku_test.go`
**依赖：** T2、T3、T4

**步骤：**
1. `TestACAutomaton`：`Contains` 命中敏感词/未命中；`Reload` 后新词立即生效。
2. `TestLocalCache`：Set/Get 命中、过期 miss、LRU 淘汰。
3. `TestSendSensitive`：敏感词弹幕返回 `ErrSensitive`（用 miniredis + nil db）。
4. `TestFetchRange`：写入不同 time_offset 弹幕，`Fetch` 只返回范围内且升序（miniredis）。

**验证：** `go test ./internal/danmaku/...` 全部通过。

---

## T7: API handlers

**文件：** `internal/api/v1/danmaku.go`、`internal/api/v1/sensitive_word.go`
**依赖：** T4

**步骤：**
1. 包级 `danmakuService` + `SetDanmakuService`。
2. `danmaku.go`：`SendDanmaku`（auth，解析 body → Send，`ErrSensitive` 返回 400）、`GetDanmaku`（公开，`start/end` 查询 → Fetch，设 `Cache-Control: public, max-age=N`）。
3. `sensitive_word.go`：`ListSensitiveWords`、`AddSensitiveWord`（body `{word}`）、`DeleteSensitiveWord`（`:id`），均 auth。

**验证：** `go build ./internal/api/...` 编译通过。

---

## T8: 路由 + 装配 + 迁移 + 配置

**文件：** `internal/routers/api/v1/danmaku.go`、`enter.go`、`internal/role/api.go`、`internal/role/worker.go`、`migrations/migrate.go`、`conf/app.toml`、`conf/app.docker.toml`
**依赖：** T5、T7

**步骤：**
1. 路由：`DanmakuRouter`，公开组（`GET /videos/:id/danmaku`）+ 受保护组（`POST /videos/:id/danmaku`、`GET/POST /admin/sensitive-words`、`DELETE /admin/sensitive-words/:id`）；`enter.go` 注册。
2. `role/api.go`：构造 `danmaku.NewService(core.Redis, core.DB, opts)`，`SetDanmakuService`，`LoadSensitiveWords`。
3. `role/worker.go`：`danmaku.StartDanmakuWorker(ctx)`。
4. `migrations`：AutoMigrate 加 `Danmaku`、`SensitiveWord`。
5. conf 两文件加 `[danmaku]` 段（enabled/local_cache_size/local_cache_ttl/cache_control_max_age）。

**验证：** `go build ./...` 编译通过。

---

## 执行顺序

```
T1 ──┬──> T4 ──> T6
T2 ──┤       │
T3 ──┘       ├──> T7 ──> T8
T5（依赖 T1 的 consts）──┘
```

- T1、T2、T3 无依赖可并行；T4 依赖 T1+T2+T3；T5 依赖 T1(consts)；T6 依赖 T2+T3+T4；T7 依赖 T4；T8 依赖 T5+T7。
