package routers

import (
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/middlewares"
	v1 "github.com/binhy/vistack/internal/routers/api/v1"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 统一注册所有子路由
func RegisterRoutes(r *gin.Engine) {
	// 公共路由组
	PublicGroup := r.Group("")
	{
		healthRouter := new(HealthRouter)
		healthRouter.InitHealthRouter(PublicGroup)
	}
	// API v1 路由组, 不需要认证路由
	PublicApiGroup := r.Group("/api/v1")
	{
		v1.RouterGroupApp.InitVideoPublicRouter(PublicApiGroup)
		v1.RouterGroupApp.InitAuthRouter(PublicApiGroup)
	}

	// API v1 路由组, 需要认证路由
	AuthApiGroup := r.Group("/api/v1")
	AuthApiGroup.Use(middlewares.AuthMiddleware(global.TokenManager))
	{
		v1.RouterGroupApp.InitUserRouter(AuthApiGroup)
		v1.RouterGroupApp.InitFileRouter(AuthApiGroup)
		v1.RouterGroupApp.InitVideoPrivatesRouter(AuthApiGroup)
	}
}
