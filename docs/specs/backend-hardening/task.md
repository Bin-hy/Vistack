# 后端高危问题修复（H1–H5）Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/core/validate.go` | 配置校验（JWT 密钥强度） |
| 修改 | `cmd/vistack/main.go` | 调用 ValidateConfig |
| 修改 | `internal/core/snowflake.go` | node_id 派生 |
| 修改 | `internal/core/kafka.go` | Async: false |
| 修改 | `internal/core/minio.go` | 删 2 处 fmt 输出 |
| 修改 | `internal/api/v1/Video.go` | 删调试输出、改 DeleteVideo、处理 Kafka 错误 |
| 修改 | `internal/core/message_queue/transcode/watchdog.go` | pending 兜底 |
| 修改 | `conf/app.toml`、`conf/app.docker.toml` | node_id=0 + jwt_secret 注释 |
| 修改 | `compose.yml`、`.env.example` | 注入 JWT_SECRET |

## T1: 配置校验（F1）

**文件：** `internal/core/validate.go`、`cmd/vistack/main.go`、`conf/app.toml`、`conf/app.docker.toml`、`compose.yml`、`.env.example`
**依赖：** 无
**步骤：**
1. 新建 `validate.go`：`ValidateConfig(cfg)`，弱密钥（空/`"secret"`/<16）时 release panic、否则 Warn
2. `main.go` 在 `InitLogger` 后调用 `core.ValidateConfig(&cfg)`
3. 两个 conf 的 `[auth]` 增加注释「可用 VISTACK_AUTH_JWT_SECRET 覆盖」
4. `compose.yml` api 服务加 `VISTACK_AUTH_JWT_SECRET: ${JWT_SECRET:-vistack-dev-secret-change-me}`
5. `.env.example` 加 `JWT_SECRET=`

**验证：** `go build ./...` 通过

## T2: 移除泄露/调试输出（F2）

**文件：** `internal/core/minio.go`、`internal/api/v1/Video.go`
**依赖：** 无
**步骤：**
1. 删 `minio.go` 的 `fmt.Printf("OnInitMinioClient, minioConfig: %+v", minioConfig)`（改为不含密钥的 Logger.Info）
2. 删 `minio.go` 的 `fmt.Printf("Internal")`
3. 删 `Video.go` 的 `fmt.Println("avatarURL:", avatarURL)`

**验证：** `go build ./...` 通过；`grep -rn "minioConfig\|avatarURL\|fmt.Printf(\"Internal\")"` 无命中

## T3: Snowflake 唯一化（F3）

**文件：** `internal/core/snowflake.go`、`conf/app.toml`、`conf/app.docker.toml`
**依赖：** 无
**步骤：**
1. `snowflake.go` 加 `deriveNodeID()`（POD_IP→hostname→fnv%1024），`InitSnowflake` 在 `NodeID<=0` 时使用
2. 两个 conf 的 `[snowflake] node_id` 改为 `0` 并注释「0=自动派生」

**验证：** `go build ./...` 通过

## T4: Kafka 同步发送（F5 基础）

**文件：** `internal/core/kafka.go`
**依赖：** 无
**步骤：**
1. `InitKafka` 中 `Async: true` 改为 `Async: false`

**验证：** `go build ./...` 通过

## T5: DeleteVideo 简化 + 错误处理（F4 + F5）

**文件：** `internal/api/v1/Video.go`
**依赖：** T4
**步骤：**
1. `DeleteVideo`：删除 ref_count 扣减循环、`deleting` 标记、事务，只保留「查视频+校验归属 → 软删 → 发 Kafka → 返回」
2. 发 Kafka 失败时记录日志并返回 500

**验证：** `go build ./...` 通过；grep `DeleteVideo` 函数内无 `ref_count`

## T6: 转码消息发送错误处理（F5）

**文件：** `internal/api/v1/Video.go`
**依赖：** T4
**步骤：**
1. `CompleteVideoUpload` 与 `InitVideoUpload`（秒传）的 `SendKafkaMessage` 失败时：标记 `video_transcodes` 为 `failed` + 调 `transcode.AddTranscodeRetry`，并返回 500

**验证：** `go build ./...` 通过

## T7: watchdog pending 兜底（F5）

**文件：** `internal/core/message_queue/transcode/watchdog.go`
**依赖：** 无
**步骤：**
1. 在现有 processing 扫描后，新增扫描 `status=pending AND updated_at < now-10min` 的任务
2. 对每个任务：查 source→file 得 ObjectKey，`Update("updated_at", now)` 去重，`AddTranscodeRetry` 重投

**验证：** `go build ./...` 通过

## T8: 收尾

**文件：** 全部
**依赖：** T1–T7
**步骤：**
1. `go build ./...`
2. `go vet ./...`

**验证：** build 与 vet 均无错误

## 执行顺序

```
T1 ─┐
T2 ─┤
T3 ─┼──> T8（收尾 build/vet）
T4 ─┼──> T5 ─┐
    └──> T6 ─┘
T7 ──────────┘
```
