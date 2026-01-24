package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"github.com/redis/go-redis/v9"
)

const retryZSetKey = "transcode:retry:zset"

type TranscodeRetryMessage = TranscodeMessage

// scheduleDelay 计算重试延迟时间，指数退避策略:
// 每次重试时间翻倍，最大重试时间8小时
func scheduleDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := time.Minute
	d := time.Duration(1<<uint(attempt-1)) * base
	if d > 8*time.Hour {
		d = 8 * time.Hour
	}
	j := time.Duration(rand.Int63n(int64(d / 5)))
	return d + j
}

// AddTranscodeRetry 添加转码重试任务到延迟队列
func AddTranscodeRetry(ctx context.Context, msg TranscodeRetryMessage) error {
	delay := scheduleDelay(msg.Attempt)
	score := float64(time.Now().Add(delay).Unix())
	b, _ := json.Marshal(msg)
	return core.Redis.ZAdd(ctx, retryZSetKey, redis.Z{Score: score, Member: string(b)}).Err()
}

// StartTranscodeRetryDispatcher 转码重试派发器
// 使用Redis ZSet 做延迟队列，定时扫描转码重试任务，再回投给Kafaka，让消费者重新转码
func StartTranscodeRetryDispatcher(ctx context.Context) {
	ticker := time.NewTicker(time.Second * 5) // 每间隔5s
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				now := float64(time.Now().Unix())
				items, err := core.Redis.ZRangeByScore(ctx, retryZSetKey, &redis.ZRangeBy{
					Min:    "-inf",
					Max:    fmt.Sprintf("%f", now),
					Offset: 0,
					Count:  100,
				}).Result()
				if err != nil || len(items) == 0 {
					continue
				}
				for _, s := range items {
					var m TranscodeRetryMessage
					if json.Unmarshal([]byte(s), &m) != nil {
						_ = core.Redis.ZRem(ctx, retryZSetKey, s).Err()
						continue
					}
					b, _ := json.Marshal(m)
					// 重新加入到消息队列中等待消费
					_ = core.SendKafkaMessage(ctx, string(consts.KafkaTopicTranscode), strconv.FormatInt(m.VideoID, 10), b)
					_ = core.Redis.ZRem(ctx, retryZSetKey, s).Err()
				}
			}
		}
	}()
}
