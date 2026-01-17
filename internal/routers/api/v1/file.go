package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type FileRouter struct{}

func (f *FileRouter) InitFileRouter(Router *gin.RouterGroup) {
	fileApi := new(v1.FileApi)
	fileRouter := Router.Group("/file")
	{
		fileRouter.POST("/avatar", fileApi.AvatarUpload)
		fileRouter.POST("/cover", fileApi.CoverUpload)
	}
}
