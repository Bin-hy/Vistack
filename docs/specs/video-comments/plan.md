# 视频评论系统 Plan

> 语言相关：Go（后端）+ Vue 3（前端）。对应已批准的 `docs/specs/video-comments/spec.md`。

## 架构概览

新增一个领域包 `internal/comment`，与现有 `internal/interaction`、`internal/danmaku` 并列，封装评论的核心逻辑；复用 `danmaku.SensitiveFilter`（AC 敏感词）、`files` 引用计数、Kafka 消费者、`authclient.UserClient`（作者信息）、`core.Cache`（列表缓存）。

后端新增/改动分六块：

1. **模型层**：扩展 `VideoComment` 实体（加 `root_id`/`reply_to_id`/`attachments`/`status`/计数/软删除），新增 `CommentLike` 实体。
2. **服务层**：`internal/comment` 包，封装「发表/列表/展开回复/点赞/删除/计数/审核」。
3. **API 层**：`CommentApi`（handler）+ 图片上传 `CommentImageUpload`，通过 `SetCommentService` 注入。
4. **路由层**：公开读路由 + 受保护写路由。
5. **审核链路**：Kafka topic `comment_moderation` + 可插拔 `Moderator` 接口 + worker。
6. **装配**：`role/api.go` 装配 service，`role/worker.go` 启动审核 worker。

前端在 `VideoPlayer` 视图新增评论组件树，复用现有 `button/input/icon/toast` 等 UI 组件。

---

## 核心数据结构

### VideoComment（扩展 `internal/model/entity/social/comment.go`）

```go
type CommentStatus string

const (
    CommentStatusVisible CommentStatus = "visible" // 可见
    CommentStatusPending CommentStatus = "pending" // 含图待审
    CommentStatusHidden  CommentStatus = "hidden"  // 审核拒绝
    CommentStatusDeleted CommentStatus = "deleted" // 软删除
)

type CommentAttachment struct {
    Type   string `json:"type"`            // "image"（图片）| "sticker"（表情包 GIF/PNG）
    FileID int64  `json:"file_id,string"`  // 引用 files.id
}

type VideoComment struct {
    ID          int64          `gorm:"primaryKey;column:id" json:"id"`
    VideoID     int64          `gorm:"not null;column:video_id" json:"video_id"`
    UserID      int64          `gorm:"not null;column:user_id" json:"user_id"`
    RootID      *int64         `gorm:"column:root_id" json:"root_id,omitempty"`       // 根评论 id，nil=一级评论
    ParentID    *int64         `gorm:"column:parent_id" json:"parent_id,omitempty"`   // 直接父评论 id
    ReplyToID   *int64         `gorm:"column:reply_to_id" json:"reply_to_id,omitempty"` // 被精确回复评论 id
    ReplyToUID  *int64         `gorm:"column:reply_to_uid" json:"reply_to_uid,omitempty"` // 被回复用户 id（冗余）
    Content     string         `gorm:"type:text;not null;column:content" json:"content"`
    Attachments string         `gorm:"type:jsonb;column:attachments" json:"attachments,omitempty"` // JSON: []CommentAttachment
    Status      CommentStatus  `gorm:"size:20;not null;default:visible;column:status" json:"status"`
    LikeCount   int64          `gorm:"default:0;column:like_count" json:"like_count"`
    ReplyCount  int64          `gorm:"default:0;column:reply_count" json:"reply_count"`
    CreatedAt   time.Time      `gorm:"column:created_at" json:"created_at"`
    UpdatedAt   time.Time      `gorm:"column:updated_at" json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index;column:deleted_at" json:"-"`

    Video video.Video `gorm:"foreignKey:VideoID;constraint:false" json:"-"`
    User  user.User   `gorm:"foreignKey:UserID;constraint:false" json:"-"`
}
```

说明：`root_id` 由后端在创建时根据 `parent_id` 推导（父为空→自身为根；父非空→取父的 root，若父是根则取父 id），前端不传、防篡改。`ReplyToUID` 由后端查 `reply_to_id` 的作者填充。

### CommentLike（新增 `internal/model/entity/social/comment_like.go`）

```go
type CommentLike struct {
    CommentID int64     `gorm:"primaryKey;column:comment_id" json:"comment_id"`
    UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}
