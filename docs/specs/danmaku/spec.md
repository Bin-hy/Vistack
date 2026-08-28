# 点播弹幕系统 Spec

## 背景

- 项目已有点播（DASH 播放）与直播（live777 SFU），但无弹幕能力。
- 已有可复用基础：限流中间件（按用户）、`core.Redis`、`core.SendKafkaMessage` / `StartKafkaConsumer`（Kafka 生产者/消费者）、worker 角色、snowflake、AC 自动机可自实现。
- 对标 B 站点播弹幕：弹幕绑定视频时间轴，按 `video_id + time_range` 分段拉取，Redis ZSet 按时间排序缓存，Kafka 削峰异步落库，热门视频走多级缓存 + CDN。

## 目标

- 用户可对视频发送弹幕（绑定视频时间轴），其他观众按播放进度分段拉取。
- 高并发写走 Redis（实时可见）+ Kafka（削峰异步落库 PostgreSQL）。
- 弹幕按时间排序缓存于 Redis ZSet，按时间范围高效查询。
- 热门视频的弹幕分段走多级缓存（本地 LRU + Redis + CDN）。
- 敏感词过滤（AC 自动机）。

## 功能需求

- **F1（发送弹幕）**：认证用户对视频发送弹幕（`POST /videos/:id/danmaku`，受限流保护）。弹幕含内容与时间轴位置（`time_offset` 秒）。发送成功即写入 Redis ZSet（实时可见），并投递 Kafka（异步持久化）。

- **F2（敏感词过滤）**：发送时用 AC 自动机检测敏感词，命中则拒绝（不落库、不入 Redis），未命中才放行。关键词从数据库 `sensitive_words` 表加载，增删后**实时重建自动机**。

- **F3（分段拉取）**：按 `video_id + time_range` 拉取弹幕（`GET /videos/:id/danmaku?start=&end=`），返回该时间范围内、按 `time_offset` 升序的弹幕。

- **F4（Redis ZSet 缓存）**：弹幕以 `time_offset` 为 score 存于 `vistack:danmaku:<video_id>`，查询用 `ZRANGEBYSCORE`；多级缓存未命中时回源 DB 并回填。

- **F5（热门视频多级缓存）**：弹幕分段读取先查本地进程内 LRU 缓存，再查 Redis，最后 DB；响应带 `Cache-Control` 头，供 CDN 缓存热门视频的分段结果。

- **F6（Kafka 异步落库）**：弹幕消息投递到 `danmaku` topic，worker 消费并写入 PostgreSQL `danmakus` 表；消费幂等（以弹幕 ID 去重）。

- **F7（降级）**：Redis 不可用时，分段拉取直接回源 DB（不 5xx）；Kafka 不可用时发送仍返回成功（Redis 已实时写入，落库重试兜底）。

- **F8（敏感词管理）**：敏感词存 `sensitive_words` 表，提供增/删/查接口（登录用户）；增删后**实时重建 AC 自动机**，立即生效。

## 非功能需求

- **N1**：高并发写走 Redis + Kafka 削峰，DB 仅承受异步批量写入压力。
- **N2**：弹幕按时间轴分段，避免全量拉取（热门视频弹幕量可能极大）。
- **N3**：发送与消费均幂等（同一弹幕不重复落库）。
- **N4**：不破坏既有 API 契约，新增接口向后兼容。
- **N5**：结构化日志（发送、敏感词命中、落库）。

## 不做的事

- 不做 WebSocket 实时增量推送（第二期）。
- 不做直播弹幕（第二期）。
- 不做弹幕采样（超多弹幕抽样展示，后续）。
- 不做弹幕点赞/举报/举报审核（后续）。
- 不做敏感词异步审核（仅同步 AC 过滤）。
- 不做 web-admin 敏感词管理页面（后续）。

## 验收标准

- **AC1（F1）**：发送弹幕后，立即能通过分段拉取查到该弹幕（Redis 实时写入）。
- **AC2（F2）**：含敏感词的弹幕被拒绝，不进入 Redis/DB。
- **AC3（F3/F4）**：分段拉取只返回指定时间范围内的弹幕，且按 `time_offset` 升序。
- **AC4（F6）**：发送后等待落库，`danmakus` 表出现对应记录（幂等，重复消费不重复）。
- **AC5（F5）**：分段响应带 `Cache-Control`；本地缓存命中时不重复查 Redis（日志可观测）。
- **AC6（F7）**：Redis 关闭时分段拉取回退 DB（不 5xx）。
- **AC7（F8）**：新增敏感词后立即生效（后续含该词的弹幕被拒）；删除后该词不再拦截。
