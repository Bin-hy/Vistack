# Auth/IAM 服务拆分 Checklist

> 每一项通过运行代码或观察行为来验证，聚焦系统行为。

## 实现完整性

- [ ] JWT 库已升级 golang-jwt/v5（验证：`grep dgrijalva go.mod` 无结果；`grep golang-jwt/jwt/v5 go.mod` 有结果）
- [ ] TokenManager 支持 RS256 签发与 JWKS 生成（验证：`go build ./pkg/auth/...` 通过；`PublicJWKS()` 可被调用）
- [ ] TokenVerifier 支持 JWKS 验签与公钥缓存（验证：`go build ./pkg/auth/...` 通过）
- [ ] auth proto 已定义并生成 pb.go（验证：`internal/auth/pb/auth/v1/auth.pb.go` 存在；`go build ./internal/auth/pb/...` 通过）
- [ ] auth HTTP handler 已实现（register/login/userinfo/profile/password/jwks）（验证：`go build ./internal/auth/...` 通过）
- [ ] auth gRPC service 已实现 GetUserInfos（验证：`go build ./internal/auth/...` 通过）
- [ ] auth role 启动引导已实现（验证：`go run ./cmd/vistack auth` 可启动——有 DB/etcd 环境时监听 8081/50052）
- [ ] 用户查询客户端已实现（验证：`go build ./internal/authclient/...` 通过）
- [ ] api 中间件已改 JWKS 验签（验证：`AuthMiddleware` 接受 `*auth.TokenVerifier`；`global.TokenManager` 已移除）
- [ ] api 已移除 auth/user 路由（验证：`internal/routers/api/v1/auth.go`、`user.go` 已删除或无注册）
- [ ] Video.go 作者展示已改 gRPC 查询（验证：grep `Preload("User` 在 `internal/api/v1/Video.go` 无结果）
- [ ] main.go 已分发 auth role（验证：`go run ./cmd/vistack` 提示的 role 列表含 auth）

## 集成

- [ ] api 不持有私钥（验证：grep `RSA_PRIVATE_KEY` 或 `rsa.PrivateKey` 仅在 `pkg/auth` 与 `internal/auth`/`internal/role/auth.go` 出现，`internal/api/` 与 `internal/middlewares/` 不出现）
- [ ] api 不回调 auth 即可验签（验证：`TokenVerifier.ValidateToken` 只依赖本地公钥缓存 + JWKS 拉取，无 gRPC/HTTP 调用 auth 的验签路径）
- [ ] 用户数据读写归属 auth（验证：`internal/api/` 下不再有对 `user.User`/`user.UserProfile` 的 DB 写操作；读操作仅经 authclient 的 gRPC）
- [ ] 注册/登录行为逻辑与迁移前一致（验证：register/login handler 的唯一性校验、默认角色、bcrypt、返回结构不变）

## 编译与测试

- [ ] 全项目编译无错误（验证：`go build ./...` 退出码 0）
- [ ] 静态检查通过（验证：`go vet ./...` 无告警）
- [ ] 配置向后兼容（验证：不带新 `[auth_service]` 段的旧配置 `go run ./cmd/vistack api` 仍能启动）

## 端到端场景

- [ ] 场景 1（注册登录）：启动 auth 服务 → `POST /api/v1/auth/register` 注册新用户 → `POST /api/v1/auth/login` 登录，返回 token
- [ ] 场景 2（RS256 + JWKS）：解析登录返回的 token 头，确认 `alg=RS256`；`GET /.well-known/jwks.json` 返回含 kid 的 RSA 公钥
- [ ] 场景 3（本地验签）：启动 api 服务（不配置任何私钥）→ 携带场景 1 的 token 访问受保护接口（如 `GET /api/v1/user/info` 迁移后应走 auth，或任一受保护业务接口）成功；伪造/过期 token 返回 401
- [ ] 场景 4（作者展示）：视频列表接口返回的视频作者昵称/头像来自 auth 批量查询，无 DB join 也能正确填充
- [ ] 场景 5（路径兼容）：`/api/v1/auth/login` 与 `/api/v1/auth/register` 的请求/响应结构与迁移前完全一致（前端零改动）