func (CommentLike) TableName() string { return "comment_likes" }
```

### Service（`internal/comment` 包）

```go
type Options struct {
    FlushInterval time.Duration // 计数 flush 周期，默认 5s
    FlushBatch    int           // 每批事件数，默认 200
    Logger        *zap.Logger
}

type CreateInput struct {
    VideoID     int64
    UserID      int64
    Content     string
    ParentID    *int64               // 回复时传，一级评论为 nil
    ReplyToID   *int64               // 精确 @ 对象（可为 nil）
    Attachments []CommentAttachment  // 有序，image+sticker 合计 ≤9
}

type Service struct {
    rdb    *redis.Client
    db     *gorm.DB
    filter *danmaku.SensitiveFilter
    opts   Options
}

func NewService(rdb *redis.Client, db *gorm.DB, opts Options) *Service

// 写
func (s *Service) Create(ctx context.Context, in CreateInput) (*entity.VideoComment, error)
func (s *Service) ToggleLike(ctx context.Context, commentID, userID int64) (bool, int64, error)
func (s *Service) Delete(ctx context.Context, commentID, userID int64) error

// 读
func (s *Service) List(ctx context.Context, videoID, cursor int64, limit int) (roots []entity.VideoComment, next int64, err error)
func (s *Service) ListReplies(ctx context.Context, rootID, cursor int64, limit int) ([]entity.VideoComment, int64, error)
func (s *Service) CommentCount(ctx context.Context, videoID int64) (int64, error)
func (s *Service) IsLiked(ctx context.Context, commentID, userID int64) (bool, error)

// 审核
func (s *Service) EnqueueModeration(ctx context.Context, commentID int64) error
func (s *Service) ApplyModerationResult(ctx context.Context, commentID int64, pass bool) error

// 生命周期
func (s *Service) LoadSensitiveWords(ctx context.Context) error
func (s *Service) StartFlusher(ctx context.Context)
```

### Moderator（可插拔图片审核器，`internal/comment` 包）

```go
type Moderator interface {
    // Review 对评论附件的文件列表做内容安全审核，true=通过。
    Review(ctx context.Context, files []mFile.File) (bool, error)
}

