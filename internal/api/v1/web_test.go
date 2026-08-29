package v1

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRegisterWebStatic 验证前端静态托管（用户端 + 管理端）：
//   - / 与静态资源正常返回（用户端）
//   - /admin 子路径返回管理端（含其静态资源与 SPA 回退）
//   - SPA 客户端路由（如 /video/123）回退到 index.html
//   - 显式 API 路由优先于静态兜底
//   - 未匹配的 /api/* 保持 404，不返回 HTML
func TestRegisterWebStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 构造最小前端产物目录：用户端 + 管理端
	webDir := t.TempDir()
	mustWrite(t, filepath.Join(webDir, "index.html"), "<html>spa-client</html>")
	mustWrite(t, filepath.Join(webDir, "logo.png"), "png-bytes")
	mustWrite(t, filepath.Join(webDir, "assets", "app.js"), "console.log('client')")

	adminDir := t.TempDir()
	mustWrite(t, filepath.Join(adminDir, "index.html"), "<html>spa-admin</html>")
	mustWrite(t, filepath.Join(adminDir, "assets", "admin.js"), "console.log('admin')")

	r := gin.New()
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	RegisterWebStatic(r, webDir, adminDir)

	cases := []struct {
		name string
		path string
		code int
		body string
	}{
		{"用户端根路径", "/", 200, "<html>spa-client</html>"},
		{"用户端静态资源", "/assets/app.js", 200, "console.log('client')"},
		{"用户端 public 文件", "/logo.png", 200, "png-bytes"},
		{"用户端 SPA 回退", "/video/123", 200, "<html>spa-client</html>"},
		{"用户端深层 SPA", "/user/profile/settings", 200, "<html>spa-client</html>"},
		{"管理端根路径", "/admin", 200, "<html>spa-admin</html>"},
		{"管理端带斜杠", "/admin/", 200, "<html>spa-admin</html>"},
		{"管理端静态资源", "/admin/assets/admin.js", 200, "console.log('admin')"},
		{"管理端 SPA 回退", "/admin/sensitive-words", 200, "<html>spa-admin</html>"},
		{"显式 API 路由优先", "/health", 200, "ok"},
		{"未匹配 API 保持 404", "/api/v1/unknown", 404, ""},
		{"管理端下 API 保持 404", "/admin/api/v1/unknown", 404, ""},
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

// TestRegisterWebStaticEmptyDir web_dir / admin_web_dir 为空或不存在时不注册静态路由
func TestRegisterWebStaticEmptyDir(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	RegisterWebStatic(r, "", "/no/such/dir")

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
