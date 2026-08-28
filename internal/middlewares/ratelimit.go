package middlewares

import (
	"net/http"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/middlewares/ratelimit"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var rateLogger *zap.Logger

// SetLogger 注入结构化日志器（可选，nil 时不打印）。
func SetLogger(l *zap.Logger) {
	rateLogger = l
}

// BuildLimiter 从配置构造限流器。rdb 供滑动窗口使用；未启用返回 nil。
func BuildLimiter(cfg *config.AppConfig, rdb *redis.Client) ratelimit.Limiter {
	rl := cfg.RateLimit
	if !rl.Enabled {
		return nil
	}
	switch rl.Algorithm {
	case "token_bucket":
		rate, burst := rl.TokenRate, rl.TokenBurst
		if rate <= 0 {
			rate = 10
		}
		if burst <= 0 {
			burst = 20
		}
		return ratelimit.NewTokenBucket(rate, burst)
	default: // sliding_window 或空
		window, limit := rl.Window, rl.Limit
		if window <= 0 {
			window = 60
		}
		if limit <= 0 {
			limit = 100
		}
		return ratelimit.NewSlidingWindow(rdb, time.Duration(window)*time.Second, limit)
	}
}

// RateLimit 限流中间件：按用户 ID 限流。limiter 为 nil 时直通；依赖不可用时 fail-open 放行。
func RateLimit(limiter ratelimit.Limiter) gin.HandlerFunc {
	if limiter == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		userID := auth.GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}

		key := strconv.FormatInt(userID, 10)
		res, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			if rateLogger != nil {
				rateLogger.Error("rate limiter unavailable, fail-open", zap.Error(err))
			}
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(res.Limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(res.ResetAt.Unix(), 10))

		if !res.Allowed {
			retryAfter := int(time.Until(res.ResetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			if rateLogger != nil {
				rateLogger.Warn("rate limited", zap.Int64("user_id", userID), zap.Int("remaining", res.Remaining))
			}
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
