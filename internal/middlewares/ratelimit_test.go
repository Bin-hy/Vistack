package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/binhy/vistack/internal/middlewares/ratelimit"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestRateLimitMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// 注入带 user_id 的 claims，模拟鉴权后的上下文
	r.Use(func(c *gin.Context) {
		c.Set("claims", auth.Claims{UserID: 42})
		c.Next()
	})
	r.Use(RateLimit(ratelimit.NewTokenBucket(1, 2)))
	r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: want 200, got %d", i+1, w.Code)
		}
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("want Retry-After header")
	}
	for _, h := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if w.Header().Get(h) == "" {
			t.Fatalf("want %s header", h)
		}
	}
}

func TestRateLimitNilLimiterPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(nil))
	r.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (passthrough), got %d", w.Code)
	}
}
