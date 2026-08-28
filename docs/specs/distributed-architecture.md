# Vistack 分布式架构演进设计

> 状态：设计稿 · 基于当前代码（三角色拆分 + Kafka + gRPC + etcd 已完成第一版）
> 目标：把"骨架分布式"演进为"可水平扩展、高可用、可观测的分布式系统"

---

## 1. 现状盘点：已经具备的分布式能力

| 维度 | 现状 | 代码位置 |
|------|------|----------|
| 角色拆分 | `api` / `worker` / `transcoder` 单二进制三角色，独立扩容 | `cmd/vistack/main.go`、`internal/role/` |
| 异步解耦 | Kafka `transcode` / `delete_file` 两个 topic，消费者组共享 | `internal/core/kafka.go`、`internal/consts/Kafka.go` |
| 远程计算 | FFmpeg 隔离在 transcoder 容器，gRPC `ProcessVideo`，无状态 | `internal/transcoder/` |
| 服务发现 | transcoder 向 etcd 注册 + 租约保活；worker 经 etcd 动态发现 + `round_robin` | `internal/transcoder/registry/etcd.go`、`internal/discovery/etcd.go` |
| 幂等 | Redis `SetNX` 租约防重复处理；DB 状态先行校验 | `worker.go` 中 `lease:transcode:*` |
| 重试/兜底 | Redis ZSet 延迟队列指数退避；Watchdog 超时重投 | `retry.go`、`watchdog.go` |
| ID 生成 | Snowflake，node_id 自动从 hostname/POD_IP 派生 | `internal/core/snowflake.go` |
| 存储 | MinIO S3 + 预签名上传 + STS 播放鉴权（带宽卸载到对象存储） | `internal/api/v1/Video.go`、`pkg/storage/minio.go` |
| 部署 | Docker Compose 一键起栈；附 K8s 清单（api/worker/transcoder/etcd/configmap） | `compose.yml`、`deploy/k8s/` |

**结论：架构骨架正确，方向对。当前处于"单副本可跑通、多副本会出错"的阶段。**

---

## 2. 欠缺的部分（按优先级）

### 2.1 P0 正确性 —— 多副本一开就出错的点（最紧急）

#### ① 单例任务没有领导选举（最严重）
`retry.go` 的 `StartTranscodeRetryDispatcher` 和 `watchdog.go` 的 `StartTranscodeWatchdog` 在**每个 worker 实例里都会启动**。worker 一扩容：
- 多个 dispatcher 同时扫同一个 Redis ZSet → **同一重试消息被多个实例重复投递**；
- 多个 watchdog 同时扫 DB → 重复打 `updated_at`、重复入重试队列。

Redis 租约只防了"转码任务本身"的重复执行，防不了"调度器/看门狗"的重复。
**解法：etcd 领导选举（见 4.2），选举一个 leader 实例运行这两个单例循环。**

#### ② AutoMigrate 多副本竞争
`role/api.go` 里每个 API 实例启动都跑 `migrations.AutoMigrate`。API 一扩容，多个实例同时建表/加列会互相竞争、偶发失败。
**解法：把 migration 拆成独立的一次性任务（K8s Job / compose run once），应用启动只校验连接不迁移。**

#### ③ Kafka 消费并发 = 1（吞吐瓶颈）
`core.StartKafkaConsumer` 是**单 goroutine 顺序循环**：`ReadMessage → handler → Commit`，handler 里的 gRPC 转码是阻塞的。意味着：
```
并发转码数 = min(topic 分区数, worker 副本数) × 1
```
一个 worker 同时只能转一个视频。增加分区 + worker 是解法之一，但更本质的是**worker 内部要支持并发消费**（见 3.3）。

#### ④ worker 无优雅停机
`role/worker.go` 最后是 `select {}`，不监听信号。滚动发布时：进程被 SIGTERM 直接杀死 → Kafka 消费者组 rebalance → 消息重复或延迟。
**解法：`signal.NotifyContext` + 停止消费者 + 排空在途任务后再退出。**

#### ⑤ API 无优雅停机
`r.Run(addr)` 直接阻塞，没有 `http.Server.Shutdown(ctx)`。滚动发布时在途请求被掐断。
**解法：`http.Server` + `Shutdown` + 等待在途请求完成。**

### 2.2 P1 高可用 —— 中间件全部单点

