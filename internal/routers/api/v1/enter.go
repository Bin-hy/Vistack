package v1

type RouterGroup struct {
	FileRouter
	VideoRouter
}

var RouterGroupApp = new(RouterGroup)
