# 后端高危问题修复（H1–H5）Checklist

> 每一项通过运行或观察行为验证，聚焦系统行为。

## 实现完整性
- [ ] `ValidateConfig` 已实现并在 `main.go` 调用（验证：`go build ./...` 通过，对应 AC1）
- [ ] Snowflake 派生逻辑已实现，配置文件 `node_id=0`（验证：`go build ./...`，对应 AC3）
- [ ] Kafka 生产者非 `Async`（验证：grep `kafka.go` 无 `Async: true`，对应 AC5）
- [ ] watchdog 含 `pending` 扫描逻辑（验证：grep 有 `TranscodeStatusPending`，对应 AC5）

## 集成与一致性
- [ ] 无泄露/调试输出（验证：grep `minioConfig`/`avatarURL`/`fmt.Printf("Internal")` 无命中，对应 AC2）
- [ ] `DeleteVideo` 无 `ref_count` 扣减（验证：grep 该函数内无 `ref_count`，对应 AC4）
- [ ] `CompleteVideoUpload`/`InitVideoUpload`/`DeleteVideo` 处理 `SendKafkaMessage` 错误（验证：grep 无 `_ = core.SendKafkaMessage`，对应 AC5）

## 编译与测试
- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无错误

## 端到端场景
- [ ] 场景 1（AC1）：`VISTACK_AUTH_JWT_SECRET=`（弱值）且 `server.mode=release` 启动 → 进程拒绝启动并报错；`debug` 模式 → 仅告警正常启动。
- [ ] 场景 2（AC3）：两个不同 hostname 的实例日志中 `snowflake initialized` 的 `node_id` 不同。
- [ ] 场景 3（AC5）：人为使 Kafka 不可用后上传视频 → `CompleteVideoUpload` 返回 500 且转码任务进入重试；Kafka 恢复后任务最终转码完成。
