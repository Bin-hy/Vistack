package role

import (
	"fmt"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/routers"
	"github.com/binhy/vistack/migrations"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RunAPI 启动 api 角色：HTTP 服务 + 上传/预签名 + 投递 Kafka 消息
func RunAPI(cfg *config.AppConfig) {
	core.InitDB(cfg)
	if core.DB != nil {
		if err := migrations.AutoMigrate(core.DB); err != nil {
			if core.Logger != nil {
				core.Logger.Error("gorm automigrate failed", zap.Error(err))
			}
		}
	}
	core.InitMinioClient(cfg)
	core.InitRedis(cfg)
	core.InitSnowflake(cfg)

	global.TokenManager = auth.NewTokenManager(cfg.Auth.JWTSecret, uint64(cfg.Auth.JWTExpiration))

	core.InitKafka(cfg)
	defer core.CloseKafka()

	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	r := core.NewServer()
	routers.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if err := r.Run(addr); err != nil {
		panic(err)
	}
}
