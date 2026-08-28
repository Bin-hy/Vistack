# 后端高危问题修复（H1–H5）Spec

## 背景

后端逻辑 Review 识别出 5 个高危问题，其中 H3/H4/H5 与「水平扩展」目标直接冲突，H1/H2 涉及安全与日志泄露。本子项目仅修复这 5 个高危，中低危（M1–M6、L1–L8）留待后续。

## 目标

- H1：消除硬编码弱 JWT 密钥，增加强度校验与安全注入。
- H2：移除所有泄露密钥/残留调试的 `fmt` 输出。
- H3：Snowflake `node_id` 实例唯一化，避免多副本主键冲突。
- H4：视频删除链路 `ref_count` 只扣减一次。
- H5：Kafka 消息可靠投递，任务不丢失、失败可感知、`pending` 可兜底。

## 功能需求

- **F1（H1，JWT 密钥安全）**：新增配置校验——`jwt_secret` 为空、等于默认值 `"secret"` 或长度不足 16 时，`release` 模式拒绝启动、`debug/test` 模式告警；支持通过环境变量 `VISTACK_AUTH_JWT_SECRET` 注入（Viper 已支持该前缀）。

- **F2（H2，移除泄露/调试输出）**：删除 `internal/core/minio.go` 中打印完整 MinIO 配置（含 `secret_key`）的 `fmt.Printf`、`GetInternalBaseURL` 里的 `fmt.Printf("Internal")`；删除 `internal/api/v1/Video.go` 中 `fmt.Println("avatarURL:")` 调试输出。

- **F3（H3，Snowflake 唯一化）**：`InitSnowflake` 在未显式配置 `node_id` 时，从实例标识（`POD_IP`，回退 `os.Hostname()`）哈希派生 0–1023 的 `node_id`；显式配置仍优先。

- **F4（H4，ref_count 单一扣减）**：`internal/api/v1/Video.go` 的 `DeleteVideo` 只做「软删视频 + 投递 Kafka 删除消息」，移除其对 `files.ref_count` 的扣减与 `deleting` 标记；引用计数扣减与物理删除统一保留在 `delete_video_worker.go`。

- **F5（H5，Kafka 可靠投递）**：
  - `internal/core/kafka.go` 生产者改为同步发送（`Async: false`），使写失败可被调用方捕获。
  - `internal/api/v1/Video.go` 的 `CompleteVideoUpload`/`DeleteVideo` 处理发送错误：转码投递失败时标记转码任务失败并进入指数退避重试，删除投递失败时返回错误给客户端。
  - `internal/core/message_queue/transcode/watchdog.go` 扩展：扫描 `pending` 且超过阈值（10 分钟）的转码任务并重新投递，兜底「消息丢失导致永久 pending」。

## 非功能需求

- N1：不改变现有 API 契约与数据库 schema。
- N2：修复后三角色可水平扩容而不产生 ID 冲突（H3）。
- N3：配置校验逻辑集中在统一入口，错误信息清晰可操作。

## 不做的事

- 不做中低危问题（M1–M6、L1–L8）。
- 不引入账号锁定/登录限流/新鉴权中间件。
- 不新增数据库表或迁移。
- 不改变转码/上传/播放的既有行为。

## 验收标准

- **AC1（F1）**：`jwt_secret` 为弱值时，`release` 模式启动失败并给出清晰错误；`debug` 模式仅告警；设置 `VISTACK_AUTH_JWT_SECRET` 后覆盖生效。
- **AC2（F2）**：grep 后端代码无 `minioConfig`、`avatarURL`、`fmt.Printf("Internal")` 等泄露/调试输出。
- **AC3（F3）**：两个不同 hostname/POD_IP 的实例派生出不同的 `node_id`（同一实例稳定一致）。
- **AC4（F4）**：`DeleteVideo` 中不再出现 `ref_count` 扣减逻辑；扣减仅存在于 `delete_video_worker.go`。
- **AC5（F5）**：Kafka 生产者非 `Async`；`CompleteVideoUpload` 发送失败会进入重试队列；`DeleteVideo` 发送失败返回错误；watchdog 能重投超时的 `pending` 任务。
