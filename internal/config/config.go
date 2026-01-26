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
		JWTSecret     string `mapstructure:"jwt_secret"`
		JWTExpiration int    `mapstructure:"jwt_expiration"` // 秒
	} `mapstructure:"auth"`

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
		Brokers []string `mapstructure:"brokers"`
		GroupID string   `mapstructure:"group_id"`
	} `mapstructure:"kafka"`

	Cors struct {
		Enable           bool     `mapstructure:"enable"`
		AllowOrigins     []string `mapstructure:"allow_origins"`
		AllowMethods     []string `mapstructure:"allow_methods"`
		AllowHeaders     []string `mapstructure:"allow_headers"`
		AllowCredentials bool     `mapstructure:"allow_credentials"`
	} `mapstructure:"cors"`
}

const DefaultConfigPath = "conf/app.local.toml"
