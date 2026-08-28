# 视频评论系统 Checklist

> 每一项通过「运行代码或观察行为」验证，聚焦系统行为。对应 `spec.md` 的 AC1–AC10。

## 实现完整性

- [ ] 后端编译通过，`internal/comment` 包与 `CommentApi`/审核 worker/路由均已实现（验证：`go build ./...` 无错误）
- [ ] `VideoComment` 已含 `root_id/reply_to_id/attachments/status/like_count/reply_count/deleted_at` 字段且 AutoMigrate 建列（验证：启动后 `\d video_comments` 可见新列与 `idx_comments_video_root` 索引）
- [ ] 前端评论组件已挂载到播放页（验证：打开视频播放页可见评论区区域）

## 功能行为（对应 AC）

- [ ] AC1 发表一级评论：登录用户发表纯文本评论，接口返回新 id，库里该记录 `root_id IS NULL`、`status='visible'`（验证：`POST /api/v1/videos/:id/comments` 后查库）
- [ ] AC2 楼中楼：对一级评论发回复 → `root_id` 指向一级评论；对该回复再回复 → 仍归属同一 `root_id`，`reply_to_id` 指向被回复对象，前端显示「回复 @xxx」且无深层缩进（验证：连续发两条回复观察数据与界面）
- [ ] AC3 列表分页：列表按时间倒序；传 `cursor` 翻页无重复无遗漏；每个一级评论带摘要回复；可展开拿到该 `root` 下全部回复（验证：造 25 条评论，limit=10 翻页比对）
- [ ] AC4 内容类型：文本+emoji、图片、GIF/PNG 表情包均能发表并按序展示；一次传 10 张图片被拒绝（验证：上传 1 张图 + 1 张 GIF 成功，10 张返回 400）
- [ ] AC5 敏感词：文本含敏感词时发表被拒绝且不落库（验证：先加敏感词，再发含词评论返回 400）
- [ ] AC6 图片审核：含图评论落库 `status='pending'`，桩审核器消费后转 `visible`；模拟 `hidden` 后前端不可见（验证：发图评论 → 观察状态流转与列表可见性）
- [ ] AC7 点赞：点赞后 `like_count` +1，同用户重复点赞不重复计数，取消后 -1（验证：连续两次 `POST /comments/:id/like` 后查数）
- [ ] AC8 软删除：删除评论后内容显示「已删除」占位，其下回复仍保留可见（验证：删一级评论后观察其回复仍在）
- [ ] AC9 计数：评论的回复数、点赞数、视频评论总数在短暂延迟后收敛为真实值（验证：操作后等一个 flush 周期再读）
- [ ] AC10 前端全流程：`查看列表 → 发表评论 → 回复 → 上传图片/表情 → 点赞 → 删除` 全程可操作（验证：浏览器手动走通）

## 集成

- [ ] 评论作者信息正确填充昵称/头像（验证：列表接口返回 author，前端展示非空）
- [ ] 图片上传走 `files` 引用计数：发评论 `ref_count+1`，删评论/审核拒绝 `ref_count-1`（验证：查 `files.ref_count` 变化）
- [ ] 审核 worker 与 API 角色解耦：worker 消费 `comment_moderation` 回写状态（验证：worker 日志可见消费并更新）
- [ ] 计数 flush 与 Redis 解耦：Redis 故障时列表仍可读、计数回退 DB（验证：见 spec N2，可读性不因 Redis 异常中断）

## 编译与测试

- [ ] `gofmt -l .` 无输出
- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无错误
- [ ] `pnpm --filter web-client run build` 通过（vue-tsc 类型检查 + vite 构建）

## 端到端场景

- [ ] 场景 1（主流程）：登录用户打开视频 → 发表一条含 emoji 的评论 → 对自己评论发一条回复 → 对回复再回复 → 点赞自己的评论 → 删除回复 → 观察楼中楼结构、计数、占位显示均正确
- [ ] 场景 2（边界）：未登录查看评论区可读但不可发表/点赞（提示登录）；发含敏感词评论被拒；发含 10 张图的评论被拒；删除他人评论被拒（403）

---

请审批这份 `checklist.md`：
1. **通过** → 四份文档齐备，进入阶段五按 `task.md` 开发
2. **修改** → 告诉我调整点（验收项、场景、验证方式等）
