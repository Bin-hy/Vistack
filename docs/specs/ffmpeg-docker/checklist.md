# FFmpeg 容器化与三角色拆分 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性
- [ ] 三角色可独立启动（验证：`VISTACK_ROLE=api|worker|transcoder go run .`，观察各角色启动的依赖是否互斥）
- [ ] transcoder 不连 DB、不起 HTTP；api 不起 Kafka 消费者与 gRPC（验证：启动日志中无对应初始化/监听输出，对应 AC1）
- [ ] `ProcessVideo` 已实现且可被调用（验证：`go build ./...` 通过，对应 AC2）

## 集成
- [ ] worker 通过 etcd 发现的 transcoder 地址调用 gRPC（验证：etcd 中注册 2 个实例，关闭 1 个后 worker 后续只调存活实例，对应 AC3）
- [ ] 转码产物与 DB 写入等价（验证：同一原视频新旧实现产出档位/分片命名/封面路径一致，DB 最终状态等价，对应 AC4）
- [ ] 失败重试与超时兜底仍生效（验证：人为让 transcoder 报错/超时，worker 指数退避重试、超上限丢弃，对应 AC5）
- [ ] 删除链路仍可清理 MinIO 与 DB（验证：删除已发布视频后 `dash/{video_id}` 分片与相关 DB 记录被清除）

## 编译与测试
- [ ] `go build ./...` 无错误
- [ ] `go vet ./...` 无错误
- [ ] `buf lint` 无错误（proto 校验）

## 部署
- [ ] `docker compose config` 校验通过（对应 AC6）
- [ ] `docker compose up` 后 api/worker/transcoder/etcd 及基础设施全部健康（对应 AC6）
- [ ] k8s 清单 `kubectl apply --dry-run=client` 通过，三应用 Deployment 就绪（对应 AC6）
- [ ] 通过环境变量改 transcoder 地址/etcd 端点后 worker 无需重编译即可连到新实例（对应 AC7）

## 端到端场景
- [ ] 场景 1：`docker compose up` → 登录 → 上传一个视频 → 等待转码完成 → 首页/个人中心可见已发布视频 → 播放页可正常播放 DASH（可切清晰度、封面正确）
- [ ] 场景 2（扩缩容）：`docker compose scale transcoder=3` 后上传多个视频，转码被分发到多个 transcoder 实例；缩到 1 个后任务仍能完成
- [ ] 场景 3（故障恢复）：转码过程中手动 `docker stop` 一个 transcoder 容器，watchdog/重试在超时后重新调度，最终视频仍转为 published
