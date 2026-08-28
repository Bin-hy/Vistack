# Vistack P0-1 分布式正确性加固 Plan

## 架构概览

四个改造点对应四个模块，全部收敛在现有 `internal/` 分层内，不改三角色边界：

1. **`internal/core/leader`（新）** — 基于 etcd `concurrency.Election` 的领导选举封装，解决 F1 单例调度。
2. **`internal/core/kafka.go`（改）** — 并发消费：`StartKafkaConsumer` 由"单 reader 顺序循环"升级为"N 个同 group reader"，解决 F5 并发=1。
3. **`internal/role/worker.go` / `internal/role/api.go`（改）** — 优雅停机：信号感知 + 排空在途 + 超时强杀，解决 F2/F3。
4. **`internal/role/migrate.go`（新）** — 独立迁移入口，`api` 启动不再隐式 AutoMigrate，解决 F4。

## 核心数据结构

### `leader.Elector`（internal/core/leader/leader.go）

```go
type Elector struct {
    client *clientv3.Client
    key    string   // 选举 key，如 /vistack/leaders/worker-singleton
    id     string   // 实例标识（hostname/instance-id），用于区分 leader
    ttl    int      // 租约 TTL 秒数
}

// New 创建选举器
func New(client *clientv3.Client, key, id string, ttl int) *Elector

// Run 阻塞直到 ctx 结束：
//   1) 循环 campaign，成为 leader 后以 leadCtx 调用 onElected
//   2) 失去领导权（租约过期/被抢占）时 leadCtx 被取消，onElected 返回后自动重新竞选
//   3) 外层 ctx 取消时退出循环返回
func (e *Elector) Run(ctx context.Context, onElected func(leadCtx context.Context)) error
```

### 配置扩展（internal/config/config.go）

```go
Kafka struct {
    Brokers     []string `mapstructure:"brokers"`
    GroupID     string   `mapstructure:"group_id"`
    Concurrency int      `mapstructure:"concurrency"` // 每个实例并发消费者数，默认 4
}
Etcd struct {
    Endpoints []string `mapstructure:"endpoints"`
    Prefix    string   `mapstructure:"prefix"`
    LeaderTTL int      `mapstructure:"leader_ttl"` // 领导选举租约 TTL 秒，默认 10
}
```

## 模块设计

### 模块 A：领导选举（F1）

**职责：** 让 retry dispatcher 与 watchdog 在任何时刻只在一个 worker 实例上运行。

**对外接口：**
- `leader.New(client, key, id, ttl) *Elector`
- `(*Elector).Run(ctx, onElected)`

**内部实现：**
- 每个竞选轮：`concurrency.NewSession(cli, WithTTL(ttl))` → `concurrency.NewElection(sess, key)` → `election.Campaign(ctx, id)`；
- 竞选成功后创建 `leadCtx`（取消链：session 丢失 或 外层 ctx），调用 `onElected(leadCtx)`；
- `onElected` 返回（即 leadCtx 被取消）后 `sess.Close()`，检查外层 ctx，未结束时进入下一轮竞选（自动重连）；
- 竞选失败（网络错误）→ 退避重试，不退出。

**依赖：** etcd client v3、现有 `go.etcd.io/etcd/client/v3`（项目已有）。

### 模块 B：并发消费（F5）

**职责：** 单个 worker 实例内多个 Kafka 消息可并行处理。

**对外接口（签名不变，内部并发）：**
- `core.StartKafkaConsumer(ctx, topic, handler)` — 内部读取 `KafkaConfig.Kafka.Concurrency`，启动 N 个同 group reader goroutine；
- `core.WaitKafkaConsumers(ctx)` — 阻塞直到全部消费者 goroutine 退出（用于优雅停机排空）。

**内部实现：**
- 每个 goroutine 创建独立 `kafka.NewReader`（同 brokers/group/topic，`CommitInterval: 0`）；
- 同一 partition 只会被组内一个 reader 持有 → **同 partition 消息仍顺序处理、顺序提交**，at-least-once 语义不破坏；
- 并发上限 = min(Concurrency, topic 分区数)；转码 topic 建议生产配置 8~16 分区；
- 现有幂等（Redis SetNX 租约 + DB 状态前置校验）在并发下继续生效，重复投递被去重；
- 修正现有退出逻辑：`ReadMessage` 返回 error 时先检查 `ctx.Err()`，已取消则直接 return，避免 select-default 随机分支导致的 1s 延迟退出。

### 模块 C：优雅停机（F2/F3）

**worker（role/worker.go）：**
1. `ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`；
2. 常规消费者（transcode/delete）以 `ctx` 启动，内部 goroutine 各自 `defer wg.Done()`；
3. 主 goroutine 运行领导选举（阻塞），`ctx.Done()` 后退出；
4. 退出序列：`stop()` → 取消 ctx（消费者停止取新消息、选举停止）→ `core.WaitKafkaConsumers()`（等在途 handler 完成）→ 超时上限（30s）后强制 `os.Exit(1)`；
5. 用 `time.AfterFunc` + `select` 实现"等排空或超时二选一"。

**api（role/api.go）：**
1. 用 `http.Server{Addr, Handler: r}` 替代 `r.Run(addr)`；
2. `go srv.ListenAndServe()` + 错误通道；
3. 信号到达 → `srv.Shutdown(ctx30s)` → 在途请求排空 → 退出。

