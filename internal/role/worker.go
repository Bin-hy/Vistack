package role

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/core/leader"
	mq_comment "github.com/binhy/vistack/internal/core/message_queue/comment"
	mq_danmaku "github.com/binhy/vistack/internal/core/message_queue/danmaku"
	mq_transcode "github.com/binhy/vistack/internal/core/message_queue/transcode"
	mq_video "github.com/binhy/vistack/internal/core/message_queue/video"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/transcoder"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// drainTimeout 优雅停机排空在途任务的上限
const drainTimeout = 30 * time.Second

// RunWorker 启动 worker 角色：消费 Kafka、编排转码、写 DB、重试与 watchdog。
// 常规消费者并发运行；retry dispatcher 与 watchdog 通过 etcd 领导选举保证全局单例。
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

	// 信号感知：SIGINT/SIGTERM 触发优雅停机
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 常规消费者（并发消费，见 core.StartKafkaConsumer）
	mq_transcode.StartTranscodeWorker(ctx)
	mq_video.StartVideoDeleteWorker(ctx)
	mq_danmaku.StartDanmakuWorker(ctx)
	mq_comment.StartCommentModerationWorker(ctx)

	// 单例任务：etcd 领导选举，仅 leader 运行 dispatcher + watchdog
	runSingletonJobs(ctx)

	// 等待停机信号
	<-ctx.Done()
	if core.Logger != nil {
		core.Logger.Info("worker shutting down: stop consuming, draining in-flight tasks")
	}

	if !core.WaitKafkaConsumers(drainTimeout) {
		if core.Logger != nil {
			core.Logger.Warn("drain timeout, force exit", zap.Duration("timeout", drainTimeout))
		}
		os.Exit(1)
	}
	if core.Logger != nil {
		core.Logger.Info("worker exited cleanly")
	}
}

// runSingletonJobs 启动全局单例任务（retry dispatcher + watchdog）。
// etcd 可用时通过领导选举保证单例；不可用时降级直接运行（仅适合单实例，多实例有重复风险，记录告警）。
func runSingletonJobs(ctx context.Context) {
	start := func(c context.Context) {
		mq_transcode.StartTranscodeRetryDispatcher(c)
		mq_transcode.StartTranscodeWatchdog(c)
	}

	cfg := &global.AppConfig
	if len(cfg.Etcd.Endpoints) == 0 {
		if core.Logger != nil {
			core.Logger.Warn("etcd not configured, running singleton jobs directly (multi-replica unsafe)")
		}
		start(ctx)
		return
	}

	cli, err := clientv3.New(clientv3.Config{Endpoints: cfg.Etcd.Endpoints, DialTimeout: 5 * time.Second})
	if err != nil {
		if core.Logger != nil {
			core.Logger.Warn("etcd connect failed, degrade to direct singleton jobs (multi-replica unsafe)", zap.Error(err))
		}
		start(ctx)
		return
	}
	defer cli.Close()

	id := instanceID()
	elector := leader.New(cli, leader.DefaultLeaderKey, id, cfg.Etcd.LeaderTTL)
	if core.Logger != nil {
		core.Logger.Info("worker joining leader election",
			zap.String("key", leader.DefaultLeaderKey),
			zap.String("id", id),
		)
	}
	if err := elector.Run(ctx, func(leadCtx context.Context) {
		if core.Logger != nil {
			core.Logger.Info("became leader, starting singleton jobs")
		}
		start(leadCtx)
	}); err != nil {
		if core.Logger != nil {
			core.Logger.Error("leader election stopped", zap.Error(err))
		}
	}
}

// instanceID 返回实例标识：POD_IP 优先，回退 hostname
func instanceID() string {
	if id := os.Getenv("POD_IP"); id != "" {
		return id
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return fmt.Sprintf("instance-%d", time.Now().UnixNano())
}
