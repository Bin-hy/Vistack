# Vistack P0-1 分布式正确性加固 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 新建 | `internal/core/leader/leader.go` | Elector 领导选举封装 |
| 修改 | `internal/config/config.go` | Kafka.Concurrency、Etcd.LeaderTTL 字段 |
| 修改 | `internal/core/kafka.go` | 并发消费 + WaitKafkaConsumers + 退出逻辑修正 |
| 修改 | `internal/role/worker.go` | 信号 ctx + 领导选举接入 + 优雅停机排空 |
| 修改 | `internal/role/api.go` | 移除隐式 AutoMigrate + http.Server Shutdown |
| 新建 | `internal/role/migrate.go` | 独立迁移入口 |
| 修改 | `cmd/vistack/main.go` | role 分发新增 "migrate" |
| 修改 | `conf/app.toml` | kafka.concurrency / etcd.leader_ttl 样例值 |

## T1: 配置字段扩展

**文件：** `internal/config/config.go`
**依赖：** 无
**步骤：**
1. `Kafka` 结构体新增 `Concurrency int \`mapstructure:"concurrency"\``（注释：每个实例并发消费者数，默认 4）
2. `Etcd` 结构体新增 `LeaderTTL int \`mapstructure:"leader_ttl"\``（注释：领导选举租约 TTL 秒，默认 10）

**验证：** `go build ./internal/config/...` 编译通过

## T2: 领导选举模块

**文件：** `internal/core/leader/leader.go`
**依赖：** 无（etcd clientv3 依赖已在 go.mod）
**步骤：**
1. 定义 `Elector` 结构体：`client *clientv3.Client`、`key string`、`id string`、`ttl int`
2. `New(client, key, id, ttl) *Elector` 构造
3. `Run(ctx, onElected func(leadCtx context.Context)) error` 实现竞选循环：
   - 每轮：`concurrency.NewSession(cli, concurrency.WithTTL(ttl))` → `concurrency.NewElection(sess, key)` → `election.Campaign(ctx, id)`
   - 竞选失败（网络错误）：`sess.Close()` + 短退避后重试；外层 ctx 已取消则返回
   - 竞选成功：派生 `leadCtx`（session 丢失或外层 ctx 取消时 leadCtx 取消），调用 `onElected(leadCtx)`，然后 `<-leadCtx.Done()` 阻塞
   - leadCtx 结束后 `sess.Close()`，外层 ctx 未取消则进入下一轮（自动重选）
4. 导出常量 `DefaultLeaderKey = "/vistack/leaders/worker-singleton"`

**验证：** `go build ./internal/core/leader/...` 编译通过

## T3: Kafka 并发消费

**文件：** `internal/core/kafka.go`
**依赖：** T1（读 KafkaConfig.Kafka.Concurrency）
**步骤：**
1. 新增包级 `var consumerWG sync.WaitGroup`
2. `StartKafkaConsumer` 改为：读取 `KafkaConfig.Kafka.Concurrency`（`<=0` 时按 1 处理），启动 N 个 goroutine，每个 goroutine 独立 `kafka.NewReader`（brokers/group/topic/CommitInterval=0 不变），每个 goroutine 内是原消费循环体，`defer consumerWG.Done()`
3. 消费循环退出逻辑修正：`ReadMessage` 返回 error 时**先检查 `ctx.Err()`**，已取消则直接 return（不再 sleep 1s 空转）；仅当 ctx 未取消时才记录日志 + sleep 重试
4. 新增 `WaitKafkaConsumers(timeout time.Duration) bool`：等待 consumerWG 归零；超时返回 false（供 worker 排空判断）

**验证：** `go build ./internal/core/...` 编译通过

## T4: worker 接入（信号 + 选举 + 排空）

**文件：** `internal/role/worker.go`
**依赖：** T2、T3
**步骤：**
1. 引入 `os/signal`、`syscall`、`context`：`ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`，`defer stop()`
2. transcoder client 创建改用 `ctx`（替换 `context.Background()`）
3. `StartTranscodeWorker(ctx)` / `StartVideoDeleteWorker(ctx)` 用该 ctx 启动（并发消费由 T3 保证）
4. 新增 `runSingletonJobs(ctx)`：
   - etcd endpoints 为空 → 日志告警（"etcd not configured, running singleton jobs directly (multi-replica unsafe)"）→ 直接启动 dispatcher + watchdog
   - etcd 连接失败 → 同样告警降级直接启动
   - 成功连接 → `leader.New(cli, DefaultLeaderKey, 实例ID, TTL)`，`elector.Run(ctx, func(leadCtx) { 启动 dispatcher + watchdog })`；TTL `<=0` 用 10
   - 实例 ID：复用 `POD_IP`/hostname（与 snowflake 派生一致的语义）
5. 主流程：`<-ctx.Done()` 后记录 "shutting down worker"，调用 `core.WaitKafkaConsumers(30 * time.Second)`，超时则 `os.Exit(1)`（强杀），否则正常返回退出
6. 保留 `select {}` 语义改为显式等待信号（删除 `select {}` 死循环）

**验证：** `go build ./...` 编译通过；`go vet ./internal/role/...` 无告警

## T5: api 优雅停机 + 移除隐式迁移

**文件：** `internal/role/api.go`
**依赖：** 无
**步骤：**
1. 删除 `migrations.AutoMigrate` 调用与 `migrations` import（保留 `core.InitDB`）
2. 用 `http.Server{Addr: addr, Handler: r}` 替代 `r.Run(addr)`；`go srv.ListenAndServe()` + 错误通道
3. `signal.NotifyContext` 捕获 SIGINT/SIGTERM
4. 收到信号 → `srv.Shutdown(ctx30s)` → 记录 "api exited cleanly"；超时则 `os.Exit(1)`；`ListenAndServe` 返回非 `http.ErrServerClosed` 错误视为启动失败 panic

**验证：** `go build ./...` 编译通过

## T6: 独立迁移角色

**文件：** `internal/role/migrate.go`（新建）、`cmd/vistack/main.go`（修改）
**依赖：** 无
**步骤：**
1. `internal/role/migrate.go`：`RunMigrate(cfg)` — `core.InitDB(cfg)`（DB 为 nil 则 panic "database not initialized"）→ `migrations.AutoMigrate(core.DB)` 失败则 panic → 成功记录 "migration completed"
2. `cmd/vistack/main.go`：role 分发 switch 新增 `case "migrate": role.RunMigrate(&cfg)`

**验证：** `go build ./...` 编译通过

## T7: 配置样例更新

**文件：** `conf/app.toml`（如 `conf/app.docker.toml` / `conf/app.local.toml` 存在同段则一并补默认值）
**依赖：** T1
**步骤：**
1. `[kafka]` 段加 `concurrency = 4`
2. `[etcd]` 段加 `leader_ttl = 10`

**验证：** `go build ./...` 通过后，`go run ./cmd/vistack migrate` 能解析配置（无 DB 环境时至少不因 toml 字段报错；有本地 DB 时可完成迁移）

## 执行顺序

```
T1 → T2 → T3 → T4
  ↘ T5 → T6 → T7
```

T5/T6 与 T2/T3 无依赖，可并行；T4 依赖 T2+T3；最终统一验证 `go build ./... && go vet ./...`
