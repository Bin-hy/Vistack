# 点播弹幕系统 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] 弹幕实体 + 敏感词实体已实现（验证：`go build ./internal/model/...`）
- [ ] AC 自动机已实现（含 `atomic.Pointer` 动态重建）（验证：编译 + 单测）
- [ ] 本地 LRU 缓存已实现（验证：编译 + 单测）
- [ ] `danmaku.Service` 已实现 Send/Fetch/敏感词 CRUD（验证：编译 + 单测）
- [ ] Kafka 消费者已实现（验证：编译）

## 集成

- [ ] `role/api.go` 构造 service + `LoadSensitiveWords`（验证：grep 调用点）
- [ ] `role/worker.go` 启动 danmaku worker（验证：grep 调用点）
- [ ] 路由公开组（GET danmaku）+ 受保护组（POST danmaku / admin 敏感词）挂载（验证：grep 路由 + 编译）
- [ ] 迁移含 `danmakus`、`sensitive_words` 表（验证：迁移后查表）
- [ ] `[danmaku]` 配置段存在（验证：grep 两处 toml）

## 编译与测试

- [ ] `go build ./...` 全量编译通过
- [ ] `go test ./internal/danmaku/...` 全部通过
- [ ] `go vet ./internal/danmaku/...` 无告警

## 端到端场景

- [ ] **场景 1（发送即见）**：`POST /videos/:id/danmaku` 后立即 `GET /videos/:id/danmaku` 能查到该弹幕（验证：HTTP + `redis-cli ZRANGE`）
- [ ] **场景 2（敏感词拦截）**：发送含敏感词的弹幕返回 400，Redis 无该弹幕（验证：HTTP + `redis-cli ZRANGEBYSCORE`）
- [ ] **场景 3（分段拉取）**：写入不同 `time_offset` 的弹幕，按 `start/end` 查询只返回范围内且升序（验证：HTTP 响应顺序）
- [ ] **场景 4（异步落库 + 幂等）**：等待 worker 后 `danmakus` 表有记录，且不重复（验证：psql 查询）
- [ ] **场景 5（敏感词动态生效）**：`POST /admin/sensitive-words` 新增词后，含该词的弹幕立即被拒；删除后恢复（验证：HTTP 先后对比）
- [ ] **场景 6（Cache-Control）**：`GET /videos/:id/danmaku` 响应带 `Cache-Control: public`（验证：响应头）
