# Auth/IAM 服务拆分 Plan

## 架构概览

新增 `auth` 角色（第四个进程），与 `api`/`worker`/`transcoder` 并列，复用单二进制多 role 模式：

1. **auth 服务（新）**：双协议 —— HTTP 对外（登录/注册/资料/改密/JWKS）+ gRPC 对内（用户批量查询）；持有 RSA 私钥，签发 RS256 JWT；向 etcd 注册（复用 transcoder 的 registry/discovery 模式）。
2. **JWT 基础设施（改 pkg/auth）**：升级 `dgrijalva/jwt-go` → `golang-jwt/jwt/v5`，实现 RS256 签发 + JWKS 生成 + JWKS 验签。
3. **api 服务（改）**：`AuthMiddleware` 改为 JWKS 本地验签；移除认证与用户资料 handler（迁到 auth）；`Video.go` 的作者展示改为调用 auth gRPC 批量查询。
4. **密钥管理（新）**：私钥经环境/Secret 注入 PEM；开发环境支持自动生成。

## 核心数据结构

### `pkg/auth/token_manager.go`（签发侧，auth 服务使用）

```go
type TokenManager struct {
    privateKey *rsa.PrivateKey
    kid        string
    issuer     string
    expire     time.Duration
}

func NewTokenManager(privateKeyPEM []byte, kid, issuer string, expire time.Duration) (*TokenManager, error)
func (tm *TokenManager) GenerateToken(userID int64) (string, error) // RS256 签名
func (tm *TokenManager) PublicJWKS() ([]byte, error)                // JWKS JSON（含 kid/kty/n/e）
```

### `pkg/auth/verifier.go`（验签侧，api 服务使用）

```go
type TokenVerifier struct {
    jwksURL  string
    keyCache map[string]*rsa.PublicKey // kid -> 公钥
    mu       sync.RWMutex
}

func NewTokenVerifier(jwksURL string) *TokenVerifier
func (tv *TokenVerifier) ValidateToken(token string) (Claims, error) // RS256 验签 + 过期校验 + kid 匹配
func (tv *TokenVerifier) refreshKeys(ctx context.Context) error       // 拉取 JWKS，按 kid 缓存公钥
```

### `proto/auth/v1/auth.proto`（对内 gRPC 契约）

```proto
service UserService {
  rpc GetUserInfos(GetUserInfosRequest) returns (GetUserInfosResponse);
}
message GetUserInfosRequest { repeated int64 user_ids = 1; }
message UserInfo {
  int64  id = 1;
  string username = 2;
  string nickname = 3;
  string avatar_url = 4;
  string role = 5;
}
```

### 配置扩展（internal/config/config.go）

```go
Auth struct {
    Kid      string `mapstructure:"kid"`      // 默认 "vistack-rs256"
    Issuer   string `mapstructure:"issuer"`   // 默认 "vistack"
    // 私钥经 VISTACK_AUTH_RSA_PRIVATE_KEY 环境变量注入（PEM），不进 toml
    JWTExpiration int `mapstructure:"jwt_expiration"` // 秒，沿用现有
    JWKSPath      string `mapstructure:"jwks_path"`   // 默认 /.well-known/jwks.json
}
AuthService struct {
    HTTPAddr string `mapstructure:"http_addr"` // 对外 HTTP，默认 :8081
    GRPCAddr string `mapstructure:"grpc_addr"` // 对内 gRPC，默认 :50052
}
```

## 模块设计

### 模块 A：auth 服务（role/auth.go，新）

**职责：** 对外认证 HTTP + 对内用户查询 gRPC + etcd 注册 + JWKS 端点。

**HTTP 路由**（迁自 api 并保持路径兼容）：
- `POST /api/v1/auth/register`、`POST /api/v1/auth/login`
- `GET /api/v1/user/info`、`PUT /api/v1/user/profile`、`PUT /api/v1/user/password`
- `GET /.well-known/jwks.json`

**gRPC：** 实现 `UserService.GetUserInfos`（按 user_id 列表批量返回公开信息）。

**启动序列：** InitDB（读写 users/user_profiles）→ 加载私钥 → InitMinio（头像 PublicURL 生成需要）→ 启动 gRPC server + etcd 注册 → 启动 HTTP server（信号优雅停机，复用 P0-1 的 http.Server Shutdown 模式）。

### 模块 B：JWT RS256/JWKS（pkg/auth，改）

- `TokenManager`：HS256 签名改 RS256；`GenerateToken` 用 `jwt.NewWithClaims(jwt.SigningMethodRS256, claims)`；`Claims` 增加 `kid` header（jwt v5 通过 `jwt.WithHeader` 或 Keyfunc 处理）。
- `TokenVerifier`：从 JWKS 拉取公钥，按 token header 的 `kid` 选公钥验签；公钥缓存 + 后台定期刷新（`refreshKeys`），支持密钥轮换（N2）。
- `Claims` 保持 `UserID + Exp`（RBAC 不做，不加 role/permission claim）。

### 模块 C：api 验签与用户域剥离（改）

- `AuthMiddleware` 改为注入 `*auth.TokenVerifier`（不再 `*auth.TokenManager`），本地验签，失败 401。
- 移除 `UserApi.Login/Register/GetUserInfo/UpdateProfileDirect/UpdateUserPassword` 及其路由（迁至 auth）；`global.TokenManager` 移除。
- `Video.go` 的 `Preload("User.Profile.Avatar")` 等 join 改为：收集视频作者的 userID 列表 → 调 auth gRPC `GetUserInfos` 批量查询 → 内存映射填充作者信息（昵称/头像/用户名/角色）。

