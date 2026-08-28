# Auth/IAM 服务拆分 Tasks

## 文件清单

| 操作 | 文件 | 职责 |
|------|------|------|
| 改 | `go.mod`/`go.sum` | 升级 golang-jwt/jwt/v5 |
| 改 | `internal/config/config.go` | Auth/AuthService 配置段 |
| 改 | `pkg/auth/token_manager.go` | RS256 签发 + JWKS 生成 |
| 新 | `pkg/auth/verifier.go` | JWKS 验签 + 公钥缓存 |
| 新 | `proto/auth/v1/auth.proto` | UserService 契约 |
| 新 | `internal/auth/pb/...` | protoc 生成的 pb.go |
| 新 | `internal/auth/handler.go` | HTTP：register/login/userinfo/profile/password/jwks |
| 新 | `internal/auth/service.go` | gRPC：GetUserInfos |
| 新 | `internal/role/auth.go` | RunAuth 启动引导 |
| 新 | `internal/authclient/client.go` | api 侧用户查询 gRPC 客户端 |
| 改 | `internal/middlewares/auth.go` | JWKS 验签 |
| 改 | `internal/role/api.go` | 注入 TokenVerifier、移除用户 handler 注册 |
| 改 | `internal/routers/` | 移除 auth/user 路由（迁至 auth） |
| 改 | `internal/api/v1/User.go` | 删除（逻辑迁至 internal/auth） |
| 改 | `internal/api/v1/Video.go` | 作者展示改 gRPC 批量查询 |
| 改 | `internal/global/global.go` | 移除 TokenManager |
| 改 | `cmd/vistack/main.go` | role 分发新增 "auth" |
| 改 | `conf/*.toml`、`compose.yml` | auth 服务配置与容器 |

## T1: 升级 JWT 库

**文件：** `go.mod` / `go.sum` / 所有 import jwt 的文件
**依赖：** 无
**步骤：**
1. `go get github.com/golang-jwt/jwt/v5@latest`
2. 替换 `github.com/dgrijalva/jwt-go` import 为 `github.com/golang-jwt/jwt/v5`（暂在 pkg/auth 内）

**验证：** `go build ./pkg/auth/...` 编译通过；`go.mod` 不再依赖 dgrijalva

## T2: 配置扩展

**文件：** `internal/config/config.go`
**依赖：** 无
**步骤：**
1. `Auth` 段扩展：保留 `jwt_expiration`，新增 `kid`（默认 "vistack-rs256"）、`issuer`（默认 "vistack"）、`jwks_path`（默认 "/.well-known/jwks.json"）；删除 `jwt_secret`（或保留兼容但弃用）
2. 新增 `AuthService` 段：`http_addr`（默认 ":8081"）、`grpc_addr`（默认 ":50052"）

**验证：** `go build ./internal/config/...` 通过

## T3: TokenManager RS256 签发 + JWKS

**文件：** `pkg/auth/token_manager.go`
**依赖：** T1、T2
**步骤：**
1. `TokenManager` 字段改为 `privateKey *rsa.PrivateKey`、`kid`、`issuer`、`expire time.Duration`
2. `NewTokenManager(privateKeyPEM []byte, kid, issuer string, expire time.Duration)`：解析 PEM 私钥（PKCS1/PKCS8 兼容）
3. `GenerateToken(userID)`：`jwt.NewWithClaims(jwt.SigningMethodRS256, claims)`，claims 含 `user_id`/`exp`/`iss`/`kid` header
4. `PublicJWKS()`：从私钥导出公钥，构造 JWKS JSON（kid/kty=RSA/n/e base64url）

**验证：** `go build ./pkg/auth/...` 通过

## T4: TokenVerifier JWKS 验签

**文件：** `pkg/auth/verifier.go`（新）
**依赖：** T1
**步骤：**
1. `TokenVerifier{jwksURL, keyCache map[string]*rsa.PublicKey, mu}`；`NewTokenVerifier(jwksURL)`
2. `refreshKeys(ctx)`：GET jwksURL → 解析 JWKS → 按 kid 缓存公钥
3. `ValidateToken(token)`：解析 header 的 kid → 取公钥 → RS256 验签 + 过期校验；kid 未命中时先 `refreshKeys` 一次再验
4. 后台 goroutine 定期刷新（默认 1h，供密钥轮换）

**验证：** `go build ./pkg/auth/...` 通过

## T5: auth proto 定义与生成

**文件：** `proto/auth/v1/auth.proto`（新）、`internal/auth/pb/auth/v1/*.pb.go`（生成）
**依赖：** 无
**步骤：**
1. 定义 `UserService.GetUserInfos(GetUserInfosRequest{repeated int64 user_ids}) returns (GetUserInfosResponse{repeated UserInfo})`，`UserInfo{id, username, nickname, avatar_url, role}`
2. 用 protoc 生成：`protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative -I proto proto/auth/v1/auth.proto`（输出到 `internal/auth/pb`）

**验证：** 生成文件存在；`go build ./internal/auth/pb/...` 通过

## T6: auth HTTP handler（迁移 UserApi 逻辑）

**文件：** `internal/auth/handler.go`（新）
**依赖：** T2、T3
**步骤：**
1. 从 `internal/api/v1/User.go` 迁移并适配：`Register`、`Login`（改用 RS256 TokenManager）、`GetUserInfo`、`UpdateProfileDirect`、`UpdateUserPassword`
2. 新增 `JWKS` handler：返回 `TokenManager.PublicJWKS()`
3. 路由注册函数：`POST /api/v1/auth/register`、`/api/v1/auth/login`、`GET /api/v1/user/info`、`PUT /api/v1/user/profile`、`PUT /api/v1/user/password`、`GET /.well-known/jwks.json`

