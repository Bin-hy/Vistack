package middlewares

import (
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

// RequestID 在响应头与上下文中注入请求ID
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := uuid.NewString()
        c.Set("request_id", id)
        c.Writer.Header().Set("X-Request-ID", id)
        c.Next()
    }
}