package v1

import (
	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/gin-gonic/gin"
)

type CommentRouter struct{}

// InitCommentPublicRouter 公开路由：读评论列表 / 展开回复 / 评论总数。
func (cr *CommentRouter) InitCommentPublicRouter(Router *gin.RouterGroup) {
	commentApi := new(v1.CommentApi)
	videoRouter := Router.Group("/videos")
	{
		videoRouter.GET("/:id/comments", commentApi.ListComments)
		videoRouter.GET("/:id/comments/count", commentApi.CommentCount)
	}
	commentRouter := Router.Group("/comments")
	{
		commentRouter.GET("/:id/replies", commentApi.ListReplies)
	}
}

// InitCommentPrivatesRouter 受保护路由：发表评论/回复、点赞、删除。
func (cr *CommentRouter) InitCommentPrivatesRouter(Router *gin.RouterGroup) {
	commentApi := new(v1.CommentApi)
	videoRouter := Router.Group("/videos")
	{
		videoRouter.POST("/:id/comments", commentApi.CreateComment)
	}
	commentRouter := Router.Group("/comments")
	{
		commentRouter.POST("/:id/like", commentApi.ToggleLike)
		commentRouter.DELETE("/:id", commentApi.DeleteComment)
	}
}
