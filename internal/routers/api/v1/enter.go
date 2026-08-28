package v1

type RouterGroup struct {
	FileRouter
	VideoRouter
	SocialRouter
	DanmakuRouter
}

var RouterGroupApp = new(RouterGroup)