### 模块 D：独立迁移（F4）

**role/migrate.go（新）：**
```go
// RunMigrate 执行数据库迁移后退出（幂等，可重复执行）
func RunMigrate(cfg *config.AppConfig) {
    core.InitDB(cfg)
    if err := migrations.AutoMigrate(core.DB); err != nil { ... panic }
    core.Logger.Info("migration completed")
}
```
- `cmd/vistack/main.go` 的 role 分发新增 `"migrate"` 分支；
- `role/api.go` 移除 `migrations.AutoMigrate` 调用（保留 InitDB）；
- compose/k8s 后续用一次性任务（`docker compose run api migrate` / k8s Job）执行迁移。

## 模块交互（worker 启动时序）

```
RunWorker(cfg)
 ├─ InitDB / InitMinio / InitRedis / InitSnowflake / InitKafka
 ├─ ctx = signal.NotifyContext(SIGINT, SIGTERM)
 ├─ 创建 transcoder gRPC client（etcd 发现/静态）
 ├─ go StartTranscodeWorker(ctx)   ── 内部 N 个 reader 并发消费
 ├─ go StartVideoDeleteWorker(ctx) ── 内部 N 个 reader 并发消费
 ├─ 领导选举（etcd 可用时）：
 │    Run(ctx, func(leadCtx) {
 │        StartTranscodeRetryDispatcher(leadCtx)  // 仅 leader 运行
 │        StartTranscodeWatchdog(leadCtx)          // 仅 leader 运行
 │    })
 │    └─ etcd 未配置/连不上：日志告警 + 降级直接运行单例任务（单实例兼容）
 ├─ <-ctx.Done()（收到信号）
 └─ 排空：WaitKafkaConsumers() 或 30s 超时 → 退出
```

## 文件组织

```
internal/
├── core/
│   ├── leader/leader.go        — 新建：Elector 领导选举
│   ├── kafka.go                — 修改：并发消费 + WaitKafkaConsumers + 退出逻辑修正
│   └── (etcd client 初始化复用 transcoder 的既有模式，不新增全局)
├── role/
│   ├── worker.go               — 修改：信号 ctx + 领导选举接入 + 优雅停机
│   ├── api.go                  — 修改：移除 AutoMigrate + http.Server Shutdown
│   ├── migrate.go              — 新建：独立迁移入口
│   └── transcoder.go           — 不变
├── config/config.go            — 修改：Kafka.Concurrency、Etcd.LeaderTTL
├── core/message_queue/transcode/
│   ├── retry.go / watchdog.go  — 不变（已接收 ctx，由 leader 以 leadCtx 调用）
│   └── worker.go               — 不变
└── core/message_queue/video/   — 不变
cmd/vistack/main.go             — 修改：role 分发新增 "migrate"
conf/app.toml                   — 修改：kafka.concurrency、etcd.leader_ttl 默认值
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 单例保证机制 | etcd `concurrency.Election`（租约 + 自动重选） | 项目已有 etcd 依赖；lease 即 liveness，leader 崩溃后无主窗口 ≤ TTL，且有 fencing 语义；优于 Redis 锁（无租约续期、无自动故障转移） |
| 并发消费模型 | N 个同 group reader | 代码改动最小（复用既有消费循环）；同 partition 由单 reader 持有，天然保序 + 顺序提交，不引入乱序提交跳过 offset 的风险 |
| 并发度上限 | min(N, 分区数) | Kafka 并行单位就是 partition；更高并发靠加大 topic 分区数（运维项，本次不改 topic） |
| 优雅停机 | signal.NotifyContext + reader 退出 + wg 排空 + 30s 超时强杀 | 顺序确定（停收新 → 排空在途 → 退出），超时兜底避免滚动发布卡死 |
| api 停机 | http.Server.Shutdown(ctx) | Gin 官方推荐，等待在途请求完成 |
| 迁移入口 | 复用单二进制新增 `migrate` role | 零部署成本（compose run / k8s Job 都可直接复用镜像），不动构建体系 |
| etcd 降级 | 未配置/连接失败 → 告警 + 降级直接运行单例 | 兼容本地单实例开发；多实例无 etcd 时通过日志显式暴露风险（属 N2 要求） |

## spec 覆盖检查

| spec 需求 | 落点 |
|-----------|------|
| F1 单例调度 | 模块 A：leader.Elector 包裹 dispatcher/watchdog |
| F2 worker 优雅停机 | 模块 C：worker 信号 ctx + 排空 |
| F3 api 优雅停机 | 模块 C：http.Server.Shutdown |
| F4 迁移独立执行 | 模块 D：migrate role + api 移除隐式迁移 |
| F5 并发消费 | 模块 B：N reader 并发 + 幂等保持 |
| N1 配置兼容 | 新增配置项均有默认值（Concurrency=4、LeaderTTL=10） |
| N2 etcd 降级 | 模块 A 的降级分支 |
| N3 幂等保持 | 模块 B 同 partition 顺序提交 + 既有 Redis 租约 |
| N4 代码组织 | 全部落在 internal/{core,role,config}，符合现有分层 |
