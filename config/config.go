package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

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
	} `mapstructure:"database"`

	MinIO struct {
		Endpoint  string `mapstructure:"endpoint"`
		AccessKey string `mapstructure:"access_key"`
		SecretKey string `mapstructure:"secret_key"`
		Secure    bool   `mapstructure:"secure"` // 对应 app.toml 的 secure
		Bucket    string `mapstructure:"bucket"`
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
}

// Load 读取配置（conf/app.toml + 环境变量覆盖）
func Load(configPath string) AppConfig {
	v := viper.New()
	v.SetConfigName("app")
	v.AddConfigPath("conf")
	v.SetConfigType("toml")
	v.SetEnvPrefix("VISTACK")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 默认值
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("logging.level", "info")
	v.SetDefault("auth.jwt_expiration", 3600)
	v.SetDefault("redis.host", "127.0.0.1")
	v.SetDefault("redis.port", 6379)
	v.SetDefault("redis.db", 0)
	v.SetDefault("redis.pool_size", 10)

	// 判断是否指定了自定义路径
	if configPath != "" {
		v.SetConfigFile(configPath)
		fmt.Println("[config] using file:", configPath)
	} else {
		v.SetConfigName("app")
		v.AddConfigPath("conf")
		fmt.Println("[config] using default conf/app.toml (if exists)")
	}

	if err := v.ReadInConfig(); err != nil {
		fmt.Println("[config] no config file found, using defaults & env overrides:", err)
	}

	var cfg AppConfig
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("failed to unmarshal config: %w", err))
	}
	return cfg
}
