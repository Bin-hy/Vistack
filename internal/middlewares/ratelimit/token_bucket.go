package ratelimit

import (
	"context"
	"math"
	"sync"
	"time"
)

// TokenBucket 进程内令牌桶（单机限流），per-key 独立桶。
type TokenBucket struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewTokenBucket 构造令牌桶。rate=每秒补充令牌数，burst=桶容量。
func NewTokenBucket(rate, burst int) *TokenBucket {
	if rate <= 0 {
		rate = 1
	}
	if burst <= 0 {
		burst = rate
	}
	return &TokenBucket{
		rate:    float64(rate),
		burst:   float64(burst),
		buckets: make(map[string]*bucket),
	}
}

// Allow 按 key 判定是否放行。ResetAt 为「下一次可得令牌」的时间。
func (tb *TokenBucket) Allow(ctx context.Context, key string) (Result, error) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	b, ok := tb.buckets[key]
	if !ok {
		b = &bucket{tokens: tb.burst, last: now}
		tb.buckets[key] = b
	}

	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(tb.burst, b.tokens+elapsed*tb.rate)
	b.last = now

	// 下一次可得令牌的等待时间（无令牌时为 (1-tokens)/rate 秒）
	resetIn := time.Duration(math.Max(0, (1-b.tokens)/tb.rate) * float64(time.Second))

	if b.tokens >= 1 {
		b.tokens--
		return Result{
			Allowed:   true,
			Limit:     int(tb.burst),
			Remaining: int(b.tokens),
			ResetAt:   now.Add(resetIn),
		}, nil
	}

	return Result{
		Allowed:   false,
		Limit:     int(tb.burst),
		Remaining: 0,
		ResetAt:   now.Add(resetIn),
	}, nil
}