| 组件 | 现状 | 生产化方向 |
|------|------|------------|
| PostgreSQL | 单实例 | Patroni 主从 + PgBouncer 连接池 / 云 RDS（读写分离） |
| Redis | 单实例 | Redis Sentinel 或 Cluster |
| Kafka | 单 broker（KRaft 单节点） | 3 broker + 分区副本因子 ≥ 2（`transcode` topic 建议 8~16 分区） |
| MinIO | 单实例 | 分布式 MinIO（≥4 节点纠删码） |
| etcd | 单节点 | 3~5 节点集群 + TLS（README roadmap 已列） |

另外 k8s 清单缺失：
- **无 readiness/liveness 探针**（/health 已存在但没接探针）；
- 无 resource requests/limits、无 HPA（API 按 CPU、worker 按队列深度）；
- `api.yaml` 只有 ClusterIP Service，无 Ingress/网关入口定义；
- 凭证明文写在 ConfigMap，未用 Secret。

### 2.3 P2 可观测性 —— 分布式排障的前提

- 只有 zap 结构化日志：无 **Prometheus metrics**（HTTP QPS/延迟、Kafka lag、队列深度、转码时长/成功率）、无 **OpenTelemetry 链路追踪**（一次上传 → Kafka → worker → gRPC → MinIO 的调用链无法串联）、无日志聚合（Loki/ELK）。
- `/health` 恒返回 200 且不区分 **readiness（依赖是否就绪）** 与 **liveness（进程是否存活）**，K8s 探针无从区分。

### 2.4 P3 安全

- transcoder gRPC **无鉴权**（roadmap 已列 mTLS）——任何内网服务都能发起转码；
- etcd 无认证/TLS；Kafka 无 ACL/TLS；JWT secret 走环境变量可接受但建议 Secret 管理。

### 2.5 P4 一致性架构细节

- **无 Outbox 模式**：`CompleteVideoUpload` 在 DB 事务提交后直接 `SendKafkaMessage`，发送失败靠"标记 failed + 入重试队列"兜底。可用但严格场景应引入 outbox 表保证"DB 变更与消息投递"原子性。
- **无 DLQ**：重试 7 次后静默丢弃（watchdog 里 `cnt > 7 continue`），没有死信 topic、没有告警。
- **消息契约无版本**：Kafka 消息是裸 JSON，字段演进无版本号，跨版本发布期会解析失败。
- **Snowflake node_id 哈希派生有碰撞风险**：`FNV32 % 1024`，实例多时理论上会撞。大规模时应改用 etcd CAS 分配（见 4.5）。

---

## 3. 如何可扩展（架构演进路径）

### 3.1 服务模型：保持"单二进制 + role"，但组件化
当前的 role 模式很好（一个镜像多个角色），继续演进：
- 每个 role 是**组件组合**而非大 if-else：`api = server + minio + redis + kafka(producer)`、`worker = kafka(consumers) + db + transcoder-client + singleton-jobs`；
- 新增角色（如 `scheduler` 定时任务、`gateway`）= 新增 `internal/role/xxx.go` + 启动分发分支，**不破坏既有角色**。

### 3.2 注册中心泛化：从"只有 transcoder 注册"到"全角色注册"
把 `internal/transcoder/registry` 与 `internal/discovery` 上提为通用 `pkg/registry`、`pkg/discovery`：
- 所有角色启动时向 etcd 注册（带 role/version/region/capacity 元数据）；
- API 也注册，供网关/入口服务动态发现 API 实例（跨集群场景），单集群内可继续用 K8s Service DNS；
- key 结构统一为 `/vistack/services/{role}/{instance-id}`，value 为 JSON 元数据（含地址）。

### 3.3 Job 抽象：新增异步任务不再复制 worker 模式
定义统一接口，按 topic 注册 handler：

```go
type JobHandler interface {
    Topic() string
    Handle(ctx context.Context, msg []byte) error
}
```

- 新任务 = 新消息结构 + 注册一个 handler（如 `like-notify`、`transcode-progress`、`hls-generate`）；
- worker 启动时扫描注册表启动所有消费者，**并支持并发消费**：fetch 一批 → 按 key 分区交给有界 worker pool → 全部成功后统一提交 offset（至少一次语义）；配合 DB 状态机 + Redis 租约保持幂等。

### 3.4 网关层：认证/限流/灰度外置
- 前端入口接入 API 网关（Nginx/APISIX/云 LB），做 TLS 终结、限流、IP 白名单、静态资源与 `/api/v1` 分流；
- 网关之后才是多副本 API（K8s Service / etcd 发现），API 本身保持无状态（JWT 无会话，已满足）。

