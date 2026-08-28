package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const slidingWindowKeyPrefix = "vistack:ratelimit:"

// slidingWindowScript 原子执行：清理窗口外旧记录 → 计数 → 未超限则写入。
// 返回 {allowed, remaining, reset_ms}。
var slidingWindowScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - window)
local count = redis.call('ZCARD', KEYS[1])

if count < limit then
	redis.call('ZADD', KEYS[1], now, ARGV[4])
	redis.call('PEXPIRE', KEYS[1], window)
	return {1, limit - count - 1, now + window}
end

local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
local resetAt = now + window
if oldest[2] then
	resetAt = tonumber(oldest[2]) + window
end
return {0, 0, resetAt}
`)

// SlidingWindow 基于 Redis ZSet 的分布式滑动窗口限流（跨实例共享窗口）。
type SlidingWindow struct {
	client *redis.Client
	window time.Duration
	limit  int
}

// NewSlidingWindow 构造滑动窗口。window=窗口时长，limit=窗口内最大请求数。
func NewSlidingWindow(client *redis.Client, window time.Duration, limit int) *SlidingWindow {
	if window <= 0 {
		window = time.Minute
	}
	if limit <= 0 {
		limit = 100
	}
	return &SlidingWindow{client: client, window: window, limit: limit}
}

// Allow 按 key 判定是否放行。ResetAt 为「窗口内最早记录过期、可再放行」的时间。
func (sw *SlidingWindow) Allow(ctx context.Context, key string) (Result, error) {
	now := time.Now()
	nowMs := now.UnixMilli()
	windowMs := sw.window.Milliseconds()
	member := uuid.NewString() // 全局唯一，避免同一毫秒多请求被 ZSet 去重合并

	vals, err := slidingWindowScript.Run(
		ctx,
		sw.client,
		[]string{slidingWindowKeyPrefix + key},
		nowMs,
		windowMs,
		sw.limit,
		member,
	).Int64Slice()
	if err != nil {
		return Result{}, err
	}
	if len(vals) != 3 {
		return Result{}, fmt.Errorf("unexpected sliding window result: %v", vals)
	}

	return Result{
		Allowed:   vals[0] == 1,
		Limit:     sw.limit,
		Remaining: int(vals[1]),
		ResetAt:   time.UnixMilli(vals[2]),
	}, nil
}
