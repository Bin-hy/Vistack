package config

// AppConfig 定义应用配置结构（对接 conf/app.toml 的结构）
type AppConfig struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port int    `mapstructure:"port"`
		Mode string `mapstructure:"mode"` // debug/release/test
	} `mapstructure:"server"`

	Logging struct {
		Level string `mapstructure:"level"` // debug/info/warn/error
	} `mapstructure:"logging"`

	// database: 支持（host, port, user, password, name）或 dsn。若 dsn 提供则优先使用 dsn。
	Database struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
		DSN      string `mapstructure:"dsn"`
		// Connection Pool
		MaxIdleConns    int `mapstructure:"max_idle_conns"`
		MaxOpenConns    int `mapstructure:"max_open_conns"`
		ConnMaxLifetime int `mapstructure:"conn_max_lifetime"` // 秒
	} `mapstructure:"database"`

	MinIO struct {
		Endpoint       string `mapstructure:"endpoint"`
		PublicEndpoint string `mapstructure:"public_endpoint"`
		AccessKey      string `mapstructure:"access_key"`
		SecretKey      string `mapstructure:"secret_key"`
		Secure         bool   `mapstructure:"secure"` // 对应 app.toml 的 secure
		Bucket         string `mapstructure:"bucket"`
	} `mapstructure:"minio"`

	Auth struct {
		Kid           string `mapstructure:"kid"`            // JWT key id，默认 vistack-rs256
		Issuer        string `mapstructure:"issuer"`         // JWT issuer，默认 vistack
		JWTExpiration int    `mapstructure:"jwt_expiration"` // 秒
		JWKSPath      string `mapstructure:"jwks_path"`      // JWKS 端点路径，默认 /.well-known/jwks.json
	} `mapstructure:"auth"`

	AuthService struct {
		HTTPAddr string `mapstructure:"http_addr"` // auth 对外 HTTP 地址，默认 :8081
		GRPCAddr string `mapstructure:"grpc_addr"` // auth 对内 gRPC 地址，默认 :50052
		JWKSURL  string `mapstructure:"jwks_url"`  // api 拉取 JWKS 的完整 URL，默认 http://127.0.0.1:8081/.well-known/jwks.json
	} `mapstructure:"auth_service"`

	Redis struct {
		Host     string `mapstructure:"host"`
		Port     int    `mapstructure:"port"`
		DB       int    `mapstructure:"db"`
		Password string `mapstructure:"password"`
		PoolSize int    `mapstructure:"pool_size"`
	} `mapstructure:"redis"`

	Snowflake struct {
		NodeID int64 `mapstructure:"node_id"`
	} `mapstructure:"snowflake"`

	Kafka struct {
		Brokers     []string `mapstructure:"brokers"`
		GroupID     string   `mapstructure:"group_id"`
		Concurrency int      `mapstructure:"concurrency"` // 每个实例并发消费者数，默认 4
	} `mapstructure:"kafka"`

	Cors struct {
		Enable           bool     `mapstructure:"enable"`
		AllowOrigins     []string `mapstructure:"allow_origins"`
		AllowMethods     []string `mapstructure:"allow_methods"`
		AllowHeaders     []string `mapstructure:"allow_headers"`
		AllowCredentials bool     `mapstructure:"allow_credentials"`
	} `mapstructure:"cors"`

	Etcd struct {
		Endpoints []string `mapstructure:"endpoints"` // 如 ["localhost:2379"]
		Prefix    string   `mapstructure:"prefix"`    // 注册前缀，默认 /vistack/transcoders
		LeaderTTL int      `mapstructure:"leader_ttl"` // 领导选举租约 TTL 秒，默认 10
	} `mapstructure:"etcd"`

	Transcoder struct {
		ListenAddr string `mapstructure:"listen_addr"` // transcoder 绑定地址，如 :50051
		Addr       string `mapstructure:"addr"`        // worker 静态兜底地址，如 localhost:50051
		UseEtcd    bool   `mapstructure:"use_etcd"`    // worker 是否通过 etcd 发现
	} `mapstructure:"transcoder"`
}

const DefaultConfigPath = "conf/app.local.toml"

// DefaultAuthPrefix auth 服务在 etcd 中的注册/发现前缀（注册方与发现方共用，须一致）。
const DefaultAuthPrefix = "/vistack/auth"