**验证：** `go build ./internal/auth/...` 通过

## T7: auth gRPC service

**文件：** `internal/auth/service.go`（新）
**依赖：** T5
**步骤：**
1. 实现 `UserService.GetUserInfos`：按 user_ids 批量查询 users + profiles + avatars，返回公开信息（id/username/nickname/avatar_url/role）

**验证：** `go build ./internal/auth/...` 通过

## T8: auth 角色启动引导

**文件：** `internal/role/auth.go`（新）
**依赖：** T2、T6、T7
**步骤：**
1. `RunAuth(cfg)`：InitDB → 加载私钥（`VISTACK_AUTH_RSA_PRIVATE_KEY` 环境变量 PEM，或 `_FILE` 路径；两者均无则生成 2048-bit 临时密钥并告警）→ InitMinio → 建 TokenManager → 启动 gRPC server（`AuthService.grpc_addr`）+ etcd 注册（复用 registry）→ 启动 HTTP server（`AuthService.http_addr`，含 JWKS/认证路由）→ 信号优雅停机（http.Server Shutdown + gRPC GracefulStop）

**验证：** `go build ./...` 通过

## T9: 用户查询客户端

**文件：** `internal/authclient/client.go`（新）
**依赖：** T5
**步骤：**
1. `UserClient{pb, conn, etcd}`；`NewUserClient(ctx, cfg)`：etcd 发现（复用 discovery.EtcdBuilder + prefix）或静态 `AuthService.grpc_addr`
2. `GetUserInfos(ctx, ids) (map[int64]*authpb.UserInfo, error)`

**验证：** `go build ./internal/authclient/...` 通过

## T10: api 中间件改 JWKS 验签

**文件：** `internal/middlewares/auth.go`、`internal/role/api.go`、`internal/global/global.go`
**依赖：** T4
**步骤：**
1. `AuthMiddleware(v *auth.TokenVerifier)`：改注入 TokenVerifier，`ValidateToken` 本地验签
2. `role/api.go`：移除 `global.TokenManager = auth.NewTokenManager(...)`，改为 `verifier := auth.NewTokenVerifier(jwksURL)`（jwksURL 由 `AuthService` 地址拼 `/...` 或独立配置）
3. `global.go`：删除 `TokenManager` 字段

**验证：** `go build ./...` 通过

## T11: api 移除 auth/user 路由

**文件：** `internal/routers/router.go`、`internal/routers/api/v1/auth.go`、`user.go`、`enter.go`
**依赖：** T10
**步骤：**
1. 移除 `v1.RouterGroupApp.InitAuthRouter` 与 `InitUserRouter` 的注册调用
2. 删除 `routers/api/v1/auth.go` 与 `user.go`（路由定义迁至 auth 服务）

**验证：** `go build ./...` 通过；`internal/api/v1/User.go` 已无引用后可删

## T12: Video.go 作者展示改 gRPC 查询

**文件：** `internal/api/v1/Video.go`
**依赖：** T9
**步骤：**
1. 移除 `Preload("User.Profile.Avatar")`、`Preload("User")` 等 join
2. 新增 `api` 包级 `authClient`（在 role/api.go 注入）；查询视频列表后收集 userIDs，`GetUserInfos` 批量查询，内存映射填充 `VideoAuthorResponse`
3. `GetVideoInfo`/`GetSelfVideoPage`/`GetVideoRecommend` 三处同步改造

**验证：** `go build ./...` 通过

## T13: main.go 分发 auth

**文件：** `cmd/vistack/main.go`
**依赖：** T8
**步骤：**
1. switch 新增 `case "auth": role.RunAuth(&cfg)`

**验证：** `go build ./...` 通过

## T14: 配置与部署

**文件：** `conf/app.toml`、`conf/app.docker.toml`、`conf/app.local.toml`、`compose.yml`
**依赖：** T2
**步骤：**
1. 配置文件补 `[auth]`（kid/issuer/jwt_expiration/jwks_path）与 `[auth_service]`（http_addr/grpc_addr）
2. compose.yml 新增 `auth` 服务（镜像复用 vistack，`command: ["auth"]`，端口 8081/50052，depends_on postgres/etcd/minio，私钥经环境注入）

**验证：** `go build ./...` 通过；配置可被 Viper 解析

## T15: 端到端验收

**依赖：** T1~T14
**步骤：**
1. `go build ./... && go vet ./...`
2. 起 postgres/redis/etcd/minio + auth 服务，curl 注册/登录拿 RS256 token，curl JWKS 端点验证公钥
3. 起 api 服务，携带 token 访问受保护接口验证本地验签；访问视频列表验证作者信息经 auth 填充

**验证：** 见 checklist.md 端到端场景

## 执行顺序

```
T1 → T2 → T3 → T4
            ↘
T5 → T7 → T8 → T13 → T14
      ↘
T6 → T8
T5 → T9 → T12
T4 → T10 → T11 → T12
                 ↘ T15（全部依赖）
```

关键路径：`T1→T2→T3→T6→T8→T13` 与 `T5→T7→T8` 汇合；api 侧 `T4→T10→T11` 与 `T5→T9→T12` 独立可并行。
