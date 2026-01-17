package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type UserRouter struct {
}

func (*UserRouter) InitUserRouter(Router *gin.RouterGroup) (R gin.IRoutes) {
	userApi := new(v1.UserApi)
	userRouter := Router.Group("/user")
	{
		userRouter.GET("/info", userApi.GetUserInfo)
		userRouter.PUT("/profile", userApi.UpdateProfileDirect)
		// userRouter.POST("/profile/update", userApi.UpdateProfileDirect)
		userRouter.PUT("/password", userApi.UpdateUserPassword)
	}
	return userRouter
}
