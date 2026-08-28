# Vistack P0-1 分布式正确性加固 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] T1 配置字段已加（验证：`go build ./internal/config/...` 通过；`conf/app.toml` 含 `concurrency`、`leader_ttl`）
- [ ] T2 领导选举模块已实现（验证：`go build ./internal/core/leader/...` 通过；Elector 支持竞选 → 执行 → 失去领导权重选）
- [ ] T3 并发消费已实现（验证：`go build ./internal/core/...` 通过；`StartKafkaConsumer` 内部并发数读取配置；`WaitKafkaConsumers` 存在且可调用）
- [ ] T4 worker 已接入信号与选举（验证：`go build ./...` 通过；worker 启动含 `signal.NotifyContext` 与 `runSingletonJobs`）
- [ ] T5 api 已移除隐式迁移并优雅停机（验证：`go build ./...` 通过；api 启动日志不再出现 AutoMigrate；`http.Server.Shutdown` 生效）
- [ ] T6 migrate 角色可用（验证：`go run ./cmd/vistack migrate` 可执行——有 DB 环境时迁移成功退出码 0）
- [ ] T7 配置样例已更新（验证：`conf/app.toml` 的 `[kafka]` 含 `concurrency`、`[etcd]` 含 `leader_ttl`）

## 集成

- [ ] dispatcher/watchdog 只被 leader 调用（验证：`runSingletonJobs` 中二者仅出现在 etcd 选举的 `onElected` 回调或降级分支；grep 无其他调用点）
- [ ] 并发消费不破坏幂等（验证：transcode handler 中 Redis SetNX 租约与 DB 状态前置校验保留未删）
- [ ] worker 排空路径真实等待在途任务（验证：`WaitKafkaConsumers` 在 `<-ctx.Done()` 之后被调用，且超时有 `os.Exit(1)` 兜底）
- [ ] api 迁移职责移交（验证：grep `AutoMigrate` 只在 `role/migrate.go` 与 `migrations/` 中出现）

## 编译与测试

- [ ] 项目编译无错误（验证：`go build ./...` 退出码 0）
- [ ] 静态检查通过（验证：`go vet ./...` 无告警）
- [ ] 配置解析向后兼容（验证：不带新字段的旧配置 `go run ./cmd/vistack api` 仍能启动——无完整环境时至少 `Viper()` 解析不 panic，`go run ./cmd/vistack migrate` 能走到 DB 初始化步骤）

## 端到端场景

- [ ] 场景 1（单例调度）：启动 3 个 worker 副本（etcd 可用），观察日志/etcd key——任意时刻只有 1 个实例输出 dispatcher/watchdog 执行日志；kill leader 实例后，约一个 TTL 内另一实例接任并恢复执行
- [ ] 场景 2（并发消费）：单个 worker 配置 `concurrency = 4`，同时投递多条转码消息，观察转码处理日志存在并发交错（不同 video_id 同时处于 processing），且完成后状态全部正确
- [ ] 场景 3（worker 优雅停机）：向 worker 发送 SIGTERM，观察日志顺序——停止接收 → 排空在途 → "exited"；期间 Kafka 消费者组 rebalance 后其他 worker 继续消费，无重复执行（幂等去重）
- [ ] 场景 4（api 优雅停机）：向 api 发送 SIGTERM 且存在慢请求时，在途请求正常返回，进程在 30s 内退出
- [ ] 场景 5（迁移独立）：`go run ./cmd/vistack migrate` 连续执行两次均成功（幂等）；api 启动日志无迁移输出
