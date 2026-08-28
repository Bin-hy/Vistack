# 点赞/收藏/播放量计数 + 榜单 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/config/config.go` | 新增 `Social` 配置结构体 |
| 修改 | `internal/model/entity/video/video.go` | `Video` 加 3 个计数列 |
| 新建 | `internal/interaction/keys.go` | Redis key 构造 |
| 新建 | `internal/interaction/interaction.go` | Service + toggle/play Lua + 计数/状态 |
| 新建 | `internal/interaction/leaderboard.go` | 热门榜单查询 |
| 新建 | `internal/interaction/flusher.go` | 异步批量落库 + StartFlusher |
| 新建 | `internal/interaction/interaction_test.go` | 单元测试 |
| 新建 | `internal/api/v1/social.go` | 6 个 handler |
| 新建 | `internal/routers/api/v1/social.go` | SocialRouter 路由 |
| 修改 | `internal/routers/api/v1/enter.go` | 注册 SocialRouter |
| 修改 | `internal/role/api.go` | 构造 Service + 启动 flusher |
| 修改 | `conf/app.toml` / `conf/app.docker.toml` | 新增 `[social]` 段 |

> 测试复用 `miniredis`（已引入），无新增依赖。

---

## T1: 新增 Social 配置结构体

**文件：** `internal/config/config.go`
**依赖：** 无

**步骤：**
1. 在 `RateLimit` 结构体之后新增 `Social` 结构体（字段 `Enabled bool`→`enabled`、`FlushInterval int`→`flush_interval`(秒)、`FlushBatch int`→`flush_batch`、`LeaderboardSize int`→`leaderboard_size`）。

**验证：** `go build ./internal/config/...` 编译通过。

---

## T2: Video 实体加计数列

**文件：** `internal/model/entity/video/video.go`
**依赖：** 无

**步骤：**
1. 在 `Video` 结构体（`UpdatedAt` 之后、关联字段之前）新增：
   - `LikeCount int64`（`gorm:"default:0;column:like_count"`）
   - `FavoriteCount int64`（`column:favorite_count`）
   - `PlayCount int64`（`column:play_count`）

**验证：** `go build ./internal/model/...` 编译通过。

---

## T3: Redis key 构造

**文件：** `internal/interaction/keys.go`
**依赖：** 无

**步骤：**
1. 声明包 `interaction`，实现：
   - `likeKey(videoID) = "vistack:like:<id>"`
   - `favKey(videoID) = "vistack:fav:<id>"`
   - `playKey(videoID) = "vistack:play:<id>"`
   - `hotPlayKey() = "vistack:hot:play"`、`hotLikeKey() = "vistack:hot:like"`
   - `pendingKey() = "vistack:interaction:pending"`

**验证：** `go build ./internal/interaction/...` 编译通过。

---

## T4: Service + toggle/play 计数

**文件：** `internal/interaction/interaction.go`
**依赖：** T3

**步骤：**
1. 定义 `EventType`、`Event{ID, Type, VideoID, UserID, At}`、`Counts{LikeCount, FavoriteCount, PlayCount}`、`Options{FlushInterval, FlushBatch, LeaderboardSize, Logger}`。
2. 定义 `Service{rdb *redis.Client; db *gorm.DB; opts Options}` 与 `NewService(rdb, db, opts) *Service`。
3. 定义 toggle Lua 脚本：`SISMEMBER` 判断 → 已赞则 `SREM`+`ZINCRBY hot:like -1`+`RPUSH unlike 事件`，未赞则 `SADD`+`ZINCRBY +1`+`RPUSH like 事件`；返回 `{liked, SCARD}`。事件 JSON（含 snowflake ID）Go 侧预生成作 ARGV。
4. 定义 play Lua 脚本：`INCR play`+`ZINCRBY hot:play 1`+`RPUSH play 事件`；返回新计数。
5. 实现 `ToggleLike`/`ToggleFavorite`（同构）、`RecordPlay`（Run Lua，`Int64Slice`/`Int64` 解析）。
6. 实现 `Counts(videoIDs)`（Pipeline 批量 `SCARD`/`GET`，Redis 错误返回 err）、`IsLiked`/`IsFavorited`（`SISMEMBER`）。

**验证：** `go build ./internal/interaction/...` 编译通过。

---

## T5: 热门榜单

**文件：** `internal/interaction/leaderboard.go`
**依赖：** T3

**步骤：**
1. 实现 `Hot(ctx, sort string, limit int) ([]int64, error)`：`sort=="like"` 用 `hotLikeKey`，否则 `hotPlayKey`；`ZREVRANGE 0 limit-1` 返回 videoID（解析为 int64）；limit 零值默认 `opts.LeaderboardSize`。

**验证：** `go build ./internal/interaction/...` 编译通过。

---

## T6: 异步落库

**文件：** `internal/interaction/flusher.go`
**依赖：** T1、T3、T4、T2

