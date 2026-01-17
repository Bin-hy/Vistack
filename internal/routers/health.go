package routers

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type HealthRouter struct{}

// RegisterHealth 健康检查与状态路由

func (s *HealthRouter) InitHealthRouter(Router *gin.RouterGroup) *gin.RouterGroup {
	healthApi := new(v1.HealthApi)
	healthGroup := Router.Group("/")
	{
		healthGroup.GET("/ping", healthApi.Ping)
		healthGroup.GET("/health", healthApi.HealthCheck)
	}
	return healthGroup
}
