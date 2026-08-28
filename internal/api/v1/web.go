package v1

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaFileSystem 支持 SPA 前端路由回退的静态文件系统。
// 目录原样返回（由 http.FileServer 自行查找 index.html）；
// 文件不存在时回退到 index.html；/api/ 前缀的未匹配路径保持 404。
type spaFileSystem struct {
	dir       http.FileSystem
	indexPath string
}

func (s *spaFileSystem) Open(name string) (http.File, error) {
	// API 未匹配路径保持 404 语义，不参与 SPA 回退
	if strings.HasPrefix(name, "/api/") {
		return nil, os.ErrNotExist
	}

	if f, err := s.dir.Open(name); err == nil {
		return f, nil
	}

	// SPA 前端路由回退到 index.html（如 /video/123 刷新后仍渲染应用）
	return s.dir.Open(s.indexPath)
}

// RegisterWebStatic 托管前端构建产物（Vite 输出目录）。
// 仅在 webDir 非空且目录存在时生效；已注册的 /api/v1 等显式路由优先，
// 未匹配请求（页面、/assets/*、SPA 客户端路由）由 NoRoute 兜底交给静态服务。
func RegisterWebStatic(r *gin.Engine, webDir string) {
	if webDir == "" {
		return
	}
	abs, err := filepath.Abs(webDir)
	if err != nil {
		return
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return
	}

	fs := &spaFileSystem{dir: http.Dir(abs), indexPath: "/index.html"}
	fileServer := http.FileServer(fs)
	r.NoRoute(func(c *gin.Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
