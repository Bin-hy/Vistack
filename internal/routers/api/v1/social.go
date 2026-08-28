package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type SocialRouter struct{}

// InitSocialPublicRouter 公开路由：播放上报 / 计数 / 榜单。
func (s *SocialRouter) InitSocialPublicRouter(Router *gin.RouterGroup) {
	socialApi := new(v1.SocialApi)
	socialRouter := Router.Group("/videos")
	{
		socialRouter.POST("/:id/play", socialApi.PlayVideo)
		socialRouter.GET("/:id/stats", socialApi.GetVideoStats)
		socialRouter.GET("/hot", socialApi.GetHotVideos)
	}
}

// InitSocialPrivatesRouter 受保护路由：点赞 / 收藏 / 状态。
func (s *SocialRouter) InitSocialPrivatesRouter(Router *gin.RouterGroup) {
	socialApi := new(v1.SocialApi)
	socialRouter := Router.Group("/videos")
	{
		socialRouter.POST("/:id/like", socialApi.LikeVideo)
		socialRouter.POST("/:id/favorite", socialApi.FavoriteVideo)
		socialRouter.GET("/:id/interaction", socialApi.GetVideoInteraction)
	}
}
