package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestTokenBucketBurst(t *testing.T) {
	tb := NewTokenBucket(1, 2) // 每秒补 1，容量 2
	ctx := context.Background()

	if r, _ := tb.Allow(ctx, "u1"); !r.Allowed {
		t.Fatal("1st should be allowed")
	}
	if r, _ := tb.Allow(ctx, "u1"); !r.Allowed {
		t.Fatal("2nd should be allowed (burst)")
	}
	if r, _ := tb.Allow(ctx, "u1"); r.Allowed {
		t.Fatal("3rd should be rejected (bucket empty)")
	}

	time.Sleep(1100 * time.Millisecond) // 补充 1 个令牌

	if r, _ := tb.Allow(ctx, "u1"); !r.Allowed {
		t.Fatal("after refill should be allowed")
	}
}

func TestTokenBucketPerKey(t *testing.T) {
	tb := NewTokenBucket(1, 1)
	ctx := context.Background()

	if r, _ := tb.Allow(ctx, "u1"); !r.Allowed {
		t.Fatal("u1 1st should be allowed")
	}
	if r, _ := tb.Allow(ctx, "u1"); r.Allowed {
		t.Fatal("u1 2nd should be rejected")
	}
	if r, _ := tb.Allow(ctx, "u2"); !r.Allowed {
		t.Fatal("u2 1st should be allowed (independent bucket)")
	}
}

func newTestSlidingWindow(t *testing.T, window time.Duration, limit int) *SlidingWindow {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewSlidingWindow(client, window, limit)
}

func TestSlidingWindowLimit(t *testing.T) {
	sw := newTestSlidingWindow(t, time.Second, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if r, err := sw.Allow(ctx, "u1"); err != nil || !r.Allowed {
			t.Fatalf("request %d should be allowed, err=%v", i+1, err)
		}
	}
	if r, err := sw.Allow(ctx, "u1"); err != nil {
		t.Fatalf("4th err=%v", err)
	} else if r.Allowed {
		t.Fatal("4th should be rejected (limit=3)")
	}
}

func TestSlidingWindowSlide(t *testing.T) {
	sw := newTestSlidingWindow(t, 200*time.Millisecond, 1)
	ctx := context.Background()

	if r, _ := sw.Allow(ctx, "u1"); !r.Allowed {
		t.Fatal("1st should be allowed")
	}
	if r, _ := sw.Allow(ctx, "u1"); r.Allowed {
		t.Fatal("2nd should be rejected")
	}

	time.Sleep(250 * time.Millisecond) // 窗口滑过

	if r, err := sw.Allow(ctx, "u1"); err != nil || !r.Allowed {
		t.Fatalf("after window slide should be allowed, err=%v", err)
	}
}

func TestSlidingWindowRedisDown(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1", // 死端口模拟 Redis 不可用
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  200 * time.Millisecond,
		WriteTimeout: 200 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	sw := NewSlidingWindow(client, time.Second, 3)
	if _, err := sw.Allow(context.Background(), "u1"); err == nil {
		t.Fatal("want error when redis down")
	}
}
