# FFmpeg 容器化与三角色拆分 Plan

## 架构概览

单一 Go 二进制通过启动参数/环境变量选择角色，拆为三个可独立部署、独立扩容的进程：

- **`api`**：Gin HTTP 服务，负责上传（分片/预签名 URL）、视频信息、推荐、删除等接口，并向 Kafka 投递 `transcode` / `delete_file` 消息。只初始化 Viper、日志、DB、Redis、MinIO、Snowflake、Kafka 生产者、TokenManager，**不**启动 Kafka 消费者、gRPC、etcd、watchdog、重试派发器。
- **`worker`**：消费 `transcode` / `delete_file` 两个 topic，编排转码与删除；承载重试派发器（Redis ZSet 指数退避）与 watchdog（处理中超时重投）。转码时通过 etcd 发现的 transcoder gRPC 客户端调用 `ProcessVideo`，成功后写数据库。初始化 DB、Redis、MinIO、Kafka（生产者+消费者）、etcd、gRPC 客户端。
- **`transcoder`**：gRPC 服务，封装 ffprobe/ffmpeg，读写 MinIO，无状态、**不**初始化数据库。启动后向 etcd 注册自身地址并保活。初始化 Viper、日志、MinIO、etcd、gRPC 服务端。

三者通过 Kafka（任务分发）+ etcd（服务发现）+ gRPC（转码执行）+ MinIO（数据传递）协作，形成「上传 → 入队 → 编排 → 远程转码 → 回写」的分布式流水线。

## 核心数据结构

### gRPC 契约（`proto/transcoder/v1/transcoder.proto`）

```proto
syntax = "proto3";
package vistack.transcoder.v1;
option go_package = "github.com/binhy/vistack/internal/transcoder/pb;transcoderpb";

service TranscoderService {
  rpc ProcessVideo(ProcessVideoRequest) returns (ProcessVideoResponse);
}

message QualityProfile {
  int32  height     = 1;  // 档位高度，如 720
  string resolution = 2;  // 展示名，如 "720p"
}

message ProcessVideoRequest {
  string  bucket             = 1;  // MinIO 桶名
  string  object_key         = 2;  // 原始视频 object key（如 raw/xxx.mp4）
  string  output_prefix      = 3;  // 输出前缀（如 dash/{video_id}）
  string  cover_object_key   = 4;  // 封面 object key（如 covers/{video_id}.jpg）
  double  cover_time_seconds = 5;  // 抽帧时间点；0 表示自动
  repeated int32 quality_heights = 6; // 空表示由 transcoder 自动选择 ABR 档位
}

message ProcessVideoResponse {
  double  duration_seconds    = 1;
  string  manifest_object_key = 2;
  int64   manifest_size       = 3;
  string  cover_object_key    = 4;  // 空表示未生成封面
  int64   cover_size          = 5;
  repeated QualityProfile profiles = 6;
}
```

### 配置结构（`internal/config/config.go` 新增）

```go
Etcd struct {
    Endpoints []string `mapstructure:"endpoints"` // 如 ["etcd:2379"]
    Prefix    string   `mapstructure:"prefix"`    // 注册前缀，默认 /vistack/transcoders
}
Transcoder struct {
    ListenAddr string `mapstructure:"listen_addr"` // transcoder 绑定，如 :50051
    Addr       string `mapstructure:"addr"`        // worker 的静态兜底地址
    UseEtcd    bool   `mapstructure:"use_etcd"`    // worker 是否走 etcd 发现
}
```

### 服务发现接口（`internal/discovery/resolver.go`）

```go
// Resolver 解析 transcoder 的可用 gRPC 地址列表
type Resolver interface {
    Targets(ctx context.Context) ([]string, error)
    Close() error
}
```

- 静态实现：直接返回配置的 `Transcoder.Addr`。
- etcd 实现：基于 `go.etcd.io/etcd/client/v3` 的 `Watch` 维护 `transcoder/` 前缀下的实时地址列表，并接入 gRPC `resolver.Builder` + `round_robin` 做客户端负载均衡。

## 模块设计

### 角色启动层（`internal/role`）
**职责：** 解析角色（`VISTACK_ROLE` 环境变量或 `os.Args[1]`），调用对应初始化并启动。
**对外接口：** `RunAPI()`、`RunWorker()`、`RunTranscoder()`。
**依赖：** `internal/core`（Viper/Logger/DB/Redis/MinIO/Kafka/Snowflake）、`internal/transcoder`、`internal/discovery`。

### transcoder 服务（`internal/transcoder`）
**职责：** 实现 `ProcessVideo`。流程：从 MinIO `FGetObject` 下载原片到临时目录 → `ffprobe` 探时长 → 抽封面帧并 `FPutObject` 上传 → 按（可选）档位或自动选择执行 DASH 转码 → 遍历产物 `FPutObject` 上传到 `output_prefix` → 返回结果。
**对外接口：** `TranscoderService` gRPC 实现；`NewServer(...)` 构造 gRPC server。
**依赖：** `internal/config`（MinIO）、`internal/pb`（生成代码）、etcd 客户端（注册）。**不依赖 DB。**
**迁移内容：** `ffmpeg.go` 中的 `GetVideoResolution` / `GetVideoDuration` / `ExtractVideoFrame` / `SelectAdaptiveQualities` / `TranscodeToDASH` 及 `DashQuality`、分辨率表、档位表整体迁入本模块。

### etcd 注册（`internal/transcoder/registry`）
**职责：** transcoder 启动后以 `{prefix}/{instanceID}` 为 key、`{addr}` 为 value 注册，租约 TTL 10s、每 3s keepalive；退出时撤销。
**对外接口：** `Register(ctx, id, addr) (io.Closer, error)`。
**依赖：** etcd client/v3。

