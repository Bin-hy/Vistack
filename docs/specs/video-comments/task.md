# 视频评论系统 Tasks

> 对应已批准的 `spec.md` 与 `plan.md`。执行顺序见文末图。

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 修改 | `internal/model/entity/social/comment.go` | 扩展 `VideoComment`（状态/级联/附件/计数/软删除） |
| 新建 | `internal/model/entity/social/comment_like.go` | `CommentLike` 复合主键实体 |
| 修改 | `internal/model/entity/file/file.go` | 加 `FileRefTypeCommentImage` 枚举 |
| 修改 | `migrations/migrate.go` | 注册 `CommentLike` + 评论索引 |
| 新建 | `internal/comment/keys.go` | Redis key 函数 |
| 新建 | `internal/comment/attachment.go` | 附件 JSON 编解码与校验 |
| 新建 | `internal/comment/comment.go` | `Service` 及 Create/List/Delete/ToggleLike |
| 新建 | `internal/comment/counter.go` | 计数 + 异步 flusher |
| 新建 | `internal/comment/moderation.go` | `Moderator` 接口 + 审核投递/回写 |
| 修改 | `internal/consts/Kafka.go` | 加 `KafkaTopicCommentModeration` |
| 新建 | `internal/core/message_queue/comment/worker.go` | 图片审核消费者 |
| 新建 | `internal/api/v1/comment.go` | `CommentApi` + 注入 |
| 修改 | `internal/api/v1/File.go` | 加 `CommentImageUpload` |
| 新建 | `internal/routers/api/v1/comment.go` | 评论路由 |
| 修改 | `internal/routers/api/v1/enter.go` | 挂 `CommentRouter` |
| 修改 | `internal/routers/router.go` | 注册公开/私有评论路由 |
| 修改 | `internal/config/config.go` | 加 `Comment` 配置块 |
| 修改 | `conf/app.toml`、`conf/app.local.toml` | 加 `[comment]` 配置 |
| 修改 | `internal/role/api.go` | 装配 comment.Service |
| 修改 | `internal/role/worker.go` | 启动审核 worker |
| 新建 | `web/web-client/src/views/VideoPlayer/comment/types.ts` | 前端类型 |
| 新建 | `web/web-client/src/views/VideoPlayer/comment/api.ts` | 评论接口封装 |
| 新建 | `web/web-client/src/views/VideoPlayer/comment/CommentItem.vue` | 单条评论/回复 |
| 新建 | `web/web-client/src/views/VideoPlayer/comment/CommentInput.vue` | 输入框 |
| 新建 | `web/web-client/src/views/VideoPlayer/comment/CommentSection.vue` | 汇总组件 |
| 修改 | `web/web-client/src/views/VideoPlayer/index.vue` | 挂载评论组件 |

---

## T1: 扩展 VideoComment 实体

**文件：** `internal/model/entity/social/comment.go`
**依赖：** 无
**步骤：**
1. 新增 `CommentStatus` 类型及常量 `CommentStatusVisible/Pending/Hidden/Deleted`。
2. 新增 `CommentAttachment` 结构体：`Type string`、`FileID int64`，json tag 分别为 `type`、`file_id,string`。
3. 在 `VideoComment` 中新增字段：`RootID *int64`、`ParentID *int64`（已有）、`ReplyToID *int64`、`ReplyToUID *int64`、`Attachments string`（`type:jsonb`）、`Status CommentStatus`（`default:visible`）、`LikeCount int64`、`ReplyCount int64`、`UpdatedAt time.Time`、`DeletedAt gorm.DeletedAt`。
4. 移除旧的 `Parent *VideoComment` 自关联字段（回复关系用 ID 表达，作者信息走 authclient）。
5. 保留 `BeforeCreate` 雪花 ID 钩子与 `TableName`。

**验证：** `go build ./internal/model/entity/social/...` 通过。

## T2: 新增 CommentLike 实体

