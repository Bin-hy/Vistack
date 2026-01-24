package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type VideoRouter struct{}

func (v *VideoRouter) InitVideoPrivatesRouter(Router *gin.RouterGroup) {
	videoApi := new(v1.VideoApi)
	videoRouter := Router.Group("/videos")
	{
		// 视频上传流程
		videoRouter.POST("/upload/init", videoApi.InitVideoUpload)
		videoRouter.GET("/upload/sign", videoApi.GetUploadPartURL)
		videoRouter.GET("/upload/parts", videoApi.ListUploadedParts)
		videoRouter.POST("/upload/complete", videoApi.CompleteVideoUpload)

		// 视频管理
		videoRouter.DELETE("/:id", videoApi.DeleteVideo)
		videoRouter.PUT("/:id", videoApi.PutVideoInfo)
		// 创作空间视频管理
		videoRouter.GET("/plateform/list", videoApi.GetSelfVideoPage)
	}
}

func (v *VideoRouter) InitVideoPublicRouter(Router *gin.RouterGroup) {
	videoApi := new(v1.VideoApi)
	videoRouter := Router.Group("/videos")
	{
		// 视频播放
		videoRouter.GET("/:id/manifest.mpd", videoApi.GetVideoMdp)
		videoRouter.GET("/:id/segments/signature", videoApi.GetVideoSegmentsSignature)

		// 视频信息
		videoRouter.GET("/:id/info", videoApi.GetVideoInfo)

		// 用户视频推荐
		videoRouter.GET("/recommend", videoApi.GetVideoRecommend)

	}
}