### 服务发现（`internal/discovery`）
**职责：** 实现 `Resolver` 接口的静态与 etcd 两种实现；etcd 实现将前缀下的实例集合以 gRPC resolver 形式暴露给 worker 的 gRPC 客户端。
**对外接口：** `NewStaticResolver(addr)`、`NewEtcdResolver(client, prefix)`。
**依赖：** etcd client/v3、`google.golang.org/grpc`。

### worker 编排（`internal/core/message_queue/transcode`）
**职责：** `handleTranscodeMessage` 改为：幂等检查 → Redis 租约 → 置 processing → 通过 `Resolver` + gRPC 客户端调 `ProcessVideo` → 成功后事务写 DB（transcode=completed、manifest、cover、video=published、duration）→ 清理重试计数；失败则标记 failed 并按指数退避重试。
**对外接口：** `StartTranscodeWorker`、`StartTranscodeRetryDispatcher`、`StartTranscodeWatchdog`。
**依赖：** DB、Redis、Kafka、`internal/discovery`、`internal/pb`。
**保留：** `retry.go`（指数退避）、`watchdog.go`（超时重投）逻辑不变。
**删除：** `ffmpeg.go` 中的 `exec` 转码逻辑（迁往 transcoder）。

### 配置模块（`internal/config`）
**职责：** 在 `AppConfig` 中新增 `Etcd`、`Transcoder` 结构，`conf/app.toml` 新增 `[etcd]`、`[transcoder]` 段落；环境变量通过 `VISTACK_` 前缀注入。

## 模块交互

### 转码主链路
```
前端上传 → api(CompleteVideoUpload) → Kafka[transcode]
  → worker 消费 → 置 processing → Resolver(etcd) 取 transcoder 地址
  → gRPC ProcessVideo
      → transcoder: MinIO 下载原片 → ffprobe → 抽封面 → DASH 转码 → MinIO 上传
  → 返回(时长/清单key/封面key/档位) → worker 事务写 DB → 视频 published
```

### 失败重试
```
worker 收到 gRPC 错误/超时 → 置 failed → Redis ZSet 指数退避
  → 到期重投 Kafka[transcode] → 重试；超上限丢弃
```

### 超时兜底
```
watchdog(每 1min) → 查 processing 且 updated_at<15min 且无租约的任务 → 重投 Kafka[transcode]
```

### 删除链路
```
api(DeleteVideo) → Kafka[delete_file] → worker 删除消费者 → 软删/减引用 → MinIO 清理 dash/{video_id} 与文件对象 → DB 物理删
```

## 文件组织

```
cmd/vistack/main.go                     修改：改为角色分发入口
internal/role/
  ├── api.go                            新增：api 角色启动
  ├── worker.go                         新增：worker 角色启动
  └── transcoder.go                     新增：transcoder 角色启动
internal/config/config.go               修改：新增 Etcd/Transcoder 结构
conf/app.toml                           修改：新增 [etcd]/[transcoder]
proto/transcoder/v1/transcoder.proto    新增：gRPC 契约
buf.yaml / buf.gen.yaml                 新增：proto 生成配置
internal/transcoder/
  ├── pb/                               新增：生成代码（提交入库）
  ├── service.go                        新增：ProcessVideo 实现
  ├── ffmpeg.go                         迁移：ffmpeg/ffprobe 逻辑
  ├── server.go                         新增：gRPC server 组装
  └── registry/etcd.go                  新增：etcd 注册
internal/discovery/
  ├── resolver.go                       新增：Resolver 接口
  ├── static.go                         新增：静态实现
  └── etcd.go                           新增：etcd resolver
internal/core/message_queue/transcode/
  ├── worker.go                         修改：编排调 gRPC
  ├── retry.go                          保留
  ├── watchdog.go                       保留
  └── ffmpeg.go                         删除（迁往 transcoder）
Dockerfile                              修改：多阶段双 target（core/transcoder）
compose.yml                             修改：新增 api/worker/transcoder/etcd
deploy/k8s/                             新增：ConfigMap/Secret/Deployment/Service/etcd
.env.example                            修改：新增 etcd/transcoder 环境变量
go.mod                                  修改：新增 grpc/protobuf/etcd 依赖
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 角色分发 | `VISTACK_ROLE` 环境变量 + `os.Args[1]` | 简单直观，Docker `command` 与 k8s `args` 都可直接指定 |
| 调用接口 | gRPC + protobuf（单 unary RPC） | 强类型、长任务友好、与 etcd 发现天然契合；进度流式留后续 |
| proto 生成 | buf（`buf.gen.yaml`），生成代码提交 | 可复现、免手工维护 pb 文件；提交生成代码避免 CI 重复生成 |
| 服务发现 | etcd client/v3 + gRPC resolver + round_robin | 标准模式，实例动态上下线自动收敛，k8s 下同样适用 |
| Docker 镜像 | 双 target：`vistack`（api/worker）与 `vistack-transcoder`（内置 ffmpeg） | 满足 N5 最小权限，api/worker 镜像不携带 ffmpeg |
| transcoder 数据传递 | 直接读写 MinIO（不共享文件卷） | 无状态、k8s 任意副本可接任务，满足 N1 |
| 档位选择位置 | 留在 transcoder（ffprobe 在此） | 内聚，避免跨服务重复探测 |
| 重试/watchdog | 保留在 worker | 需要 DB/Kafka/Redis，属编排职责 |
| 消息类型 | `TranscodeMessage`/`VideoDeleteMessage` 暂留原包 | 最小改动；api 已依赖该包，后续再收敛 |
