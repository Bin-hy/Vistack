# 点赞/收藏/播放量计数 + 榜单 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] `interaction.Service` 已实现 toggle/play 计数/状态/榜单/落库（验证：`go build ./internal/interaction/...`）
- [ ] `videos` 表含 `like_count`/`favorite_count`/`play_count` 列（验证：编译 + 迁移后查表结构）
- [ ] 6 个 API handler 已实现（验证：编译 + grep 路由）

## 集成

- [ ] `role/api.go` 构造 `interaction.Service` 并 `SetInteractionService` + `StartFlusher`（验证：grep 调用点）
- [ ] 路由公开组（play/stats/hot）与受保护组（like/favorite/interaction）正确挂载（验证：grep 路由 + 编译）
- [ ] `[social]` 配置段存在（验证：grep 两处 toml）

## 编译与测试

- [ ] `go build ./...` 全量编译通过
- [ ] `go test ./internal/interaction/...` 全部通过
- [ ] `go vet ./internal/interaction/...` 无告警

## 端到端场景

- [ ] **场景 1（点赞 toggle）**：同一用户点赞 2 次，Redis `SCARD` 最终为 0（赞→取消），返回状态正确（验证：HTTP 调用 + `redis-cli SCARD`）
- [ ] **场景 2（播放上报）**：调用 `POST /videos/:id/play` N 次，`GET play` 计数 +N（验证：HTTP + `redis-cli GET`）
- [ ] **场景 3（计数/状态查询）**：`GET /videos/:id/stats` 返回三计数；`GET /videos/:id/interaction` 返回当前用户 `liked/favorited`（验证：HTTP 响应）
- [ ] **场景 4（异步落库一致）**：等待 flush 后，`videos.like_count/play_count` 与 Redis 一致，`video_likes` 有明细行（验证：psql 查询 + redis-cli 对比）
- [ ] **场景 5（榜单）**：多个视频不同播放量，`GET /videos/hot?sort=play` 按播放量降序（验证：HTTP 响应顺序）
- [ ] **场景 6（降级）**：Redis 关闭时 `GET /videos/:id/stats` 回退 DB 冗余列，不 5xx（验证：停 Redis 后请求仍 200）
