package timeutil

import (
	"math/rand"
	"time"
)

// RandomRangeExpire 随机一个范围的过期时间
func RandomRangeExpire(minExpire, maxExpire time.Duration) time.Duration {
	if minExpire >= maxExpire {
		return minExpire
	}
	// 使用 Int63n 处理 int64 类型的 Duration，避免溢出
	randVal := rand.Int63n(int64(maxExpire - minExpire))
	return time.Duration(randVal) + minExpire
}