// PassthroughModerator 桩实现：未接入第三方时默认通过。
type PassthroughModerator struct{}
func (PassthroughModerator) Review(ctx context.Context, files []mFile.File) (bool, error) { return true, nil }
```

---

## 模块设计

### 模块 A：`internal/model/entity/social`（模型）
- **职责**：`VideoComment` 扩展字段、`CommentLike` 新实体。
- **依赖**：`pkg/snowflake`、`gorm`。
- 迁移：`migrations/migrate.go` 增加 `&mSocial.CommentLike{}`；`VideoComment` 新增列与 `idx_comments_video_root (video_id, root_id, id)` 索引由 AutoMigrate + 显式 `CREATE INDEX IF NOT EXISTS` 完成。

### 模块 B：`internal/comment`（服务）
- **职责**：评论领域逻辑、计数、审核编排、敏感词。
- **子文件**：
  - `comment.go` — `Service`、`Create/List/ListReplies/Delete/ToggleLike`
  - `keys.go` — Redis key 函数
  - `counter.go` — 点赞/回复/视频计数 + `StartFlusher`（复用 interaction 的 Lua+flusher 思路）
  - `moderation.go` — `Moderator`/`PassthroughModerator`、`EnqueueModeration`、`ApplyModerationResult`
  - `attachment.go` — 附件 JSON 编解码、校验（数量 ≤9、file 存在、ref_type 正确）
- **依赖**：`core`（DB/Redis/Kafka/Logger）、`danmaku`（`SensitiveFilter` + `SensitiveWord` 实体）、`model/entity/social`、`model/entity/file`。

### 模块 C：`internal/api/v1`（handler）
- `comment.go`：`CommentApi`（`ListComments/ListReplies/CreateComment/ToggleLike/DeleteComment/CommentCount`）+ `SetCommentService` 注入。
- `File.go`：新增 `CommentImageUpload`（multipart，限制大小，`ref_type=comment_image`，返回 `file_id`）。
- **依赖**：`comment`、`auth`（`GetUserID`）、`authclient`（作者信息批量查询，复用 `resolveAuthors` 思路）。

### 模块 D：`internal/core/message_queue/comment`（审核 worker）
- `worker.go`：`StartCommentModerationWorker(ctx)`，消费 `comment_moderation`，解析 `{comment_id}`，读附件文件列表 → `Moderator.Review` → `Service.ApplyModerationResult`。
- **依赖**：`core`、`comment`、`model/entity/file`。

### 模块 E：路由与装配
- `internal/routers/api/v1/comment.go`（新）+ `enter.go`/`router.go`（改）。
- `internal/role/api.go`：装配 `comment.NewService` + `LoadSensitiveWords` + `StartFlusher`。
- `internal/role/worker.go`：`StartCommentModerationWorker`。
- `internal/consts/Kafka.go`：加 `KafkaTopicCommentModeration = "comment_moderation"`。
- `internal/config/config.go`：加 `Comment` 配置块（`Enabled/FlushInterval/FlushBatch`），`conf/app.toml` 加 `[comment]`。
- `internal/model/entity/file/file.go`：加 `FileRefTypeCommentImage FileRefType = "comment_image"`。

### 模块 F：前端 `web/web-client/src/views/VideoPlayer/comment/`
- `api.ts` — 评论接口封装（list/replies/create/like/delete/count/上传图片）
- `types.ts` — `CommentItem/Author/Attachment` 类型
- `CommentSection.vue` — 汇总：列表 + 分页加载 + 输入框
- `CommentItem.vue` — 单条（头像/昵称/时间/内容/附件/点赞/回复/删除 + 楼中楼展开）
- `CommentInput.vue` — 输入（文本 + emoji + 图片/表情上传 + 回复 @ 提示）
- 接入 `VideoPlayer/index.vue`：在视频信息下方挂载 `<CommentSection :video-id="videoId" />`。

---

## 模块交互

### 写：发表评论/回复（文本 + 附件）
```
POST /api/v1/videos/:id/comments
  CommentApi.CreateComment
    → comment.Service.Create
       1. filter.Contains(content) 命中 → 返回 ErrSensitive（400）
       2. parent_id != nil → 查父评论，推导 root_id；校验 reply_to_id 属于同一 root
       3. attachment.Validate：数量≤9、file 存在且 ref_type=comment_image、对应文件 ref_count+1
       4. snowflake 生成 id，DB 写 VideoComment（纯文本 status=visible；含附件 status=pending）
       5. Redis：INCR comment:count:{video}；若为回复则 INCR comment:reply:{root}
       6. 含附件 → EnqueueModeration（投 Kafka）
    → 返回评论 id + 内容
```

### 写：点赞（幂等 toggle）
```
POST /api/v1/comments/:id/like
  → Service.ToggleLike：Lua SISMEMBER→SADD/SREM + INCR/DECR like_count（Redis）
    → 计数异步 flush 到 video_comments.like_count + comment_likes 表（复用 interaction flusher 模式）
```

### 写：删除（软删除）
```
DELETE /api/v1/comments/:id
  → Service.Delete：校验归属（user_id==当前用户）→ status=deleted + deleted_at=now + content 置空
    → 附件 ref_count 递减；不回删子回复
```

### 读：评论列表
```
GET /api/v1/videos/:id/comments?cursor=&limit=
  → Service.List
      1. 首屏（cursor=0）命中 core.Cache → 返回
      2. DB：video_id=? AND root_id IS NULL AND status='visible' AND (cursor=0 OR id<cursor) ORDER BY id DESC LIMIT limit
      3. 批量取每个根的摘要回复（IN root_id，每条最新 2 条，status='visible'）
      4. 批量查作者（authclient）+ 点赞数（Redis）+ 当前用户点赞状态（若登录）
      5. 首屏写 core.Cache（TTL 60s，写入/删除/审核时失效）
```

### 读：展开回复
```
GET /api/v1/comments/:id/replies?cursor=&limit=
  → Service.ListReplies：root_id=? AND status='visible' AND id<cursor ORDER BY id DESC
```

### 审核：异步图片审核
```
发表含图评论 → Kafka "comment_moderation"
  → StartCommentModerationWorker
      1. 解析 {comment_id}
      2. 读评论附件 files
      3. Moderator.Review(files) → pass
      4. ApplyModerationResult(comment_id, pass)：
           pass=true → status=visible
           pass=false → status=hidden + 附件 ref_count 递减
