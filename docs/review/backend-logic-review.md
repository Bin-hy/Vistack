# 后端逻辑 Review 报告

> 评审范围：Go 后端全链路（认证 / 上传 / 转码 / 删除 / 存储 / 配置）。结论按严重度分级，均附文件位置与修复建议。

## 总评

整体架构清晰、分层合理（`api → router → core → model`，`pkg` 工具层），转码链路（分片直传 + 秒传去重 + Kafka 任务 + DASH 分片 + STS 防盗链）设计完整，密码使用 bcrypt、JWT 校验算法、删除走软删 + 引用计数，这些是**正确的**。但存在若干**会影响生产可靠性与安全性**的问题，其中「Snowflake node_id 冲突」「ref_count 双重扣减」「Kafka 异步发送静默丢消息」三项与刚做的水平扩展改造直接相关，建议优先处理。

---

## 🔴 高危（必须处理）

### H1. 默认 JWT 密钥为 `"secret"`
- 位置：`conf/app.toml`、`conf/app.docker.toml` 的 `[auth] jwt_secret = "secret"`
- 问题：任何知道默认值的人都能伪造任意用户（含 admin）的 Token。
- 建议：从环境变量/Secret 注入，部署时强制非空且长度 ≥ 32；启动时校验 `jwt_secret == "secret"` 则拒绝启动。

### H2. 启动日志打印完整配置（泄露密钥）
- 位置：`cmd/vistack/main.go`（原 `fmt.Println("gorm automigrate failed:", cfg)`）、`internal/core/minio.go`（`fmt.Printf("OnInitMinioClient, minioConfig: %+v", minioConfig)`）
- 问题：把 `cfg` 整体打印到 stdout，包含 MinIO `secret_key`、JWT `jwt_secret` 等；且文案 "automigrate failed" 是误导（实际未必失败）。
- 建议：删除这两处 `fmt.Println/Printf`，改用脱敏的 zap 日志。

### H3. Snowflake `node_id=1` 硬编码，多实例会撞 ID
- 位置：`conf/app.toml` `[snowflake] node_id = 1`；`pkg/snowflake/snowflake.go` 懒加载兜底也是 `Init(1)`
- 问题：三角色拆分后 `api` 与 `worker` 都生成 ID（视频、文件、转码记录等）。水平扩容成多副本时，各实例 `node_id` 都是 1，同一毫秒生成 → **主键冲突**。这与「可水平扩展」目标直接矛盾。
- 建议：每个实例从环境派生唯一 `node_id`（如 k8s 用 `POD_IP` 末段哈希、compose 用 `--scale` 序号、或 etcd 分配）。

### H4. 视频删除 `ref_count` 被扣减两次
- 位置：`internal/api/v1/Video.go` 的 `DeleteVideo`（先减引用计数并标记 deleting）**和** `internal/core/message_queue/video/delete_video_worker.go` 的 `handleVideoDeleteMessage`（`processFile` 又减一次）
- 问题：同一文件的 `ref_count` 被 API 层和 Worker 层各减一次，单引用文件会从 1 → 0 → -1，引用计数彻底失真。
- 建议：职责单一化——API 只做软删 + 发消息，`ref_count` 的扣减与物理删除统一放到 Worker 一处执行。

### H5. Kafka 异步发送 + 忽略返回值 → 任务静默丢失
- 位置：`internal/core/kafka.go`（`Async: true`）、`internal/api/v1/Video.go`（`_ = core.SendKafkaMessage(...)`）
- 问题：`Async` 模式下 `WriteMessages` 立即返回，真正失败只在上层 `Completion` 回调里记日志；调用方又丢弃返回值。Kafka 抖动时，转码消息丢失，`video_transcodes` 停在 `pending`（watchdog 只看 `processing`），视频永久卡死且无人兜底。
- 建议：改为同步发送并处理错误；或对 `pending` 超时任务也加一个兜底扫描。