**步骤：**
1. 实现 `popEvents(ctx, batch)`：`rdb.LPopCount(ctx, pendingKey(), batch)` 弹出，逐条 `json.Unmarshal` 成 `Event`（坏数据跳过记日志）。
2. 实现 `applyEvents(ctx, events)`：按类型分组——
   - `like`/`favorite`：`db.Clauses(clause.OnConflict{DoNothing: true}).Create(&mSocial.VideoLike{...})`（收藏用 VideoFavorite）；
   - `unlike`/`unfavorite`：`db.Where("video_id=? AND user_id=?", ...).Delete(...)`；
   - `play`：构造 `mSocial.VideoPlayLog{ID: event.ID, VideoID, PlayedAt}`，`OnConflict DoNothing` 批量 Create。
3. 实现 `syncCounts(ctx, videoIDs)`：对每个 videoID，Pipeline 读 `SCARD(like)`/`SCARD(fav)`/`GET(play)`，`UPDATE videos SET like_count=?, favorite_count=?, play_count=?`；Redis 读取失败跳过并告警（不丢事件）。
4. 实现 `FlushPending(ctx, batch)`：popEvents → applyEvents → syncCounts，返回处理条数。
5. 实现 `StartFlusher(ctx)`：goroutine + ticker（`FlushInterval`）循环调 `FlushPending`，记日志。

**验证：** `go build ./internal/interaction/...` 编译通过。

---

## T7: 单元测试

**文件：** `internal/interaction/interaction_test.go`
**依赖：** T4、T5、T6（用 miniredis）

**步骤：**
1. `TestToggleLike`：toggle 一次 `liked=true, count=1`；再 toggle `liked=false, count=0`。
2. `TestToggleFavorite`：同上。
3. `TestRecordPlay`：record 3 次，`count=3`。
4. `TestCountsAndStatus`：点赞+收藏+播放后，`Counts` 与 `IsLiked`/`IsFavorited` 正确。
5. `TestHotLeaderboard`：两个视频不同播放量，`Hot("play", 2)` 返回降序 videoID。
6. `TestPopEvents`：推 5 条事件，`popEvents(3)` 返回 3 条、队列剩 2 条。

**验证：** `go test ./internal/interaction/...` 全部通过。

---

## T8: API handlers

**文件：** `internal/api/v1/social.go`
**依赖：** T4、T5、T1

**步骤：**
1. 包级 `var interactionService *interaction.Service` + `SetInteractionService(svc)`。
2. 实现 `SocialApi`：
   - `LikeVideo`/`FavoriteVideo`（auth，`POST`）：`auth.GetUserID` → `ToggleLike/ToggleFavorite` → 返回 `{liked, like_count}` / `{favorited, favorite_count}`。
   - `PlayVideo`（公开，`POST`）：`RecordPlay` → `{play_count}`。
   - `GetVideoStats`（公开，`GET`）：`Counts([]int64{id})` → 三计数；Redis 失败回退 DB 冗余列（查 `videos`）。
   - `GetVideoInteraction`（auth，`GET`）：`IsLiked`/`IsFavorited` → `{liked, favorited}`。
   - `GetHotVideos`（公开，`GET`）：`Hot` → 用 Cache 层/DB 补全视频信息返回列表。
3. 所有 handler 对 `interactionService == nil` 返回 503（未启用）。

**验证：** `go build ./internal/api/...` 编译通过。

---

## T9: 路由注册

**文件：** `internal/routers/api/v1/social.go`（新建）、`enter.go`（修改）
**依赖：** T8

**步骤：**
1. 新建 `social.go`：`SocialRouter`，公开组（`GET /videos/:id/stats`、`POST /videos/:id/play`、`GET /videos/hot`）+ 受保护组（`POST /videos/:id/like`、`POST /videos/:id/favorite`、`GET /videos/:id/interaction`）。
2. `enter.go` 的 `RouterGroup` 增加 `SocialRouter` 字段。

**验证：** `go build ./internal/routers/...` 编译通过。

---

## T10: role/api.go 装配

**文件：** `internal/role/api.go`
**依赖：** T8、T6、T1

**步骤：**
1. 在 `core.InitCache` 之后构造 `interaction.NewService(core.Redis, core.DB, opts)`（opts 从 `cfg.Social` 读取，零值套默认：interval 5s、batch 200、leaderboard 50）。
2. `v1.SetInteractionService(svc)`。
3. 在 `go v1.BuildVideoBloom(...)` 之后 `svc.StartFlusher(ctx)`。

**验证：** `go build ./...` 编译通过。

---

## T11: 配置文件新增 [social] 段

**文件：** `conf/app.toml`、`conf/app.docker.toml`
**依赖：** 无（可与 T1 并行）

**步骤：**
1. 两个文件在 `[ratelimit]` 段之后新增：
   ```toml
   [social]
   enabled = true
   flush_interval = 5
   flush_batch = 200
   leaderboard_size = 50
   ```

**验证：** 目视确认两文件含 `[social]`；`go build ./...` 通过。

---

## 执行顺序

```
T1 ──┬───────────────> T6 ──> T7
T2 ──┘                   │
T3 ──> T4 ──> T5 ──> T8 ──> T9 ──> T10
T11（可并行）
```

- T1、T2、T3、T11 无依赖可并行；T4 依赖 T3；T5 依赖 T3；T6 依赖 T1+T2+T3+T4；T7 依赖 T4+T5+T6；T8 依赖 T1+T4+T5；T9 依赖 T8；T10 依赖 T6+T8。
