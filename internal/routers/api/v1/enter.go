package v1

type RouterGroup struct {
	FileRouter
	VideoRouter
	SocialRouter
	DanmakuRouter
	CommentRouter
}

var RouterGroupApp = new(RouterGroup)
