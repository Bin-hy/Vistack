# 后端高危问题修复（H1–H5）Plan

## 架构概览

5 个修复分散在配置校验、核心基础设施（Kafka/Snowflake/MinIO）、API 层与转码编排层，均为**定点小改**，不引入新服务、不改表结构。按依赖关系分三组推进：先基础（F1 校验 + F2 清理 + F3 Snowflake），再 API 层（F4 删除 + F5 发送错误处理），最后兜底（F5 watchdog）。

## 模块设计

### F1 — 配置校验（`internal/core/validate.go`，新增）
**职责：** 提供 `ValidateConfig(cfg *config.AppConfig)`，校验 `Auth.JWTSecret`。
**行为：** `jwt_secret` 为空、等于 `"secret"` 或长度 < 16 时：`Server.Mode == "release"` 则 `panic`（带可操作错误信息），否则 `Logger.Warn`。
**调用：** `cmd/vistack/main.go` 在 `InitLogger` 之后、角色分发之前调用。
**注入：** 复用 Viper `VISTACK_` 前缀（`VISTACK_AUTH_JWT_SECRET` 覆盖 `auth.jwt_secret`），无需改 Viper 代码。

### F2 — 清理泄露/调试输出
**`internal/core/minio.go`**：删除 `InitMinioClient` 中 `fmt.Printf("OnInitMinioClient, minioConfig: %+v", minioConfig)`（泄露 `secret_key`），改为不含密钥的 `Logger.Info`；删除 `GetInternalBaseURL` 中 `fmt.Printf("Internal")`。
**`internal/api/v1/Video.go`**：删除 `GetVideoInfo` 中 `fmt.Println("avatarURL:", avatarURL)`。

### F3 — Snowflake 唯一化（`internal/core/snowflake.go`）
**职责：** `InitSnowflake` 中，若 `cfg.Snowflake.NodeID <= 0`，调用 `deriveNodeID()` 派生。
**派生：** 取 `POD_IP`（回退 `os.Hostname()`）→ `fnv` 哈希 → `% 1024`。同一实例稳定、不同实例大概率不同。
**配置：** `conf/app.toml` / `app.docker.toml` 的 `node_id` 改为 `0`（语义：0 = 自动派生），注释说明；显式 1–1023 仍可覆盖。

### F4 — ref_count 单一扣减（`internal/api/v1/Video.go`）
`DeleteVideo` 简化为：查视频 + 校验归属 → 单条 `Update("status", "deleted")` 软删 → 投递 Kafka → 返回。删除原 `ref_count` 扣减循环、`deleting` 标记与事务。引用计数扣减与物理删除保留在 `delete_video_worker.go`。

### F5 — Kafka 可靠投递
**`internal/core/kafka.go`**：`Async: false`（同步写，失败由 `WriteMessages` 返回）。
**`internal/api/v1/Video.go`**：
- `CompleteVideoUpload` / `InitVideoUpload`(秒传) 投递转码消息失败时：标记 `video_transcodes` 为 `failed` + 调用 `transcode.AddTranscodeRetry`（进入指数退避重试），并返回 500。
- `DeleteVideo` 投递删除消息失败时：记录日志 + 返回 500（软删已生效，重试即可）。
**`internal/core/message_queue/transcode/watchdog.go`**：新增 `pending` 兜底——扫描 `status=pending AND updated_at < now-10min` 的任务，查 source→file 得到 ObjectKey，`Update("updated_at", now)` 防重复，再 `AddTranscodeRetry` 重投。

## 模块交互

```
main → ValidateConfig(F1) → 角色启动
api 启动 → InitSnowflake(F3 派生 node_id)
上传完成/删除 → SendKafkaMessage(同步) ──失败──> 标记 failed + AddTranscodeRetry / 返回 500
watchdog(每 1min) ──> processing 超时兜底 + pending 超时兜底 → AddTranscodeRetry → 重投
```

## 文件组织

```
cmd/vistack/main.go                              修改：调用 ValidateConfig
internal/core/validate.go                        新增：配置校验
internal/core/snowflake.go                       修改：node_id 派生
internal/core/kafka.go                           修改：Async: false
internal/core/minio.go                           修改：删 2 处 fmt.Printf
internal/api/v1/Video.go                         修改：删 avatarURL print、改 DeleteVideo、处理 Kafka 错误
internal/core/message_queue/transcode/watchdog.go 修改：pending 兜底
conf/app.toml                                    修改：node_id=0 + jwt_secret 注释
conf/app.docker.toml                             修改：node_id=0 + jwt_secret 注释
compose.yml                                      修改：api 注入 VISTACK_AUTH_JWT_SECRET
.env.example                                     修改：新增 JWT_SECRET
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 校验位置 | `internal/core/validate.go` | 需访问 `Logger`，且与 config 无循环依赖 |
| 弱密钥判定 | 空 / `"secret"` / <16 字符 | 覆盖默认值与明显弱值 |
| Snowflake 派生 | fnv(hostname/POD_IP) % 1024 | 无外部依赖、稳定、跨实例大概率唯一 |
| ref_count 职责 | 收敛到 worker 单点 | 消除双重扣减 |
| Kafka 模式 | 同步（Async:false） | 本项目吞吐小，可靠性优先 |
| pending 去重 | 触碰 `updated_at` | 复用现有字段，避免额外 Redis key |