### 模块 D：用户查询客户端（internal/authclient，新）

```go
type UserClient struct {
    pb   authpb.UserServiceClient
    conn *grpc.ClientConn
    etcd *clientv3.Client
}
func NewUserClient(ctx context.Context, cfg *config.AppConfig) (*UserClient, error) // etcd 发现（复用 discovery.EtcdBuilder）或静态地址
func (c *UserClient) GetUserInfos(ctx context.Context, ids []int64) (map[int64]*authpb.UserInfo, error)
```

### 模块 E：密钥管理

- 私钥来源：环境变量 `VISTACK_AUTH_RSA_PRIVATE_KEY`（PEM 内容，多行经 `\n` 转义）或 `VISTACK_AUTH_RSA_PRIVATE_KEY_FILE`（文件路径）。
- 开发模式：两者均未提供时自动生成 2048-bit RSA 密钥并日志告警（仅开发用，不落盘）。
- 生产：k8s Secret / 环境注入；公钥从私钥导出，无需单独存放。

## 模块交互（登录 + 作者展示两条链路）

```
[登录] 前端 → 网关(/api/v1/auth/*) → auth 服务 → DB 校验 → RS256 签发 token → 返回
[业务] 前端携带 token → api 服务 → AuthMiddleware(JWKS 验签, 本地) → 业务处理
[作者展示] api 处理视频列表 → 收集 userIDs → gRPC GetUserInfos → auth 服务查 DB → 返回 → api 填充作者
[注册] auth 服务向 etcd 注册(租约保活)；api 经 etcd resolver 发现 auth
```

## 文件组织

```
pkg/auth/
├── token_manager.go   — 改：HS256→RS256 签发 + JWKS 生成
├── verifier.go        — 新：JWKS 验签 + 公钥缓存
└── claim.go           — 不变（UserID+Exp）
internal/role/
├── auth.go            — 新：RunAuth（HTTP + gRPC + etcd 注册 + 信号停机）
├── api.go             — 改：注入 TokenVerifier；移除 auth/user handler 注册
├── worker.go          — 不变
└── migrate.go         — 不变
internal/authclient/
└── client.go          — 新：用户查询 gRPC 客户端（etcd 发现）
internal/api/v1/
├── User.go            — 删（Login/Register/GetUserInfo/UpdateProfile/UpdatePassword 迁至 auth）
└── Video.go           — 改：作者展示改 gRPC 批量查询
internal/auth/         — 新：auth 服务的 HTTP handler（register/login/userinfo/profile/password/jwks）+ gRPC service
internal/middlewares/auth.go — 改：JWKS 验签
internal/global/global.go     — 改：移除 TokenManager
internal/config/config.go     — 改：Auth/AuthService 段
proto/auth/v1/auth.proto      — 新：UserService 契约（buf 生成）
internal/transcoder/...       — 参考其 registry/discovery 复用
cmd/vistack/main.go           — 改：role 分发新增 "auth"
compose.yml / conf/*.toml     — 改：新增 auth 服务与配置
```

## 技术决策

| 决策点 | 选择 | 理由 |
|--------|------|------|
| auth 对外协议 | HTTP（Gin） | login/register 是面向前端的 REST，路径需兼容 |
| 对内用户查询 | gRPC + etcd 发现 | 复用 transcoder 已验证的 registry/discovery 范式，服务间同步调用走 gRPC |
| JWT 库 | 升级 `golang-jwt/jwt/v5` | 现用 `dgrijalva/jwt-go` 已废弃；v5 原生支持 RS256 + JWKS，迁移成本最低 |
| 验签模式 | api 本地验签（JWKS 公钥缓存） | 无状态、不回调 auth（F4），符合零信任 |
| 密钥注入 | 环境变量 PEM（私钥），开发模式自动生成 | 私钥不进 toml/镜像；开发零配置 |
| 用户展示 | gRPC 批量查询 + 内存映射 | 消除 N+1 与 join；数据归属 auth |
| 数据归属 | 共享 PG，auth 独占 users/user_profiles 读写，api 不再触碰 | 拆服务不拆库，边界清晰 |
| 密钥轮换 | kid + 公钥缓存刷新 + 新旧并存 | JWKS 天然支持多 key，轮换平滑 |
| 停机 | 复用 P0-1 的 http.Server Shutdown + gRPC GracefulStop | 与既有角色一致 |

## spec 覆盖检查

| spec 需求 | 落点 |
|-----------|------|
| F1 认证服务独立 | 模块 A：auth role |
| F2 RS256 签发 | 模块 B：TokenManager RS256 |
| F3 JWKS 分发 | 模块 A：JWKS 端点；模块 B：PublicJWKS |
| F4 本地验签 | 模块 C：TokenVerifier + AuthMiddleware |
| F5 用户数据归属 | 模块 A（auth 读写）+ 模块 D（api 经 gRPC 查） |
| F6 行为保持 | 模块 A：handler 迁移不改逻辑 |
| F7 路径兼容 | 模块 A：路径不变，网关按路径路由 |
| N1 密钥安全 | 模块 E：环境注入 |
| N2 密钥轮换 | 模块 B：kid + 缓存刷新 |
| N3 无状态扩展 | auth 无会话 + gRPC 无状态 |
| N4 配置兼容 | 新增配置有默认值，jwt_expiration 沿用 |
| N5 编译测试 | go build ./... |
