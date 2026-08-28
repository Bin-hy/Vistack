# 高并发点赞/收藏/播放量计数 + 热门榜单 Plan

## 架构概览

新增领域包 `internal/interaction`（复用现有 `internal/model/entity/social` 的 GORM 实体），职责：

- **计数与去重**：点赞/收藏用 Redis Set（去重 + `SCARD` 计数 + `SISMEMBER` 状态），播放量用 Redis `INCR`。
- **事件队列**：每次交互向 Redis List 推送一条事件（供异步落库）。
- **异步落库**：后台 goroutine 定时批量 drain 队列，更新 `videos` 冗余计数列 + 写/删明细表，幂等。
- **热门榜单**：Redis ZSet 按播放量/点赞数维护排名。

API 层新增 `internal/api/v1/social.go` 暴露交互接口；`role/api.go` 启动 flusher goroutine。

## 核心数据结构

### 交互事件（Redis 队列元素，JSON 序列化）

```go
type EventType string

const (
    EventLike      EventType = "like"
    EventUnlike    EventType = "unlike"
    EventFavorite  EventType = "favorite"
    EventUnfavorite EventType = "unfavorite"
    EventPlay      EventType = "play"
)

type Event struct {
    ID      int64     `json:"id"`       // snowflake，幂等去重
    Type    EventType `json:"type"`
    VideoID int64     `json:"video_id"`
    UserID  int64     `json:"user_id"`
    At      int64     `json:"at"`       // unix 秒
}
```

### 视频实体扩展（`internal/model/entity/video/video.go`）

`Video` 新增冗余计数列：`LikeCount int64`、`FavoriteCount int64`、`PlayCount int64`（`column:like_count` 等）。

### interaction.Service

```go
type Service struct {
    rdb *redis.Client
}

func NewService(rdb *redis.Client, opts Options) *Service

// 用户交互（Lua 原子：toggle + 推事件）
func (s *Service) ToggleLike(ctx, videoID, userID int64) (liked bool, count int64, err error)
func (s *Service) ToggleFavorite(ctx, videoID, userID int64) (favorited bool, count int64, err error)
func (s *Service) RecordPlay(ctx, videoID int64) (count int64, err error)

// 计数与状态（读）
func (s *Service) Counts(ctx, videoIDs []int64) (map[int64]Counts, error)
func (s *Service) IsLiked(ctx, videoID, userID int64) (bool, error)
func (s *Service) IsFavorited(ctx, videoID, userID int64) (bool, error)

// 榜单
func (s *Service) Hot(ctx, sort string, limit int) ([]int64, error)

// 落库（flusher 调用）
func (s *Service) FlushPending(ctx, batch int) (int, error)

type Counts struct {
    LikeCount     int64
    FavoriteCount int64
    PlayCount     int64
}
```

## Redis Key 设计

| Key | 类型 | 说明 |
|-----|------|------|
| `vistack:like:<videoID>` | Set | 点赞用户集合（SCARD=计数） |
| `vistack:fav:<videoID>` | Set | 收藏用户集合 |
| `vistack:play:<videoID>` | string | 播放计数（INCR） |
| `vistack:hot:play` | ZSet | 播放榜（member=videoID, score=播放量） |
| `vistack:hot:like` | ZSet | 点赞榜 |
| `vistack:interaction:pending` | List | 待落库事件队列 |

## 模块设计

### 1. 用户交互（interaction.go）

- `ToggleLike`：Lua 原子——`SISMEMBER` 判断当前状态，已赞则 `SREM` + 推 `unlike` 事件，未赞则 `SADD` + 推 `like` 事件；同时 `ZINCRBY hot:like ±1`；返回最新状态与 `SCARD`。`ToggleFavorite` 同构。
- `RecordPlay`：Lua 原子——`INCR play` + `ZINCRBY hot:play 1` + 推 `play` 事件；返回新计数。
- 事件 JSON（含 snowflake `ID`）在 Go 侧预生成，作为 ARGV 传入 Lua，保证「状态变更 + 入队」原子。
- `Counts` / `IsLiked` / `IsFavorited`：Pipeline 批量 `SCARD`/`GET`/`SISMEMBER`。

### 2. 热门榜单（leaderboard.go）

- 榜单 ZSet 在交互时同步 `ZINCRBY`（play +1 / like ±1）。
- `Hot(sort, limit)`：按 sort 选 ZSet，`ZREVRANGE 0 limit-1` 返回 videoID 列表；调用方用 Cache 层/DB 补全视频信息。

### 3. 异步落库（flusher.go）

