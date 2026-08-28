package v1

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRegisterWebStatic 验证前端静态托管：
//   - / 与静态资源正常返回
//   - SPA 客户端路由（如 /video/123）回退到 index.html
//   - 显式 API 路由优先于静态兜底
//   - 未匹配的 /api/* 保持 404，不返回 HTML
func TestRegisterWebStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构造最小前端产物目录
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "index.html"), "<html>spa-index</html>")
	mustWrite(t, filepath.Join(dir, "logo.png"), "png-bytes")
	mustWrite(t, filepath.Join(dir, "assets", "app.js"), "console.log(1)")

	r := gin.New()
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	RegisterWebStatic(r, dir)

	cases := []struct {
		name string
		path string
		code int
		body string
	}{
		{"根路径", "/", 200, "<html>spa-index</html>"},
		{"静态资源", "/assets/app.js", 200, "console.log(1)"},
		{"public 文件", "/logo.png", 200, "png-bytes"},
		{"SPA 客户端路由回退", "/video/123", 200, "<html>spa-index</html>"},
		{"深层 SPA 路由", "/user/profile/settings", 200, "<html>spa-index</html>"},
		{"显式 API 路由优先", "/health", 200, "ok"},
		{"未匹配 API 保持 404", "/api/v1/unknown", 404, ""},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != c.code {
			t.Errorf("%s: code = %d, want %d (body=%q)", c.name, w.Code, c.code, w.Body.String())
			continue
		}
		if c.body != "" && w.Body.String() != c.body {
			t.Errorf("%s: body = %q, want %q", c.name, w.Body.String(), c.body)
		}
	}
}

// TestRegisterWebStaticEmptyDir web_dir 为空或目录不存在时不注册静态路由
func TestRegisterWebStaticEmptyDir(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	RegisterWebStatic(r, "")            // 空路径
	RegisterWebStatic(r, "/no/such/dir") // 不存在

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("without web dir, GET / should be 404, got %d", w.Code)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
