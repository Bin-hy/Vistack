# FFmpeg 容器化与三角色拆分 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `proto/transcoder/v1/transcoder.proto` | gRPC 契约（ProcessVideo） |
| 新建 | `buf.yaml`、`buf.gen.yaml` | proto 生成配置 |
| 新建 | `internal/transcoder/pb/` | 生成代码（提交） |
| 新建 | `internal/transcoder/ffmpeg.go` | 迁移的 ffmpeg/ffprobe 逻辑 |
| 新建 | `internal/transcoder/service.go` | ProcessVideo 实现 |
| 新建 | `internal/transcoder/server.go` | gRPC server 组装 |
| 新建 | `internal/transcoder/registry/etcd.go` | etcd 注册 |
| 新建 | `internal/transcoder/client.go` | gRPC 客户端构造 |
| 新建 | `internal/discovery/resolver.go` | Resolver 接口 |
| 新建 | `internal/discovery/static.go` | 静态地址实现 |
| 新建 | `internal/discovery/etcd.go` | etcd resolver |
| 新建 | `internal/role/api.go`、`worker.go`、`transcoder.go` | 三角色启动 |
| 修改 | `cmd/vistack/main.go` | 角色分发入口 |
| 修改 | `internal/config/config.go`、`conf/app.toml` | 新增 Etcd/Transcoder 配置 |
| 修改 | `internal/core/message_queue/transcode/worker.go` | 编排改调 gRPC |
| 删除 | `internal/core/message_queue/transcode/ffmpeg.go` | 迁往 transcoder |
| 修改 | `Dockerfile` | 双 target（core/transcoder） |
| 修改 | `compose.yml`、`.env.example` | 新增三角色 + etcd |
| 新建 | `deploy/k8s/` | k8s 清单 |
| 修改 | `go.mod`、`go.sum` | 新增 grpc/protobuf/etcd 依赖 |

## T1: 新增依赖 + proto 定义 + 生成代码

**文件：** `go.mod`、`proto/transcoder/v1/transcoder.proto`、`buf.yaml`、`buf.gen.yaml`
**依赖：** 无
**步骤：**
1. `go get google.golang.org/grpc google.golang.org/protobuf go.etcd.io/etcd/client/v3`
2. 按 plan 编写 `transcoder.proto`（service + Request/Response/QualityProfile）
3. 编写 `buf.yaml`（module + lint）与 `buf.gen.yaml`（`go` + `go-grpc` 插件，`out: internal/transcoder/pb`）
4. `go run github.com/bufbuild/buf/cmd/buf generate` 生成 pb 代码

**验证：** `go build ./internal/transcoder/pb/...` 通过；`buf lint` 无错误

## T2: 配置新增 Etcd/Transcoder

**文件：** `internal/config/config.go`、`conf/app.toml`
**依赖：** 无
**步骤：**
1. `AppConfig` 新增 `Etcd{Endpoints, Prefix}`、`Transcoder{ListenAddr, Addr, UseEtcd}`
2. `conf/app.toml` 新增 `[etcd]`、`[transcoder]` 段落，给出本地默认值

**验证：** `go build ./internal/config/...` 通过

## T3: 迁移 ffmpeg 逻辑到 transcoder

**文件：** `internal/transcoder/ffmpeg.go`
**依赖：** 无
**步骤：**
1. 将 `DashQuality`、`standard169Resolutions`、`allQualities`、`VideoStream`、`ProbeResult`、`GetVideoResolution`、`GetVideoDuration`、`ExtractVideoFrame`、`SelectAdaptiveQualities`、`filterQualities`、`TranscodeToDASH` 复制到新文件，包名改为 `transcoder`
2. 保留函数签名不变；`fmt.Printf` 调试输出改为结构化日志（zap）

**验证：** `go build ./internal/transcoder/...` 通过

## T4: 实现 ProcessVideo 服务

**文件：** `internal/transcoder/service.go`
**依赖：** T1、T3
**步骤：**
1. 实现 `ProcessVideo`：`FGetObject` 下载原片到临时目录
2. `ffprobe` 探时长；`cover_time_seconds<=0` 时按原逻辑自动选（>10s→5s，>2s→一半，否则 1s）
3. 抽封面帧并 `FPutObject` 到 `cover_object_key`
4. `quality_heights` 为空则自动选档，否则按指定高度过滤档位
5. `TranscodeToDASH` → 遍历输出目录 `FPutObject` 到 `output_prefix`
6. 组装 `ProcessVideoResponse`；`defer os.RemoveAll(tempDir)`

**验证：** `go build ./internal/transcoder/...` 通过

## T5: etcd 注册

**文件：** `internal/transcoder/registry/etcd.go`
**依赖：** T1
**步骤：**
1. 实现 `Register(ctx, id, addr) (io.Closer, error)`：`Grant` 10s 租约、`Put {prefix}/{id}=addr`、每 3s `KeepAliveOnce`
2. `Close()` 撤销租约并删除 key

**验证：** `go build ./internal/transcoder/registry/...` 通过

## T6: gRPC server 组装

