package role

import (
	"context"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	mq_transcode "github.com/binhy/vistack/internal/core/message_queue/transcode"
	mq_video "github.com/binhy/vistack/internal/core/message_queue/video"
	"github.com/binhy/vistack/internal/transcoder"
	"go.uber.org/zap"
)

// RunWorker 启动 worker 角色：消费 Kafka、编排转码、写 DB、重试与 watchdog
func RunWorker(cfg *config.AppConfig) {
	core.InitDB(cfg)
	core.InitMinioClient(cfg)
	core.InitRedis(cfg)
	core.InitSnowflake(cfg)

	core.InitKafka(cfg)
	defer core.CloseKafka()

	// transcoder gRPC 客户端（etcd 发现或静态地址）
	client, err := transcoder.NewClient(context.Background(), cfg)
	if err != nil {
		core.Logger.Error("create transcoder client failed", zap.Error(err))
		panic(err)
	}
	defer client.Close()
	mq_transcode.SetTranscoderClient(client)

	ctx := context.Background()
	mq_transcode.StartTranscodeRetryDispatcher(ctx)
	mq_transcode.StartTranscodeWatchdog(ctx)
	mq_transcode.StartTranscodeWorker(ctx)
	mq_video.StartVideoDeleteWorker(ctx)

	// 常驻：所有 worker 均在各自 goroutine 中运行
	select {}
}
