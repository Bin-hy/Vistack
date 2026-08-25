# FFmpeg 容器化与三角色拆分 Spec

## 背景

当前转码链路把 FFmpeg 当作宿主机命令直接调用：`internal/core/message_queue/transcode/ffmpeg.go` 通过 `os/exec` 执行 `ffmpeg` / `ffprobe`，`worker.go` 在本地临时目录下载原片、抽帧、DASH 转码、并发上传 MinIO。存在三个问题：

1. **依赖宿主机 FFmpeg**：`Dockerfile` 构建的镜像未安装 FFmpeg，`compose.yml` 只有 postgres/redis/minio/kafka 四个基础设施服务、没有应用容器，因此容器化部署根本无法执行转码。
2. **单体进程耦合**：`cmd/vistack/main.go` 把 API 服务、转码 Worker、重试派发器、Watchdog、视频删除 Worker 全部塞进一个进程，任何一个角色要扩容都得整套起，无法按负载独立水平扩展。
3. **FFmpeg 执行位置不可寻址**：转码写死在本机 `exec`，无法把 FFmpeg 当作一个可寻址、可发现的远程服务，也无法平滑过渡到 etcd + k8s 的分布式部署。

## 目标

- 把 FFmpeg 隔离到独立容器，作为远程 gRPC 转码服务，主程序通过「配置的地址 + etcd 服务发现」调用，而不是 `exec` 本机二进制。
- 按职责拆成三个可独立部署、独立扩容的角色：`api` / `worker` / `transcoder`。
- 保留现有转码行为（ABR 档位、封面抽取、DASH 分片命名、重试与 watchdog 语义）不变，避免前端播放端改动。
- 交付 docker-compose 与 Kubernetes 两种部署方式，并集成 etcd 服务发现。

## 功能需求

- **F1（三角色启动）**：单一二进制通过启动命令选择角色，支持 `api`、`worker`、`transcoder` 三种模式：
  - `api`：提供 HTTP 接口（上传、预签名 URL、视频信息、推荐等），负责投递转码/删除 Kafka 消息，不消费 Kafka、不执行 FFmpeg。
  - `worker`：消费 `transcode` 与 `delete_file` 两个 Kafka topic，编排转码任务、写数据库，并承载重试派发器与 watchdog，不直接执行 FFmpeg。
  - `transcoder`：提供 gRPC 转码服务，封装 ffprobe/ffmpeg，读写 MinIO，无状态、不持有数据库连接。

- **F2（gRPC 转码契约）**：`transcoder` 暴露单个 `ProcessVideo` RPC。入参为原始视频的 MinIO object key、输出前缀、封面 object key、封面时间点、质量档位（可空，空则自动选择）；转码产物直接写回 MinIO。返回视频时长、封面 object key 与大小、DASH 清单 object key 与大小、实际质量档位列表。

- **F3（etcd 服务发现）**：`transcoder` 启动后向 etcd 注册自身 gRPC 地址并通过租约保活；`worker` 通过 etcd 监听并维护 transcoder 实例列表，对多个实例做负载均衡调用；实例下线后自动从可用列表移除。

- **F4（转码行为兼容）**：FFmpeg 参数、ABR 档位选择、封面抽取时间点、DASH 分片命名与现有实现保持一致。转码产物目录结构仍为 `dash/{video_id}/manifest.mpd` + 分片、`covers/{video_id}.jpg`，数据库写入字段（transcode 状态/分辨率/编码、manifest、cover、video 状态与时长）与现在等价。

- **F5（重试与看护迁移）**：现有「Redis ZSet 指数退避重试 + 处理中超时 watchdog」逻辑保留，但从「重投后由 worker 本地跑 FFmpeg」变为「重投后由 worker 调 gRPC transcoder」。转码失败（gRPC 错误/超时）按现有规则计数重试，超过上限丢弃。

- **F6（部署产物）**：提供两套部署：
  - `docker-compose`：`api`、`worker`、`transcoder` 三个应用服务 + etcd + 现有基础设施（postgres/redis/minio/kafka）。
  - `k8s` 清单：`api`、`worker`、`transcoder` 的 Deployment 与 Service，以及 etcd 部署，应用配置以 ConfigMap/Secret 注入。

- **F7（配置化地址）**：新增 transcoder 地址与 etcd 端点等配置项，支持通过环境变量/配置文件注入；本地开发与容器/k8s 部署共用同一套配置结构。

## 非功能需求

- **N1（无状态 transcoder）**：transcoder 不依赖本地文件与本地状态，任意副本可处理任意任务（输入输出都走 MinIO），便于水平扩容。
- **N2（故障兜底）**：transcoder 崩溃或重启不影响任务状态机，由 worker 侧 watchdog 与重试兜底。
- **N3（兼容性）**：转码输出格式、对象路径、数据库字段与现有实现兼容，前端播放端与上传端无需改动。
- **N4（可观测性）**：三角色统一使用 zap 日志，gRPC 调用带超时与重试，转码关键阶段有结构化日志。
- **N5（最小权限）**：transcoder 仅持有 MinIO 与 etcd 所需的连接信息，不持有数据库（PostgreSQL）凭证。
- **N6（配置一致）**：docker-compose 与 k8s 使用同一套环境变量键名注入配置。

## 不做的事

- 不做转码进度实时上报（本次为单次 unary RPC，server-streaming 进度流式留待后续）。
- 不重写 FFmpeg 命令参数本身（保持现有 ABR/编码/GOP 参数），只迁移执行位置。
- 不改动数据库 schema（不新增迁移）。
- 不做 etcd 集群高可用（本次单节点 etcd，集群模式留后续）。
- 不做 gRPC 的 mTLS 鉴权（内部调用暂明文，留后续）。
- 不做 HLS 兼容输出（现有代码本就只产 DASH）。
- 不做 Prometheus/Grafana 监控接入（README 提及的监控不在本子项目范围）。

## 验收标准

- **AC1（对应 F1）**：`api`、`worker`、`transcoder` 三种模式均可独立启动；`transcoder` 模式下不建立数据库连接、不启动 HTTP 服务；`api` 模式下不启动 Kafka 消费者与 gRPC 服务。
- **AC2（对应 F2）**：给定一个已上传到 MinIO 的原始视频 object key，调用 `ProcessVideo` 后，MinIO 中出现 `dash/{video_id}/manifest.mpd` 与分片、`covers/{video_id}.jpg`，返回的时长/清单 key/封面 key/档位与实际产物一致。
- **AC3（对应 F3）**：启动 2 个 transcoder 实例后，etcd 中可见两条注册记录；关闭 1 个实例，其记录在租约到期后消失，worker 后续调用只落在存活实例上。
- **AC4（对应 F4）**：对同一原始视频，新旧实现产出的 DASH 清单档位、分片命名、封面路径一致；数据库最终状态（transcode=completed、manifest、cover、video=published、duration）等价。
- **AC5（对应 F5）**：人为让 transcoder 返回错误/超时，worker 按指数退避重试；同一任务重试超过上限后被丢弃且不再重投。
- **AC6（对应 F6）**：`docker compose up` 后三应用服务 + 基础设施全部健康；k8s 清单可 apply 并让三应用 Deployment 就绪。
- **AC7（对应 F7）**：通过环境变量改变 transcoder 地址与 etcd 端点后，worker 能连接到新的 transcoder 实例，无需重新编译。
