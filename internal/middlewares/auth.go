package middlewares

import (
	"net/http"
	"strings"

	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 校验 JWT 有效性，并将解析后的 claims 写入 Gin 上下文
// 使用方式：在路由初始化时注入 TokenManager，例如：
//
//	tm := auth.NewTokenManager(secret, expireSeconds)
//	r.Use(middlewares.AuthMiddleware(tm))
func AuthMiddleware(tm *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) 获取 Authorization 头
		raw := c.GetHeader("Authorization")
		if raw == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权", "details": "缺少 Authorization 头"})
			c.Abort()
			return
		}

		// 2) 兼容 Bearer 前缀（大小写不敏感）
		token := strings.TrimSpace(raw)
		lower := strings.ToLower(token)
		if strings.HasPrefix(lower, "bearer ") {
			token = strings.TrimSpace(token[7:])
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权", "details": "无效的 Authorization 格式"})
			c.Abort()
			return
		}

		// 3) 验证 token 并获取 claims
		claims, err := tm.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权", "details": err.Error()})
			c.Abort()
			return
		}

		// 4) 注入 claims 到上下文
		c.Set("claims", claims)
		c.Next()
	}
}
