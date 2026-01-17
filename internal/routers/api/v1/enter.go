package v1

type RouterGroup struct {
	UserRouter
	AuthRouter
	FileRouter
	VideoRouter
}

var RouterGroupApp = new(RouterGroup)
