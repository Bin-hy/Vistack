package v1

type RouterGroup struct {
	FileRouter
	VideoRouter
	SocialRouter
}

var RouterGroupApp = new(RouterGroup)
