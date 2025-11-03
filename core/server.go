package core

import (
	"time"

	"github.com/binhy/vistack/middlewares"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
)

// NewServer 创建并返回 Gin 引擎，挂载 zap 日志与通用中间件
func NewServer() *gin.Engine {
	r := gin.New()
	if Logger != nil {
		r.Use(ginzap.Ginzap(Logger, time.RFC3339, true))
		r.Use(ginzap.RecoveryWithZap(Logger, true))
	} else {
		r.Use(gin.Logger(), gin.Recovery())
	}

	// 通用中间件
	r.Use(middlewares.RequestID())

	return r
}