### 3.5 契约版本化
- gRPC：proto 已用 buf 管理，继续按兼容规则演进（只加字段不删改）；
- Kafka JSON 消息增加 `"v": 1` 版本字段 + 兼容解析；量大后可上 Schema Registry；
- 配置结构：`AppConfig` 保持向后兼容，新增字段给默认值。

### 3.6 多区域/多集群（远期）
etcd 做区域元数据注册，网关按 region 路由；Kafka 跨集群镜像（MirrorMaker）；对象存储按区域就近。

---

## 4. 如何使用 etcd（具体到模块）

> 现状：etcd 只做了"transcoder 注册 + worker 发现"两件事。下面按价值排序补齐。

### 4.1 服务注册与发现（已有，泛化）
- 已有 `registry.Registrar`（租约 TTL 10s / 3s 保活）和 `discovery.EtcdBuilder`（gRPC resolver）。
- **扩展点**：注册值从"裸地址字符串"升级为 JSON 元数据 `{"addr","role","version","region","capacity"}`；resolver 解析 value 中的 addr；watch 逻辑不变。
- **注意**：`keepAlive` 目前用 `KeepAliveOnce` + 失败重 Grant，建议换 `KeepAlive` 通道（吞吐更高、更稳），并用 `WithRequireLeader`。

### 4.2 领导选举（P0，立刻要做）
解决 dispatcher/watchdog 单例问题，用 `go.etcd.io/etcd/client/v3/concurrency`：

```go
func RunLeaderElection(ctx context.Context, cli *clientv3.Client, key string, run func(ctx context.Context)) {
    s, _ := concurrency.NewSession(cli, concurrency.WithTTL(10))
    defer s.Close()
    e := concurrency.NewElection(s, key) // key 如 "/vistack/leaders/transcode-singleton"
    for {
        if err := e.Campaign(ctx, instanceID); err != nil { continue }
        go run(ctx)                       // 只有 leader 执行单例循环
        <-e.Done()                        // 失去领导权（租约过期/被抢占）→ 停止并重选
    }
}
```

- 被选中的 leader 运行 retry dispatcher + watchdog；
- leader 崩溃 → 租约过期 → 其他 worker 自动接任，**无主窗口 ≈ TTL**；
- 同样的机制可复用于：migration 单跑、定时任务（如统计聚合、DASH 分段 GC）。

### 4.3 分布式锁（低频全局操作）
`concurrency.NewMutex(sess, "/vistack/locks/{name}")` 用于跨实例互斥：
- 秒传去重的 ref_count 修复、全量缓存重建、配置切换瞬间的一致性操作。

### 4.4 动态配置中心（把配置从"文件 + 重启"升级为"热更新"）
- 非敏感动态配置（限流阈值、重试次数、功能开关、ABR 档位表）写入 `/vistack/config/{env}/{key}`；
- 服务启动时 `Get` 一次 + `Watch` 监听，变更时更新 `global.AppConfig` 并触发对应模块 reload（限流器重建、档位表刷新）；
- 优先级：**文件默认值 < 环境变量 < etcd（运行时最高）**；
- 敏感信息（JWT secret、DB 密码）仍走 Secret/环境变量，**不进 etcd**。

### 4.5 节点 ID 分配（消除 Snowflake 碰撞）
用 etcd 事务（CAS）分配 node_id：

```go
for i := int64(0); i < 1024; i++ {
    key := fmt.Sprintf("/vistack/alloc/node-ids/%d", i)
    if ok := cli.Txn(ctx).If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
        Then(clientv3.OpPut(key, instanceID, clientv3.WithLease(lease.ID))).Commit(); ok.Succeeded {
        return i // 抢占成功，租约释放后自动归还
    }
}
```

### 4.6 特性开关 / 容量元数据
- `/vistack/features/{name}`：全服务 Watch，灰度开关、紧急降级（如临时关闭转码）不用发版；
- transcoder 注册时上报 `capacity`（并发数/是否 GPU），未来 scheduler 可做容量感知派发（当前 Kafka 分发已足够，属增强项）。

### 4.7 etcd 自身高可用
- 生产部署 3 节点（`--initial-cluster` 对等 URL），K8s 可用 Bitnami/etcd-operator Helm chart；
- 开启 TLS（peer + client）与用户名密码认证；
- compose 里目前只有单节点，仅供开发。

---

## 5. 演进路线图（分阶段落地）

