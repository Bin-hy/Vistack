package danmaku

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	entity "github.com/binhy/vistack/internal/model/entity/danmaku"
	"github.com/redis/go-redis/v9"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewService(client, nil, Options{})
}

func TestACAutomaton(t *testing.T) {
	f := NewSensitiveFilter([]string{"abc", "bc", "傻逼"})
	if !f.Contains("xxabcxx") {
		t.Fatal("should match abc")
	}
	if !f.Contains("xxbcxx") {
		t.Fatal("should match bc")
	}
	if !f.Contains("这是傻逼") {
		t.Fatal("should match 傻逼")
	}
	if f.Contains("abx") {
		t.Fatal("should not match abx")
	}

	f.Reload([]string{"广告"})
	if !f.Contains("这是广告") {
		t.Fatal("new word should take effect after reload")
	}
	if f.Contains("这是傻逼") {
		t.Fatal("old word should be removed after reload")
	}
}

func TestLocalCache(t *testing.T) {
	c := NewLocalCache(2, 100*time.Millisecond)
	c.Set("a", []entity.Danmaku{{Content: "a"}})
	if items, ok := c.Get("a"); !ok || len(items) != 1 || items[0].Content != "a" {
		t.Fatal("want hit a")
	}
	c.Set("b", []entity.Danmaku{{Content: "b"}})
	c.Set("c", []entity.Danmaku{{Content: "c"}}) // 淘汰 a
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	time.Sleep(150 * time.Millisecond)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be expired")
	}
}

func TestSendSensitive(t *testing.T) {
	s := newTestService(t)
	s.filter.Reload([]string{"傻逼", "广告"})
	ctx := context.Background()

	if _, err := s.Send(ctx, 1, 100, "这是一条正常弹幕", 1.5, "", 0); err != nil {
		t.Fatalf("normal danmaku should pass, got %v", err)
	}
	if _, err := s.Send(ctx, 1, 100, "这是傻逼弹幕", 2.0, "", 0); !errors.Is(err, ErrSensitive) {
		t.Fatalf("sensitive danmaku should be rejected, got %v", err)
	}
}

func TestFetchRange(t *testing.T) {
	s := newTestService(t)
	s.filter.Reload(nil)
	ctx := context.Background()

	_, _ = s.Send(ctx, 1, 100, "t0", 0.5, "", 0)
	_, _ = s.Send(ctx, 1, 100, "t1", 1.5, "", 0)
	_, _ = s.Send(ctx, 1, 100, "t2", 2.5, "", 0)
	_, _ = s.Send(ctx, 1, 100, "t3", 3.5, "", 0)

	items, err := s.Fetch(ctx, 1, 1, 3) // [1, 3]
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items in [1,3], got %d", len(items))
	}
	if items[0].Content != "t1" || items[1].Content != "t2" {
		t.Fatalf("want [t1 t2] ascending, got %q %q", items[0].Content, items[1].Content)
	}
}
