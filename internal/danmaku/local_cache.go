package danmaku

import (
	"container/list"
	"sync"
	"time"

	entity "github.com/binhy/vistack/internal/model/entity/danmaku"
)

type lruEntry struct {
	key     string
	value   []entity.Danmaku
	expires time.Time
}

// LocalCache 进程内 LRU 缓存（按分段 key 缓存弹幕，带 TTL）。
type LocalCache struct {
	mu    sync.Mutex
	size  int
	ttl   time.Duration
	items map[string]*list.Element
	order *list.List
}

func NewLocalCache(size int, ttl time.Duration) *LocalCache {
	if size <= 0 {
		size = 1024
	}
	if ttl <= 0 {
		ttl = time.Second
	}
	return &LocalCache{
		size:  size,
		ttl:   ttl,
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Get 命中返回 value；过期删除返回 miss。
func (c *LocalCache) Get(key string) ([]entity.Danmaku, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*lruEntry)
	if time.Now().After(e.expires) {
		c.order.Remove(el)
		delete(c.items, key)
		return nil, false
	}
	c.order.MoveToFront(el)
	return e.value, true
}

// Set 写入/更新，超容量时淘汰最久未用。
func (c *LocalCache) Set(key string, v []entity.Danmaku) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		e := el.Value.(*lruEntry)
		e.value = v
		e.expires = time.Now().Add(c.ttl)
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&lruEntry{key: key, value: v, expires: time.Now().Add(c.ttl)})
	c.items[key] = el
	for c.order.Len() > c.size {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.items, back.Value.(*lruEntry).key)
	}
}