**文件：** `internal/model/entity/social/comment_like.go`
**依赖：** 无
**步骤：**
1. 定义 `CommentLike`：`CommentID int64`（`primaryKey`）、`UserID int64`（`primaryKey`）、`CreatedAt time.Time`。
2. `TableName()` 返回 `comment_likes`。

**验证：** `go build ./internal/model/entity/social/...` 通过。

## T3: files 加评论图片枚举

**文件：** `internal/model/entity/file/file.go`
**依赖：** 无
**步骤：**
1. 在 `FileRefType` 常量组新增 `FileRefTypeCommentImage FileRefType = "comment_image"`。

**验证：** `go build ./internal/model/entity/file/...` 通过。

## T4: 迁移注册 + 索引

**文件：** `migrations/migrate.go`
**依赖：** T1、T2
**步骤：**
1. 在社交互动迁移组加入 `&mSocial.CommentLike{}`。
2. 在 `AutoMigrate` 末尾加显式建索引：`CREATE INDEX IF NOT EXISTS idx_comments_video_root ON video_comments (video_id, root_id, id);`（忽略错误，仿照现有 transcode 索引写法）。

**验证：** `go build ./...` 通过。

## T5: Redis key 定义

**文件：** `internal/comment/keys.go`
**依赖：** 无
**步骤：**
1. 定义 key 函数：`likeKey(commentID)` → `vistack:comment:like:{id}`；`replyKey(rootID)` → `vistack:comment:reply:{id}`；`commentCountKey(videoID)` → `vistack:comment:count:{video_id}`；`listCacheKey(videoID)` → `vistack:comment:list:{video_id}`。

**验证：** `go build ./internal/comment/...` 通过。

## T6: 附件编解码与校验

**文件：** `internal/comment/attachment.go`
**依赖：** T3、T5
**步骤：**
1. 定义常量 `maxAttachments = 9`。
2. `parseAttachments(raw string) ([]CommentAttachment, error)`：空串返回空切片，否则 JSON 反序列化。
3. `marshalAttachments([]CommentAttachment) (string, error)`。
4. `validateAttachments(ctx, db, items) error`：数量 >9 报错；每个 FileID 查 `files` 表，不存在或 `ref_type != comment_image` 报错。

**验证：** `go build ./internal/comment/...` 通过。

## T7: Service 骨架

**文件：** `internal/comment/comment.go`
**依赖：** T5、T6
**步骤：**
1. 定义 `Options`（`FlushInterval`/`FlushBatch`/`Logger`）、`CreateInput`、`ErrSensitive = errors.New("sensitive word")`、`ErrNotFound`、`ErrForbidden`。
2. 定义 `Service` 结构体（`rdb`/`db`/`filter *danmaku.SensitiveFilter`/`opts`）。
3. `NewService(rdb, db, opts)`：默认 `FlushInterval=5s`、`FlushBatch=200`，`filter = danmaku.NewSensitiveFilter(nil)`。
4. `LoadSensitiveWords(ctx)`：查 `danmaku.SensitiveWord` 表，重建 filter。

**验证：** `go build ./internal/comment/...` 通过。

## T8: Create 发表评论/回复

**文件：** `internal/comment/comment.go`
**依赖：** T7
**步骤：**
1. `Create(ctx, in)`：`filter.Contains(in.Content)` 命中返回 `ErrSensitive`。
2. 若 `in.ParentID != nil`：查父评论，父不存在返回 `ErrNotFound`；推导 `rootID`（父的 `RootID` 非空取之，否则取父的 `ID`）；设置 `ParentID`。
3. 若 `in.ReplyToID != nil`：查该评论，校验其 `root` 与 `rootID` 一致，否则报错；`ReplyToUID` 取其 `UserID`。
4. `validateAttachments`；通过后对每个附件文件 `ref_count +1`。
5. 构造 `VideoComment`（snowflake id、`Status`：纯文本=visible，含附件=pending）。
6. 事务内写库（附件 ref_count 递增 + 评论插入同一事务）。
7. Redis：`INCR commentCountKey(videoID)`；若为回复 `INCR replyKey(rootID)`。
8. 含附件则 `EnqueueModeration(ctx, id)`。