| 阶段 | 内容 | 验收标准 |
|------|------|----------|
| **P0-1 正确性** | ① etcd 领导选举接管 dispatcher/watchdog ② migration 拆独立任务 ③ worker 信号优雅退出 + API Shutdown ④ Kafka 并发消费 worker pool | `--scale worker=3` 无重复投递；滚动发布无消息丢失；单 worker 并发转码 > 1 |
| **P0-2 契约与兜底** | Kafka 消息加版本字段；新增 DLQ topic + 失败告警 | 重试超限消息进 DLQ 可重放 |
| **P1-1 可观测性** | Prometheus metrics（HTTP/Kafka lag/转码指标）；OpenTelemetry trace 串联 api→kafka→worker→gRPC；/health 拆 readiness/liveness | 一个上传-转码-播放的完整链路可追踪；K8s 探针生效 |
| **P1-2 高可用** | etcd 3 节点；Kafka 3 broker + 分区副本；PG 主从；Redis Sentinel；分布式 MinIO；k8s 补 Secret/探针/resource/HPA/Ingress | 任意单节点故障不影响服务；HPA 按负载自动扩缩 |
| **P2-1 平台化** | 通用 registry/discovery + 全角色注册；Job handler 抽象；etcd 配置中心；网关层 | 新角色/新任务可插拔式接入 |
| **P2-2 安全与远期** | gRPC mTLS、etcd/Kafka 加密认证、snowflake node_id etcd 分配、多区域 | 内网默认加密；多集群可路由 |

---

## 6. 一句话总结

> 项目已具备分布式**骨架**（三角色 + 消息队列 + 服务发现 + 对象存储），
> 当务之急是补**正确性**：领导选举（dispatcher/watchdog 单例）、迁移单跑、优雅停机、并发消费；
> 然后是**高可用**（中间件去单点）与**可观测性**；
> etcd 的完整价值在于：**服务注册（已用）→ 领导选举（急用）→ 配置中心 + 锁 + ID 分配（规模化后用）**。

---

## 7. 微服务拆分边界

### 7.1 判断框架

- 现状是「模块化单体 + 进程级微服务化」（api/worker/transcoder 三角色 + Kafka 解耦 + gRPC），是微服务最佳起点，不急于拆多 repo。
- 拆分信号只有三个：**独立扩容需求、独立发布频率、安全/故障隔离需求**。
- 按「业务能力边界（bounded context）」拆，不按技术层或一表一服务拆。
- **代码库拆分（拆 repo）是最后一步，不是第一步**：先 monorepo 多二进制 → 部署拆分（独立容器/SLA）→ 独立 repo/go module。

### 7.2 拆分候选（按优先级）

| 梯队 | 服务 | 现状 | 拆出理由 |
|------|------|------|----------|
| ① | 转码执行（transcoder） | 已是独立 gRPC 服务 | 保持；未来按任务类型插件化 |
| ① | 认证与身份（Auth/IAM） | JWT 无状态签发 + RBAC 在 api 内，共享 secret | 安全隔离、密钥轮换、令牌撤销、未来 SSO；拆后升级 RS256 + JWKS |
| ① | 媒体文件/存储（File/Storage） | MinIO 直传 + 预签名 + STS + ref_count + 秒传 + delete worker | 对象生命周期管理边界；delete worker 已是其异步消费者 |
| ② | 视频目录/元数据（Video Catalog） | 视频 CRUD/列表/推荐/manifest/播放签名 | 读多写少独立缓存与扩容；未来搜索/推荐宿主 |
| ② | 社交互动（Social） | 仅 entity 预留（comment/like/favorite/play_log） | **实现时直接独立成服务**（greenfield 成本最低） |
| ② | 调度（Scheduler） | retry dispatcher + watchdog（P0-1 已 etcd 选举包裹） | 让 worker 完全无状态无限扩容；单例逻辑独立 |
| ③ | 直播 SFU（live777） | 仅 README，代码未集成 | 本就是独立 Rust 服务，属待接入项 |
| ③ | 标签/审计 | 仅 entity 预留 | 并入 Video Catalog / 独立审计服务 |

### 7.3 明确不拆

- 不按 entity 一表一服务（comment/like/favorite 各拆一个 = 过度拆分）。
- 数据库暂不拆：先保持共享 PostgreSQL，每服务只读写自己归属的表集合，跨服务用 ID 引用 + 事件/API；到读写瓶颈再按域拆库或 CQRS。

### 7.4 推荐路径

优先拆 **Auth/IAM** 与 **File/Storage**（边界最清晰、安全/隔离收益最直接、现有代码天然支持）。
