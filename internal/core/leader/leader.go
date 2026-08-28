package leader

import (
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// DefaultLeaderKey 单例任务默认选举 key
const DefaultLeaderKey = "/vistack/leaders/worker-singleton"

// defaultTTL 未显式配置时的租约 TTL 秒数
const defaultTTL = 10

// defaultRetryDelay 竞选失败后的重试退避间隔
const defaultRetryDelay = 3 * time.Second

// Elector 基于 etcd 租约的领导选举器。
// 同一 key 下任意时刻只有一个实例是 leader，其余实例等待；leader 租约过期后自动转移。
type Elector struct {
	client *clientv3.Client
	key    string
	id     string
	ttl    int
}

// New 创建选举器。ttl 为租约秒数，<=0 时使用默认值 10。
func New(client *clientv3.Client, key, id string, ttl int) *Elector {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	return &Elector{client: client, key: key, id: id, ttl: ttl}
}

// Run 阻塞直到 ctx 结束：
//  1. 循环竞选，成为 leader 后以 leadCtx 调用 onElected；
//  2. 失去领导权（租约过期 / 被抢占）时 leadCtx 被取消，onElected 返回后自动重新竞选；
//  3. 外层 ctx 取消时退出循环并返回 nil。
//
// onElected 中启动的常驻 goroutine 应在 leadCtx 取消时退出。
func (e *Elector) Run(ctx context.Context, onElected func(leadCtx context.Context)) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		sess, err := concurrency.NewSession(e.client, concurrency.WithTTL(e.ttl))
		if err != nil {
			if !waitRetry(ctx) {
				return nil
			}
			continue
		}

		election := concurrency.NewElection(sess, e.key)
		if err := election.Campaign(ctx, e.id); err != nil {
			_ = sess.Close()
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		// 成为 leader：leadCtx 在 session 丢失（失去领导权）或外层 ctx 取消时取消
		leadCtx, cancel := context.WithCancel(ctx)
		sessDone := sess.Done()
		go func() {
			select {
			case <-sessDone:
				cancel()
			case <-leadCtx.Done():
			}
		}()

		onElected(leadCtx)

		// 等待 leadCtx 结束（失去领导权 / 外层取消），然后释放租约进入下一轮
		<-leadCtx.Done()
		cancel()
		_ = sess.Close()

		if ctx.Err() != nil {
			return nil
		}
	}
}

// waitRetry 等待重试间隔；外层 ctx 取消时返回 false
func waitRetry(ctx context.Context) bool {
	t := time.NewTimer(defaultRetryDelay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