**验证：** `go build ./internal/comment/...` 通过。

## T9: List / ListReplies / CommentCount

**文件：** `internal/comment/comment.go`
**依赖：** T7
**步骤：**
1. `List(ctx, videoID, cursor, limit)`：`WHERE video_id=? AND root_id IS NULL AND status='visible' AND id < cursor`（cursor=0 不加 id 条件），`ORDER BY id DESC LIMIT ?`；返回结果与 `next`（最后一条 id，少于 limit 则 0）。
2. `ListReplies(ctx, rootID, cursor, limit)`：`WHERE root_id=? AND status='visible' AND id < cursor ORDER BY id DESC LIMIT ?`。
3. `CommentCount(ctx, videoID)`：`GET commentCountKey`，`redis.Nil` 回退 DB `COUNT`（`video_id=? AND root_id IS NULL AND status='visible'`）。

**验证：** `go build ./internal/comment/...` 通过。

## T10: ToggleLike / IsLiked

**文件：** `internal/comment/counter.go`
**依赖：** T5、T7
**步骤：**
1. 写 Lua 脚本（仿 interaction `toggleLikeScript`）：`SISMEMBER`→`SADD/SREM` + 推事件到 pending list，返回 `{liked, count}`。
2. `ToggleLike(ctx, commentID, userID)`：校验评论存在且 `status='visible'`，执行脚本，返回 `(liked, count, err)`。
3. `IsLiked(ctx, commentID, userID)`：`SIsMember likeKey`。

**验证：** `go build ./internal/comment/...` 通过。

## T11: 计数 flusher

**文件：** `internal/comment/counter.go`
**依赖：** T10
**步骤：**
1. 定义 `countEvent{Kind, ID, Delta}`（`like`/`reply`/`video_comment`），复用 T10 的事件 pending list。
2. `flush()`：批量 LPOP 事件，聚合后写库：`comment_likes` upsert/delete、`video_comments.like_count`、`reply_count`、视频评论总数。
3. `StartFlusher(ctx)`：ticker 定时 drain（仿 `interaction.StartFlusher`）。

**验证：** `go build ./internal/comment/...` 通过。

## T12: Delete 软删除

**文件：** `internal/comment/comment.go`
**依赖：** T7
**步骤：**
1. `Delete(ctx, commentID, userID)`：查评论，`userID` 不匹配返回 `ErrForbidden`。
2. 事务：`status=deleted`、`deleted_at=now`、`content` 置空、`attachments` 置空；附件文件 `ref_count -1`。
3. `DECR commentCountKey`（若为一级评论）；若为回复 `DECR replyKey(rootID)`。

**验证：** `go build ./internal/comment/...` 通过。

## T13: 审核编排

**文件：** `internal/comment/moderation.go`
**依赖：** T7
**步骤：**
1. 定义 `Moderator` 接口与 `PassthroughModerator`（`Review` 恒返回 `true, nil`）。
2. 定义 `ModerationMessage{CommentID int64}`，`json.Marshal`。
3. `EnqueueModeration(ctx, commentID)`：`core.SendKafkaMessage(ctx, string(consts.KafkaTopicCommentModeration), strconv.FormatInt(commentID,10), raw)`。
4. `ApplyModerationResult(ctx, commentID, pass)`：pass→`status=visible`；否则 `status=hidden` 且附件 `ref_count -1`。

**验证：** `go build ./internal/comment/...` 通过。

## T14: Kafka topic 常量

**文件：** `internal/consts/Kafka.go`
**依赖：** 无
**步骤：**
1. 常量组新增 `KafkaTopicCommentModeration KafkaTopic = "comment_moderation"`。

