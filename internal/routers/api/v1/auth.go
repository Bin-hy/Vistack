package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type AuthRouter struct{}

// InitAuthRouter 认证相关路由
func (s *AuthRouter) InitAuthRouter(Router *gin.RouterGroup) (R gin.IRoutes) {
	userApi := new(v1.UserApi)
	authRouter := Router.Group("/auth")
	{
		authRouter.POST("/login", userApi.Login)
		authRouter.POST("/register", userApi.Register)
	}
	return authRouter
}
