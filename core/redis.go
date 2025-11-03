package core

import (
	"fmt"
	"time"

	"github.com/binhy/vistack/config"
	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

// InitRedis 初始化 Redis 客户端（可选）
func InitRedis(cfg *config.AppConfig) {
	if cfg.Redis.Host == "" {
		if Logger != nil {
			Logger.Warn("Redis host is empty, skip Redis initialization")
		}
		return
	}
	addr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
	Redis = redis.NewClient(&redis.Options{
		Addr:         addr,
		DB:           cfg.Redis.DB,
		Password:     cfg.Redis.Password,
		PoolSize:     cfg.Redis.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
}