---

## 🟠 中危（建议尽快）

### M1. 上传文件只校验大小、不校验类型
- 位置：`internal/api/v1/File.go`（AvatarUpload/CoverUpload）、`User.go`（UpdateProfileDirect）
- 问题：仅 `>5MB` 拦截，未校验 MIME/魔数；上传 SVG 可造成存储型 XSS（头像/封面走公开 URL 直出）。
- 建议：校验文件头（`http.DetectContentType` 或魔数），白名单 `image/jpeg/png/webp`。

### M2. 登录无限流 / 无账号锁定
- 位置：`internal/api/v1/User.go` `Login`
- 问题：无速率限制，可暴力破解（Redis 已有依赖，可加 `login:attempts:<ip>` 计数）。

### M3. STS 使用 Root 凭证签发
- 位置：`internal/api/v1/Video.go` `GetVideoSegmentsSignature`
- 问题：用 `MinIO.AccessKey/SecretKey`（root）做 `AssumeRole`。应使用权限受限的专用账号，降低泄露面。

### M4. JWT 库已废弃
- 位置：`pkg/auth/token_manager.go` 依赖 `github.com/dgrijalva/jwt-go`
- 问题：该库已停维护，官方推荐迁移到 `github.com/golang-jwt/jwt/v5`。当前用法（HS256+算法校验）无已知严重漏洞，但应升级。

### M5. `UpdateProfileDirect` panic 后无响应
- 位置：`User.go` `UpdateProfileDirect` 的 `defer recover()` 只 `Rollback`，不回写 JSON，客户端会得到空响应。
- 建议：recover 后返回 500，或去掉裸 recover 让 Gin 的 Recovery 中间件兜底。

### M6. 头像/封面文件记录无生命周期 GC
- 位置：`File.go`/`User.go` 上传后创建 `files` 记录；替换时仅打 `status=replaced` 标签 + MinIO 生命周期删对象，但**数据库行永不删除**。
- 建议：加定时任务清理 `status=replaced/deleting` 且超期的 `files` 行。

---

## 🟡 低危 / 改进

- **L1**：`internal/core/minio.go` 的 `GetInternalBaseURL` 有残留 `fmt.Printf("Internal")`（无换行，污染日志）；`Video.go` `GetVideoInfo` 有 `fmt.Println("avatarURL:")` 调试输出。
- **L2**：`CompleteVideoUpload` 硬编码 `MimeType: "video/mp4"`，应据上传扩展名/探测推断，兼容 mov/webm/mkv。
- **L3**：`GetVideoMdp` 每次读 MinIO 且 `object.Stat()` 调两次；可加 Redis 缓存。
- **L4**：`GetVideoRecommend` 无分页/无个性化（v1 可接受）。
- **L5**：`Register` 昵称唯一性靠 `Count` 而非唯一索引，存在并发下重复昵称的竞态（用户名已有 `uniqueIndex` 是好的）。
- **L6**：`InitMinioClient` 每次启动都重写 bucket policy，会覆盖手工配置。
- **L7**：`login` 未校验 `User.Status`，被封禁用户仍可登录。
- **L8**：transcode 强制 `-r 30` + `cfr`，非 30fps 源会被重采样（可接受，但需知晓）。

---

## ✅ 做得好的地方（保持）

- 密码 bcrypt 哈希、JWT HS256 且校验算法（拒绝非 HMAC）。
- RBAC 模型完整（roles/authorities/role_authority/user_authority）。
- 分片直传 + 预签名 URL + 文件哈希秒传去重，链路完整。
- DASH 分片 STS 临时凭证防盗链，`dash/` 目录不公开。
- 转码指数退避重试 + Redis ZSet 延迟队列 + watchdog 超时兜底。
- 删除走软删 + 引用计数意图（虽 H4 有重复扣减 bug）。
- 数据库写入普遍使用事务；日志统一 zap。