**验证：** `go build ./internal/consts/...` 通过。

## T15: 审核 worker

**文件：** `internal/core/message_queue/comment/worker.go`
**依赖：** T13、T14
**步骤：**
1. `StartCommentModerationWorker(ctx)`：`EnsureTopic` + `core.StartKafkaConsumer`。
2. handler：反序列化 `ModerationMessage` → 读评论附件 `files` → `moderator.Review` → 调 `comment.Service.ApplyModerationResult`。
3. 用包级 `var moderator comment.Moderator = comment.PassthroughModerator{}` + `SetModerator` 供未来替换。

**验证：** `go build ./...` 通过。

## T16: CommentApi handler

**文件：** `internal/api/v1/comment.go`
**依赖：** T8–T13
**步骤：**
1. 包级 `var commentService *comment.Service` + `SetCommentService`。
2. 请求/响应结构：`createCommentRequest{Content string; Attachments []comment.CommentAttachment; ParentID *int64; ReplyToID *int64}`、`commentItemResponse{...}`、`commentListResponse{comments, next_cursor, total}`。
3. 实现：`ListComments`（公开）、`ListReplies`（公开）、`CreateComment`（鉴权，返回 400 敏感词）、`ToggleLike`、`DeleteComment`、`CommentCount`（公开）。
4. 作者信息用 `authclient.UserClient` 批量填充（复用 `resolveAuthors` 思路，对 comment 的 `UserID` 去重查询）。

**验证：** `go build ./internal/api/v1/...` 通过。

## T17: 评论图片上传

**文件：** `internal/api/v1/File.go`
**依赖：** T3
**步骤：**
1. 新增 `CommentImageUpload`：`c.FormFile("file")`，限 5MB，`storage.UploadFile(ctx, file, "comments")`。
2. 建 `mFile.File{RefType: mFile.FileRefTypeCommentImage, ...}`，返回 `FileUploadedResponse`。

**验证：** `go build ./internal/api/v1/...` 通过。

## T18: 路由注册

**文件：** `internal/routers/api/v1/comment.go`（新）、`enter.go`、`router.go`（改）
**依赖：** T16
**步骤：**
1. `comment.go`：`CommentRouter`，公开 `GET /videos/:id/comments`、`GET /videos/:id/comments/count`、`GET /comments/:id/replies`；私有 `POST /videos/:id/comments`、`POST /comments/:id/like`、`DELETE /comments/:id`。
2. `enter.go`：`RouterGroup` 加 `CommentRouter`。
3. `router.go`：公开组加 `InitCommentPublicRouter`，鉴权组加 `InitCommentPrivatesRouter`。
4. 文件上传路由：`file.go` 的 `InitFileRouter` 加 `POST /file/comment`。

**验证：** `go build ./...` 通过。

## T19: 配置 + API 装配

**文件：** `internal/config/config.go`、`conf/app.toml`、`conf/app.local.toml`、`internal/role/api.go`
**依赖：** T7、T11
**步骤：**
1. `config.go`：加 `Comment struct{ Enabled bool; FlushInterval int; FlushBatch int }`。
2. `conf/app.toml`、`conf/app.local.toml`：加 `[comment]`（`enabled=true`、`flush_interval=5`、`flush_batch=200`）。
3. `role/api.go`：`cfg.Comment.Enabled` 时 `comment.NewService` + `SetCommentService` + `LoadSensitiveWords` + `StartFlusher`。

**验证：** `go build ./...` 通过。

## T20: worker 装配

**文件：** `internal/role/worker.go`
**依赖：** T15
**步骤：**
1. import `mq_comment`，在 `RunWorker` 常规消费者处加 `mq_comment.StartCommentModerationWorker(ctx)`。

**验证：** `go build ./...` 通过。

## T21: 后端全量校验