```

---

## 文件组织

```
internal/
├── comment/
│   ├── comment.go            — Service、Create/List/ListReplies/Delete/ToggleLike、CreateInput
│   ├── keys.go               — Redis key 函数
│   ├── counter.go            — 计数 + flusher（点赞/回复/视频总数）
│   ├── moderation.go         — Moderator、PassthroughModerator、审核队列/结果回写
│   └── attachment.go         — 附件编解码与校验
├── model/entity/social/
│   ├── comment.go            — 扩展 VideoComment（新增字段）
│   └── comment_like.go       — 新增 CommentLike
├── model/entity/file/file.go — 加 FileRefTypeCommentImage
├── api/v1/
│   ├── comment.go            — CommentApi + SetCommentService
│   └── File.go               — 加 CommentImageUpload
├── routers/api/v1/
│   ├── comment.go            — 新路由
│   └── enter.go              — 加 CommentRouter
├── routers/router.go         — 注册公开/私有路由
├── role/
│   ├── api.go                — 装配 comment.Service
│   └── worker.go             — 启动审核 worker
├── core/message_queue/comment/
│   └── worker.go             — 审核消费者
├── consts/Kafka.go           — 加 comment_moderation topic
└── config/config.go          — 加 Comment 配置块

conf/app.toml                 — 加 [comment]
migrations/migrate.go         — 加 CommentLike + 评论索引

web/web-client/src/views/VideoPlayer/
├── comment/
│   ├── api.ts
│   ├── types.ts
│   ├── CommentSection.vue
│   ├── CommentItem.vue
│   └── CommentInput.vue
└── index.vue                 — 挂载 <CommentSection />
```

---

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 级联模型 | `root_id` + `reply_to_id` 邻接表，两级扁平 | 视频场景最优；`root_id` 等值查询展开回复，无需递归 CTE |
| 附件存储 | `VideoComment.attachments` JSONB（`[]CommentAttachment`） | 读多写少、保持顺序、复用 `files` 引用计数，与 `video_manifest.profiles` 的 JSONB 习惯一致 |
| 敏感词 | 复用 `internal/danmaku` 的 `SensitiveFilter` 类型，敏感词仍读 `sensitive_words` 表 | 零风险纯增量，不重写 AC，也不改现有弹幕代码 |
| 计数 | Redis Set/计数器 + 异步 flush（仿 interaction） | 现有成熟模式，最终一致 |
| 图片审核 | Kafka topic + `Moderator` 接口 + `PassthroughModerator` 桩 | 解耦第三方供应商，无供应商也能跑通链路 |
| 软删除 | `gorm.DeletedAt` + `status=deleted` + 清空 content | 保留楼中楼结构，不回删子回复 |
| 评论点赞 | 独立 `comment_likes` 复合主键表 | 与视频点赞（`video_likes`）隔离 |
| 列表缓存 | `core.Cache` 首屏缓存（TTL 60s）+ 写失效 | 读多场景；游标深分页不缓存，避免失效复杂度 |
| 分页 | `id < cursor` 游标（snowflake 时间有序） | 避免 OFFSET 深分页性能问题 |
| 评论总数 | Redis 计数器（`comment:count:{video}`），异步回写视频评论总数 | 复用计数模式 |
| 作者信息 | `authclient.UserClient` 批量查询 | 用户资料已在 auth 服务，评论列表需批量填充昵称/头像 |

---

## 与 spec 功能需求的覆盖

| spec 需求 | plan 归属 |
|-----------|-----------|
| F1 发表一级评论 | Service.Create（parent=nil）、CommentApi、前端 CommentInput |
| F2 楼中楼 | `root_id`/`reply_to_id` 推导 + ListReplies + CommentItem 展开 |
| F3 列表与分页 | Service.List/ListReplies（游标 + 摘要回复） |
| F4 内容类型 | `attachments` JSONB + attachment.go 校验 + 前端上传 |
| F5 敏感词 | 复用 `danmaku.SensitiveFilter` |
| F6 图片审核 | `moderation.go` + `message_queue/comment/worker.go` |
| F7 点赞 | Service.ToggleLike + CommentLike |
| F8 软删除 | Service.Delete（gorm.DeletedAt + status） |
| F9 计数冗余 | counter.go + flusher |
| F10 前端界面 | 模块 F 前端组件 |

---

请审批这份 `plan.md`：
1. **通过** → 进入阶段三，写 `task.md`
2. **修改** → 告诉我调整点（如敏感词复用方式、缓存策略、接口路径、附件建模等）
