package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := New(client, Options{
		DefaultTTL: [2]time.Duration{time.Minute, 2 * time.Minute},
		NullTTL:    time.Second,
		LockTTL:    time.Second,
		LockWait:   time.Second,
	})
	t.Cleanup(func() { _ = client.Close() })
	return c, mr
}

func TestBloomPositions(t *testing.T) {
	b := NewBloom(nil, "k", 1000, 5)
	for _, item := range []string{"a", "b", "hello", "vistack:video:info:123"} {
		pos := b.positions(item)
		if len(pos) != 5 {
			t.Fatalf("want 5 positions, got %d", len(pos))
		}
		for _, p := range pos {
			if p >= 1000 {
				t.Fatalf("position %d out of range", p)
			}
		}
	}
}

func TestBloomBuildAndExists(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBloom(client, "bloomkey", 10000, 5)
	ctx := context.Background()

	// 未就绪：Exists 降级返回 true（不误拦截）
	exists, err := b.Exists(ctx, "x")
	if err != nil {
		t.Fatalf("Exists before build should not error, got %v", err)
	}
	if !exists {
		t.Fatal("want true (degrade) before build")
	}

	if err := b.Build(ctx, []string{"a", "b", "c"}); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if exists, _ := b.Exists(ctx, "a"); !exists {
		t.Fatal("want true for added item")
	}
	if exists, _ := b.Exists(ctx, "definitely-not-present-zzz"); exists {
		t.Fatal("want false for non-added item")
	}
}

func TestCacheNullValue(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	var calls int32
	loader := func(ctx context.Context) (any, bool, error) {
		atomic.AddInt32(&calls, 1)
		return nil, false, nil
	}
	var dst map[string]any
	for i := 0; i < 3; i++ {
		found, err := c.GetOrLoad(ctx, "nullkey", &dst, loader)
		if err != nil {
			t.Fatal(err)
		}
		if found {
			t.Fatal("want not found")
		}
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("want loader called once (null value cached), got %d", calls)
	}
}

func TestCacheSingleflight(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	var calls int32
	loader := func(ctx context.Context) (any, bool, error) {
		atomic.AddInt32(&calls, 1)
		return map[string]string{"id": "1"}, true, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var dst map[string]string
			if _, err := c.GetOrLoad(ctx, "sfkey", &dst, loader); err != nil {
				t.Errorf("GetOrLoad: %v", err)
			}
		}()
	}
	wg.Wait()
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("want loader called once under singleflight, got %d", calls)
	}
}

func TestCacheDelete(t *testing.T) {
	c, _ := newTestCache(t)
	ctx := context.Background()
	var calls int32
	loader := func(ctx context.Context) (any, bool, error) {
		atomic.AddInt32(&calls, 1)
		return "v1", true, nil
	}
	var dst string
	if _, err := c.GetOrLoad(ctx, "delkey", &dst, loader); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, "delkey"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetOrLoad(ctx, "delkey", &dst, loader); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("want loader called twice after delete, got %d", calls)
	}
}

func TestWithTTL(t *testing.T) {
	c, mr := newTestCache(t)
	ctx := context.Background()
	loader := func(ctx context.Context) (any, bool, error) {
		return "v", true, nil
	}
	var dst string
	if _, err := c.GetOrLoad(ctx, "ttlkey", &dst, loader, WithTTL(10*time.Second, 10*time.Second)); err != nil {
		t.Fatal(err)
	}
	ttl := mr.TTL("ttlkey")
	if ttl <= 0 || ttl > 10*time.Second {
		t.Fatalf("want ttl ~10s, got %v", ttl)
	}
}

// TestCacheDegradeOnRedisDown 模拟 Redis 不可用时降级直查 DB（不回源失败）。
func TestCacheDegradeOnRedisDown(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1", // 端口关闭，模拟 Redis 不可用
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
		MaxRetries:   -1,
	})
	t.Cleanup(func() { _ = client.Close() })

	c := New(client, Options{
		DefaultTTL: [2]time.Duration{time.Minute, 2 * time.Minute},
		NullTTL:    time.Second,
		LockTTL:    time.Second,
		LockWait:   time.Second,
	})

	ctx := context.Background()
	var dst map[string]string
	found, err := c.GetOrLoad(ctx, "anykey", &dst, func(ctx context.Context) (any, bool, error) {
		return map[string]string{"ok": "from-db"}, true, nil
	})
	if err != nil {
		t.Fatalf("want degrade without error, got %v", err)
	}
	if !found || dst["ok"] != "from-db" {
		t.Fatalf("want degraded result from loader, got found=%v dst=%v", found, dst)
	}
}
