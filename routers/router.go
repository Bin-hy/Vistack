package routers

import "github.com/gin-gonic/gin"

// RegisterRoutes 统一注册所有子路由
func RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1")
	{
		RegisterHealth(api)
		RegisterAuth(api)
	}
}
