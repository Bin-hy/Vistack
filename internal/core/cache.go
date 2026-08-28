package core

import (
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core/cache"
)

// Cache 全局通用缓存组件（api 角色读路径）。
var Cache *cache.Cache

// VideoBloom 全局视频布隆过滤器（构建/新增用，元素为视频详情缓存 key）。
var VideoBloom *cache.Bloom

// InitCache 从配置构造缓存组件与布隆过滤器；对零值配置套默认值。
func InitCache(cfg *config.AppConfig) {
	if !cfg.Cache.Enabled {
		return
	}

	c := &cfg.Cache
	if c.DefaultTTLMin <= 0 {
		c.DefaultTTLMin = 300
	}
	if c.DefaultTTLMax <= 0 {
		c.DefaultTTLMax = 600
	}
	if c.NullTTL <= 0 {
		c.NullTTL = 60
	}
	if c.LockTTL <= 0 {
		c.LockTTL = 5
	}
	if c.LockWaitMS <= 0 {
		c.LockWaitMS = 2000
	}
	if c.RecommendTTL <= 0 {
		c.RecommendTTL = 300
	}
	if c.BloomBits <= 0 {
		c.BloomBits = 10_000_000
	}
	if c.BloomHashes <= 0 {
		c.BloomHashes = 7
	}

	cache.SetLogger(Logger)

	opts := cache.Options{
		DefaultTTL: [2]time.Duration{
			time.Duration(c.DefaultTTLMin) * time.Second,
			time.Duration(c.DefaultTTLMax) * time.Second,
		},
		NullTTL:  time.Duration(c.NullTTL) * time.Second,
		LockTTL:  time.Duration(c.LockTTL) * time.Second,
		LockWait: time.Duration(c.LockWaitMS) * time.Millisecond,
	}

	if c.BloomEnabled {
		opts.Bloom = &cache.BloomOptions{
			Key:    "vistack:video:bloom",
			Bits:   uint64(c.BloomBits),
			Hashes: uint64(c.BloomHashes),
		}
		VideoBloom = cache.NewBloom(Redis, "vistack:video:bloom", uint64(c.BloomBits), uint64(c.BloomHashes))
	}

	Cache = cache.New(Redis, opts)
}
