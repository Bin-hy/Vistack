package main

import (
	"flag"
	"fmt"

	"github.com/binhy/vistack/config"
	"github.com/binhy/vistack/core"
	"github.com/binhy/vistack/migrations"
	"github.com/binhy/vistack/routers"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// 解析 config 路径参数
	configPath := flag.String("config", "", "path to config file (e.g., go run . --config conf/dev.toml)")
	flag.Parse()
	// 加载配置
	cfg := config.Load(*configPath)

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
	core.InitMinio(&cfg)

	// 初始化 Redis（如果提供配置）
	core.InitRedis(&cfg)

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
