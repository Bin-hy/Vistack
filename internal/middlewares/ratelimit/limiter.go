package ratelimit

import (
	"context"
	"time"
)

// Result 限流判定结果，用于构造标准限流响应头。
type Result struct {
	Allowed   bool
	Limit     int       // 总配额
	Remaining int       // 剩余可用量
	ResetAt   time.Time // 重置/可再请求时间
}

// Limiter 限流器统一接口。err 表示依赖（如 Redis）不可用，由调用方决定降级策略。
type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
}