**文件：** 无（检查）
**依赖：** T1–T20
**步骤：**
1. `gofmt -l .` 无输出（或格式化）。
2. `go build ./...` 通过。
3. `go vet ./...` 通过。

**验证：** 三条命令均通过。

---

## T22: 前端类型与 API

**文件：** `web/web-client/src/views/VideoPlayer/comment/types.ts`、`api.ts`
**依赖：** 无
**步骤：**
1. `types.ts`：`Author{id,nickname,avatar_url}`、`Attachment{type,file_id}`、`CommentItem{id,video_id,user_id,root_id,parent_id,reply_to_id,reply_to_uid,content,attachments,status,like_count,reply_count,created_at,author,liked,replies}`、`CommentListResponse{comments,next_cursor,total}`。
2. `api.ts`：`listComments(videoId,cursor,limit)`、`listReplies(commentId,cursor,limit)`、`createComment(videoId,payload)`、`toggleLike(commentId)`、`deleteComment(commentId)`、`commentCount(videoId)`、`uploadCommentImage(file)`（multipart）。

**验证：** 待 T25 后整体 `pnpm --filter web-client run build` 通过。

## T23: CommentItem 组件

**文件：** `web/web-client/src/views/VideoPlayer/comment/CommentItem.vue`
**依赖：** T22
**步骤：**
1. 渲染头像/昵称/相对时间/正文/附件（图片 `file_id` 拼 URL，复用 `file.PublicURL` 约定或后端返回 url）/点赞数/回复按钮/删除按钮（本人可见）。
2. 楼中楼：有 `reply_count` 时显示「展开 N 条回复」，点击加载 `listReplies` 并渲染子 `CommentItem`（`is-reply` 模式，显示「回复 @xxx」引用）。
3. 点赞：调 `toggleLike` 乐观更新。

**验证：** 待 T26 后整体构建通过。

## T24: CommentInput 组件

**文件：** `web/web-client/src/views/VideoPlayer/comment/CommentInput.vue`
**依赖：** T22
**步骤：**
1. 文本输入框 + 未登录提示登录。
2. 图片/表情上传按钮：选文件 → `uploadCommentImage` → 追加到附件列表 → 预览（可删除）。
3. 回复态：显示「回复 @xxx」，提交带 `parent_id`/`reply_to_id`。
4. 提交调 `createComment`，成功后回调父组件刷新列表。

**验证：** 待 T26 后整体构建通过。

## T25: CommentSection 组件

**文件：** `web/web-client/src/views/VideoPlayer/comment/CommentSection.vue`
**依赖：** T23、T24
**步骤：**
1. 顶部显示评论总数（`commentCount`）。
2. 首屏 `listComments`，游标 `next_cursor` 加载更多。
3. 组合 `CommentInput` + `CommentItem` 列表，发表/删除后刷新。

**验证：** 待 T26 后整体构建通过。

## T26: 接入 VideoPlayer

**文件：** `web/web-client/src/views/VideoPlayer/index.vue`
**依赖：** T25
**步骤：**
1. import `CommentSection`，在视频信息/统计下方挂载 `<CommentSection :video-id="videoId" />`。

**验证：** `pnpm --filter web-client run build` 通过（vue-tsc + vite build）。

---

## 执行顺序

```
T1 → T2 → T4          T3
   ↘        ↘          ↘
     T5 → T6 → T7 → T8 → T9
                     ↘
                 T10 → T11
T14 → T13 → T15 → T20（worker 装配）
T7 → T12（软删除）
T3 → T17（上传）
T8,T9,T10,T12,T13 → T16 → T18 → T19 → T21（后端完成）

T22 → T23 → T24 → T25 → T26（前端完成）
```

后端与前端可并行；后端完成后整体联调。

---

请审批这份 `task.md`：
1. **通过** → 进入阶段四，写 `checklist.md`
2. **修改** → 告诉我调整点（任务粒度、拆分方式、验证命令等）
