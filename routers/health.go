package routers

import (
    "context"
    "net/http"
    "time"

    "github.com/binhy/vistack/core"
    "github.com/gin-gonic/gin"
)

// RegisterHealth 健康检查与状态路由
func RegisterHealth(g *gin.RouterGroup) {
    g.GET("/ping", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"message": "pong"})
    })

    g.GET("/health", func(c *gin.Context) {
        status := gin.H{
            "time":   time.Now().Format(time.RFC3339),
            "server": "ok",
        }
        if core.DB != nil {
            if sqlDB, err := core.DB.DB(); err == nil {
                if err := sqlDB.Ping(); err != nil {
                    status["db"] = "error"
                    status["db_error"] = err.Error()
                } else {
                    status["db"] = "ok"
                }
            }
        } else {
            status["db"] = "skip"
        }

        if core.Minio != nil {
            // 简单调用以验证客户端可用
            ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
            defer cancel()
            _, err := core.Minio.ListBuckets(ctx)
            if err != nil {
                status["minio"] = "error"
                status["minio_error"] = err.Error()
            } else {
                status["minio"] = "ok"
            }
        } else {
            status["minio"] = "skip"
        }

        if core.Redis != nil {
            ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
            defer cancel()
            if err := core.Redis.Ping(ctx).Err(); err != nil {
                status["redis"] = "error"
                status["redis_error"] = err.Error()
            } else {
                status["redis"] = "ok"
            }
        } else {
            status["redis"] = "skip"
        }

        c.JSON(http.StatusOK, status)
    })
}