- 后台 goroutine 每 `flush_interval` 秒 `LPOP`（或 `LPopCount`）批量取出最多 `flush_batch` 条事件。
- 按类型应用（幂等）：
  - `like` → `INSERT video_likes ON CONFLICT DO NOTHING`；`unlike` → `DELETE video_likes`；
  - `favorite`/`unfavorite` → 同构 `video_favorites`；
  - `play` → `INSERT video_play_logs`（以事件 `ID` 为 `id`，`ON CONFLICT DO NOTHING`）。
- 对涉及的每个 `video_id`，`UPDATE videos SET like_count = SCARD(...), favorite_count = SCARD(...), play_count = GET(...)`（以 Redis 为准，幂等；Redis 不可用则跳过并告警，不丢事件）。
- 失败不丢事件：先应用 DB 写成功，再提交队列删除（`LPOP` 已弹出，失败重试走幂等 INSERT/DELETE）。

### 4. API 层（internal/api/v1/social.go）

| 接口 | 鉴权 | 行为 |
|------|------|------|
| `POST /videos/:id/like` | 是 | toggle 点赞，返回 `{liked, like_count}` |
| `POST /videos/:id/favorite` | 是 | toggle 收藏，返回 `{favorited, favorite_count}` |
| `POST /videos/:id/play` | 否 | 播放上报 +1，返回 `{play_count}` |
| `GET /videos/:id/stats` | 否 | 返回三计数 `{like_count, favorite_count, play_count}` |
| `GET /videos/:id/interaction` | 是 | 返回 `{liked, favorited}` |
| `GET /videos/hot?sort=play&limit=20` | 否 | 热门榜单（sort=play/like） |

### 5. 装配（role/api.go + routers）

- `role/api.go`：`core.InitRedis` 后构造 `interaction.NewService(core.Redis, ...)`，注入 API 层；启动 flusher goroutine（`service.StartFlusher(ctx)`）。
- `routers`：新增 `SocialRouter`，公开路由（play/stats/hot）+ 受保护路由（like/favorite/interaction，挂 AuthApiGroup + RateLimit 之后）。

## 模块交互

```
点赞请求 → RateLimit(限流) → AuthMiddleware(鉴权)
  → interaction.ToggleLike → Lua[ SISMEMBER→SADD/SREM + 推事件 + ZINCRBY榜单 ]
  → 返回 {liked, count}

后台 flusher（每 N 秒）：
  LPOP 事件批 → 分组 → INSERT/DELETE 明细表 + UPDATE videos 冗余列(SCARD/GET)
```

## 文件组织

```
internal/interaction/
├── interaction.go     — Service + ToggleLike/ToggleFavorite/RecordPlay/Counts/IsLiked/IsFavorited
├── keys.go            — Redis key 构造
├── leaderboard.go     — Hot 榜单查询
├── flusher.go         — 异步批量落库 + StartFlusher
└── interaction_test.go — 单元测试

internal/model/entity/video/video.go  — Video 加 3 个计数列

internal/api/v1/
└── social.go          — 6 个 handler

internal/routers/api/v1/
├── enter.go           — 注册 SocialRouter
└── social.go          — SocialRouter 路由

internal/role/api.go   — 构造 Service + 启动 flusher

internal/config/config.go — 新增 Social 配置结构体

conf/app.toml / app.docker.toml — 新增 [social] 段
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 点赞/收藏去重 | Redis Set + `SCARD`/`SISMEMBER` | 天然去重、O(1) 计数与状态判断 |
| toggle 原子性 | 单 Lua 脚本「判断→SADD/SREM→推事件→ZINCRBY」 | 避免 check-then-act 竞态，状态与入队一致 |
| 播放量 | Redis `INCR` + 榜单 `ZINCRBY` | 计数维度，O(1) 递增 |
| 事件幂等 | 事件带 snowflake `ID`，明细表 `ON CONFLICT DO NOTHING` | 落库重试不重复 |
| 计数落库 | 以 Redis `SCARD`/`GET` 为权威回写冗余列 | 幂等、避免 delta 漂移 |
| 队列消费 | `LPOP` 批量 + DB 写成功后确认 | 简单；明细行为 best-effort（计数经 SCARD 始终正确） |
| 榜单 | 两个 ZSet（play/like），交互时 `ZINCRBY` | 实时榜，`ZREVRANGE` O(log n) |
| 降级 | 计数读 Redis 失败回退 DB 冗余列 | F8 要求，不 5xx |
| 播放上报公开 | 匿名可上报 | 播放量=总播放次数（刷量防护属后续 IP 限流） |
