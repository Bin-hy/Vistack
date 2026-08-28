package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/binhy/vistack/pkg/timeutil"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// nullSentinel 空值缓存哨兵，用于穿透防护。
const nullSentinel = "\x00NULL\x00"

// logger 由 SetLogger 注入（可选），nil 时不打印。
var logger *zap.Logger

// SetLogger 注入结构化日志器。
func SetLogger(l *zap.Logger) {
	logger = l
}

// releaseLockScript 校验持有者后删除锁，避免误删他人锁。
var releaseLockScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

// Options 缓存组件构造参数。
type Options struct {
	DefaultTTL [2]time.Duration // 随机 TTL 范围 [min, max]
	NullTTL    time.Duration    // 空值缓存 TTL
	LockTTL    time.Duration    // 互斥锁 TTL
	LockWait   time.Duration    // 未抢到锁时的最大等待
	Bloom      *BloomOptions    // 非 nil 表示启用布隆过滤
}

// BloomOptions 布隆过滤器参数。
type BloomOptions struct {
	Key    string
	Bits   uint64
	Hashes uint64
}

// Loader 回源函数。value 为要缓存的对象；found=false 表示源中不存在（触发空值缓存）。
type Loader func(ctx context.Context) (value any, found bool, err error)

type loadResult struct {
	found bool
	raw   string
}

type callOpts struct {
	ttlMin   time.Duration
	ttlMax   time.Duration
	useBloom bool
}

// Option 每调用覆盖项。
type Option func(*callOpts)

// WithTTL 覆盖本次调用的随机 TTL 范围。
func WithTTL(min, max time.Duration) Option {
	return func(o *callOpts) {
		o.ttlMin = min
		o.ttlMax = max
	}
}

// WithBloom 本次调用启用布隆过滤器（仅对按 ID 查询的 key 使用）。
func WithBloom() Option {
	return func(o *callOpts) {
		o.useBloom = true
	}
}

// Cache 通用 Cache-Aside 缓存组件。
type Cache struct {
	client *redis.Client
	sf     singleflight.Group
	opts   Options
	bloom  *Bloom
}

// New 构造 Cache。opts.Bloom 非 nil 时内部创建布隆过滤器实例。
func New(client *redis.Client, opts Options) *Cache {
	c := &Cache{client: client, opts: opts}
	if opts.Bloom != nil {
		c.bloom = NewBloom(client, opts.Bloom.Key, opts.Bloom.Bits, opts.Bloom.Hashes)
	}
	return c
}

// GetOrLoad 读缓存；未命中则回源并写缓存。dst 为反序列化目标。
func (c *Cache) GetOrLoad(ctx context.Context, key string, dst any, loader Loader, opts ...Option) (bool, error) {
	co := callOpts{ttlMin: c.opts.DefaultTTL[0], ttlMax: c.opts.DefaultTTL[1]}
	for _, o := range opts {
		o(&co)
	}

	if found, raw, hit := c.readCache(ctx, key); hit {
		return c.decode(raw, found, dst)
	}

	if co.useBloom && c.bloom != nil {
		if exists, err := c.bloom.Exists(ctx, key); err == nil && !exists {
			return false, nil
		}
	}

	v, err, _ := c.sf.Do(key, func() (any, error) {
		return c.loadAndCache(ctx, key, loader, co)
	})
	if err != nil {
		return false, err
	}
	res := v.(loadResult)
	return c.decode(res.raw, res.found, dst)
}

// Delete 删除缓存 key（写路径失效）。
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.client.Del(ctx, keys...).Err()
}

func (c *Cache) readCache(ctx context.Context, key string) (found bool, raw string, hit bool) {
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return false, "", false // 未命中或 Redis 错误 → 走回源
	}
	if val == nullSentinel {
		return false, "", true
	}
	return true, val, true
}

func (c *Cache) decode(raw string, found bool, dst any) (bool, error) {
	if !found {
		return false, nil
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return false, err
	}
	return true, nil
}

func (c *Cache) loadAndCache(ctx context.Context, key string, loader Loader, co callOpts) (loadResult, error) {
	// double-check：可能已被其他实例/协程填上
	if found, raw, hit := c.readCache(ctx, key); hit {
		return loadResult{found: found, raw: raw}, nil
	}

	token, acquired, err := c.acquireLock(ctx, key)
	if err != nil {
		if logger != nil {
			logger.Error("acquire cache lock failed, degrade to direct load", zap.String("key", key), zap.Error(err))
		}
		return c.directLoad(ctx, loader)
	}
	if acquired {
		defer c.releaseLock(ctx, key, token)
		if found, raw, hit := c.readCache(ctx, key); hit {
			return loadResult{found: found, raw: raw}, nil
		}
		return c.loadAndWrite(ctx, key, loader, co)
	}
	return c.waitAndRead(ctx, key, loader, c.opts.LockWait)
}

func (c *Cache) loadAndWrite(ctx context.Context, key string, loader Loader, co callOpts) (loadResult, error) {
	value, found, err := loader(ctx)
	if err != nil {
		return loadResult{}, err
	}
	if !found {
		if err := c.client.Set(ctx, key, nullSentinel, c.opts.NullTTL).Err(); err != nil && logger != nil {
			logger.Error("cache set null failed", zap.String("key", key), zap.Error(err))
		}
		return loadResult{found: false}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return loadResult{}, err
	}
	ttl := c.randomTTL(co.ttlMin, co.ttlMax)
	if err := c.client.Set(ctx, key, string(raw), ttl).Err(); err != nil && logger != nil {
		logger.Error("cache set failed", zap.String("key", key), zap.Error(err))
	}
	return loadResult{found: true, raw: string(raw)}, nil
}

func (c *Cache) directLoad(ctx context.Context, loader Loader) (loadResult, error) {
	value, found, err := loader(ctx)
	if err != nil {
		return loadResult{}, err
	}
	if !found {
		return loadResult{found: false}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return loadResult{}, err
	}
	return loadResult{found: true, raw: string(raw)}, nil
}

func (c *Cache) waitAndRead(ctx context.Context, key string, loader Loader, wait time.Duration) (loadResult, error) {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if found, raw, hit := c.readCache(ctx, key); hit {
			return loadResult{found: found, raw: raw}, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return c.directLoad(ctx, loader)
}

func (c *Cache) acquireLock(ctx context.Context, key string) (token string, acquired bool, err error) {
	lockKey := key + ":lock"
	token = uuid.NewString()
	ok, err := c.client.SetNX(ctx, lockKey, token, c.opts.LockTTL).Result()
	if err != nil {
		return "", false, err
	}
	return token, ok, nil
}

func (c *Cache) releaseLock(ctx context.Context, key, token string) {
	lockKey := key + ":lock"
	if err := releaseLockScript.Run(ctx, c.client, []string{lockKey}, token).Err(); err != nil && logger != nil {
		logger.Error("release cache lock failed", zap.String("key", lockKey), zap.Error(err))
	}
}

func (c *Cache) randomTTL(min, max time.Duration) time.Duration {
	if min >= max {
		return min
	}
	return timeutil.RandomRangeExpire(min, max)
}
