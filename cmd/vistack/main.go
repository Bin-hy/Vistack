package main

import (
	"context"
	"fmt"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/routers"
	"github.com/binhy/vistack/migrations"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {

	core.Viper() // 初始化 Viper 并解析配置
	cfg := global.AppConfig
	// 初始化日志
	core.InitLogger(&cfg)
	defer core.SyncLogger()

	// 初始化数据库（如果提供配置）
	core.InitDB(&cfg)

	// 执行 GORM 自动迁移（如已初始化数据库）
	if core.DB != nil {
		if err := migrations.AutoMigrate(core.DB); err != nil {
			if core.Logger != nil {
				core.Logger.Error("gorm automigrate failed", zap.Error(err))
			} else {
				fmt.Println("gorm automigrate failed:", err)
			}
		}
	}

	// 初始化 MinIO（如果提供配置）
	core.InitMinioClient(&cfg)

	// 初始化 Redis（如果提供配置）
	core.InitRedis(&cfg)

	// 初始化 Snowflake
	core.InitSnowflake(&cfg)

	// 初始化 TokenManager
	global.TokenManager = auth.NewTokenManager(cfg.Auth.JWTSecret, uint64(cfg.Auth.JWTExpiration))

	// 初始化 Kafka 生产者
	core.InitKafka(&cfg)
	defer core.CloseKafka()

	// 启动转码 Worker (异步消费 Kafka 消息)
	// 在实际生产中，Worker 通常作为独立服务部署
	// 使用 context.Background()，如果需要优雅退出可结合 signal.Notify
	core.StartTranscodeWorker(context.Background())

	// 设置 Gin 运行模式
	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	// 创建服务并挂载中间件
	r := core.NewServer()

	// 注册子路由
	routers.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}
