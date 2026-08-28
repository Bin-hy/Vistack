package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type DanmakuRouter struct{}

// InitDanmakuPublicRouter 公开路由：按时间范围拉取弹幕。
func (d *DanmakuRouter) InitDanmakuPublicRouter(Router *gin.RouterGroup) {
	danmakuApi := new(v1.DanmakuApi)
	danmakuRouter := Router.Group("/videos")
	{
		danmakuRouter.GET("/:id/danmaku", danmakuApi.GetDanmaku)
	}
}

// InitDanmakuPrivatesRouter 受保护路由：发送弹幕 + 敏感词管理。
func (d *DanmakuRouter) InitDanmakuPrivatesRouter(Router *gin.RouterGroup) {
	danmakuApi := new(v1.DanmakuApi)
	sensitiveApi := new(v1.SensitiveWordApi)
	videoRouter := Router.Group("/videos")
	{
		videoRouter.POST("/:id/danmaku", danmakuApi.SendDanmaku)
	}
	adminRouter := Router.Group("/admin")
	{
		adminRouter.GET("/sensitive-words", sensitiveApi.ListSensitiveWords)
		adminRouter.POST("/sensitive-words", sensitiveApi.AddSensitiveWord)
		adminRouter.DELETE("/sensitive-words/:id", sensitiveApi.DeleteSensitiveWord)
	}
}
