package role

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/transcoder"
	"go.uber.org/zap"
)

// RunTranscoder 启动 transcoder 角色：gRPC 服务 + ffmpeg，无状态、不连 DB
func RunTranscoder(cfg *config.AppConfig) {
	core.InitMinioClient(cfg)
	// 不初始化 DB / Redis / Kafka

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := transcoder.RunServer(ctx, cfg); err != nil {
		if core.Logger != nil {
			core.Logger.Error("transcoder server stopped", zap.Error(err))
		}
		panic(err)
	}
}
