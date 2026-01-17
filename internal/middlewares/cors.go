package middlewares

import (
	"net/http"
	"strings"

	"github.com/binhy/vistack/internal/global"
	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	cfg := global.AppConfig.Cors
	if !cfg.Enable {
		return func(c *gin.Context) {
			c.Next()
		}
	}
	allowOrigins := cfg.AllowOrigins
	allowMethods := cfg.AllowMethods
	allowHeaders := cfg.AllowHeaders
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}
		allowedOrigin := ""
		if len(allowOrigins) == 0 {
			allowedOrigin = "*"
		} else {
			for _, o := range allowOrigins {
				if o == "*" {
					allowedOrigin = "*"
					break
				}
				if strings.EqualFold(o, origin) {
					allowedOrigin = origin
					break
				}
			}
		}
		if allowedOrigin == "" {
			c.Next()
			return
		}
		if cfg.AllowCredentials && allowedOrigin == "*" {
			allowedOrigin = origin
		}
		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if len(allowMethods) == 0 {
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		} else {
			c.Header("Access-Control-Allow-Methods", strings.Join(allowMethods, ","))
		}
		if len(allowHeaders) == 0 {
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
		} else {
			c.Header("Access-Control-Allow-Headers", strings.Join(allowHeaders, ","))
		}
		c.Header("Access-Control-Expose-Headers", "Content-Length,Content-Type,Date,Server")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
