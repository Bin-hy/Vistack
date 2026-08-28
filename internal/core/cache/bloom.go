package cache

import (
	"context"
	"errors"
	"hash/fnv"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Bloom 基于 Redis bitmap 的布隆过滤器，用于拦截「一定不存在」的元素。
type Bloom struct {
	client *redis.Client
	key    string
	bits   uint64
	hashes uint64
}

// NewBloom 构造布隆过滤器。
func NewBloom(client *redis.Client, key string, bits, hashes uint64) *Bloom {
	if bits == 0 {
		bits = 10_000_000
	}
	if hashes == 0 {
		hashes = 7
	}
	return &Bloom{client: client, key: key, bits: bits, hashes: hashes}
}

func (b *Bloom) readyKey() string { return b.key + ":ready" }

// positions 计算 k 个位下标（FNV-1a 双哈希）。
func (b *Bloom) positions(item string) []uint64 {
	h1 := fnv1a64([]byte(item))
	h2 := fnv1a64([]byte(item + "\x00"))
	out := make([]uint64, 0, b.hashes)
	for i := uint64(0); i < b.hashes; i++ {
		out = append(out, (h1+i*h2)%b.bits)
	}
	return out
}

func fnv1a64(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

// Build 清空并用 items 重建布隆，完成后置 ready 标志。
func (b *Bloom) Build(ctx context.Context, items []string) error {
	pipe := b.client.TxPipeline()
	pipe.Del(ctx, b.key)
	pipe.Del(ctx, b.readyKey())
	for _, item := range items {
		for _, pos := range b.positions(item) {
			pipe.SetBit(ctx, b.key, int64(pos), 1)
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return b.client.Set(ctx, b.readyKey(), "1", 0).Err()
}

// Add 新增单个元素。
func (b *Bloom) Add(ctx context.Context, item string) error {
	pipe := b.client.TxPipeline()
	for _, pos := range b.positions(item) {
		pipe.SetBit(ctx, b.key, int64(pos), 1)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		if logger != nil {
			logger.Error("bloom add failed", zap.String("item", item), zap.Error(err))
		}
		return err
	}
	return nil
}

// Exists 判断元素是否可能存在；返回 false 表示一定不存在。
// 未就绪（未构建/构建中）时降级返回 (true, nil)，避免误拦截；Redis 错误返回 (true, err)。
func (b *Bloom) Exists(ctx context.Context, item string) (bool, error) {
	ready, err := b.client.Get(ctx, b.readyKey()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return true, nil // 未构建 → 降级为「可能存在」
		}
		return true, err // Redis 错误 → 降级
	}
	if ready != "1" {
		return true, nil // 构建中/被清除 → 降级
	}
	pipe := b.client.TxPipeline()
	cmds := make([]*redis.IntCmd, 0, b.hashes)
	for _, pos := range b.positions(item) {
		cmds = append(cmds, pipe.GetBit(ctx, b.key, int64(pos)))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err
	}
	for _, cmd := range cmds {
		if cmd.Val() == 0 {
			return false, nil
		}
	}
	return true, nil
}
