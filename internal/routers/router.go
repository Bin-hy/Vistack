package routers

import (
	"github.com/binhy/vistack/internal/middlewares"
	v1 "github.com/binhy/vistack/internal/routers/api/v1"
	authpkg "github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes 统一注册所有子路由。
// validator 用于受保护路由的本地验签（api 用 JWKS TokenVerifier）。
// 认证与用户资料路由已迁至 auth 服务，此处不再注册。
func RegisterRoutes(r *gin.Engine, validator authpkg.TokenValidator) {
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
	}

	// API v1 路由组, 需要认证路由
	AuthApiGroup := r.Group("/api/v1")
	AuthApiGroup.Use(middlewares.AuthMiddleware(validator))
	{
		v1.RouterGroupApp.InitFileRouter(AuthApiGroup)
		v1.RouterGroupApp.InitVideoPrivatesRouter(AuthApiGroup)
	}
}