**文件：** `internal/transcoder/server.go`
**依赖：** T4、T5
**步骤：**
1. `NewServer(cfg)` 构造 `grpc.Server` 并注册 `TranscoderService`
2. 监听 `cfg.Transcoder.ListenAddr`，启动前调用 registry 注册自身地址
3. 优雅退出时反注册

**验证：** `go build ./internal/transcoder/...` 通过

## T7: 服务发现实现

**文件：** `internal/discovery/resolver.go`、`static.go`、`etcd.go`
**依赖：** T1
**步骤：**
1. 定义 `Resolver{ Targets(ctx) ([]string,error); Close() error }`
2. `NewStaticResolver(addr)` 直接返回配置地址
3. `NewEtcdResolver(client, prefix)`：watch 前缀、维护实时地址集合；实现 gRPC `resolver.Builder`（scheme `etcd`），`cc.UpdateState` + `round_robin`
4. 提供 `Dial(ctx, r Resolver) (*grpc.ClientConn, error)` 帮助函数

**验证：** `go build ./internal/discovery/...` 通过

## T8: worker 编排重构

**文件：** `internal/core/message_queue/transcode/worker.go`（改）、`ffmpeg.go`（删）
**依赖：** T1、T7
**步骤：**
1. 删除本地下载/ffprobe/抽帧/转码/上传代码，改为：幂等检查 → 租约 → processing → `Resolver` + gRPC 调 `ProcessVideo`
2. `ProcessVideoRequest` 传 `bucket`、`object_key`、`output_prefix=dash/{video_id}`、`cover_object_key=covers/{video_id}.jpg`
3. 用响应写 DB：manifest file、transcode（resolution/codec/status）、VideoManifest（profiles）、cover file、video（duration/cover_file_id/status=published）
4. 失败路径与 retry/watchdog 保持不变；删除 `internal/core/message_queue/transcode/ffmpeg.go`

**验证：** `go build ./...` 通过；`go vet ./internal/core/message_queue/transcode/...` 通过

## T9: 角色分发

**文件：** `cmd/vistack/main.go`（改）、`internal/role/{api,worker,transcoder}.go`（新）
**依赖：** T2、T6、T7、T8
**步骤：**
1. `main.go` 读取 `VISTACK_ROLE`（缺省用 `os.Args[1]`，再缺省 `api`），分发到对应 `Run*`
2. `RunAPI`：Viper/Logger/DB/Redis/MinIO/Snowflake/Kafka 生产者/TokenManager → Gin
3. `RunWorker`：上述 + Kafka 消费者 + etcd 客户端 + discovery resolver + 启动 transcode 消费者/删除消费者/重试/watchdog
4. `RunTranscoder`：Viper/Logger/MinIO/etcd → gRPC server（不初始化 DB/Kafka/Redis 消费者）

**验证：** `go build ./...` 通过；`go run . api` 能启动 HTTP（日志确认无消费者/gRPC）

## T10: Dockerfile 双 target

**文件：** `Dockerfile`
**依赖：** T9
**步骤：**
1. 多阶段构建二进制（保留现有 build stage）
2. target `vistack`：`alpine` + `ca-certificates` + 二进制，`ENTRYPOINT ["/app/vistack"]`
3. target `vistack-transcoder`：`alpine` + `apk add ffmpeg ca-certificates` + 二进制

**验证：** `docker build --target vistack-transcoder -t vistack-transcoder .` 成功（若本地有 docker）

## T11: compose 编排

**文件：** `compose.yml`、`.env.example`
**依赖：** T10
**步骤：**
1. 新增 `api`（vistack，`command: api`）、`worker`（vistack，`command: worker`）、`transcoder`（vistack-transcoder，`command: transcoder`）、`etcd`（单节点）
2. 为 api/worker/transcoder 注入 DB/Redis/MinIO/Kafka/etcd 环境变量；`transcoder` 用 `deploy.replicas` 演示多副本
3. `.env.example` 补齐 `VISTACK_ROLE`、etcd、transcoder 地址等键

**验证：** `docker compose config` 校验通过（若本地有 docker compose）

## T12: k8s 清单

**文件：** `deploy/k8s/`
**依赖：** T10、T11
**步骤：**
1. `configmap.yaml`（app.toml 内容）、`secret.yaml`（MinIO/DB 凭证）
2. `api/worker/transcoder` 的 `deployment.yaml` + `service.yaml`；transcoder 用 `Deployment.spec.replicas` 扩容
3. `etcd.yaml`（StatefulSet 或单 Pod Deployment + headless Service）

**验证：** `kubectl apply --dry-run=client -f deploy/k8s/` 通过（若本地有 kubectl）

## T13: 收尾构建

**文件：** `go.mod`、`go.sum`、`.env.example`
**依赖：** T1–T9
**步骤：**
1. `go mod tidy`
2. `go build ./...`
3. `go vet ./...`

**验证：** `go build ./...`、`go vet ./...` 均无错误

## 执行顺序

```
T1 ──┬──> T4 ──> T6 ──┐
     │                 │
T2 ──┼──────────────> T9 ──> T10 ──> T11 ──> T12
     │                 │
T3 ──┘                 │
T5 ────────> T6 ───────┤
T7 ────────> T8 ───────┘

T13 收尾（依赖 T1–T9 全部完成后执行）
```
