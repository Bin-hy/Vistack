package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/core/cache"
	"github.com/binhy/vistack/internal/global"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"go.uber.org/zap"
)

const cacheKeyVideoRecommend = "vistack:video:recommend"

// videoInfoCacheKey 视频详情缓存 key。
func videoInfoCacheKey(id int64) string {
	return fmt.Sprintf("vistack:video:info:%d", id)
}

// recommendCacheTTL 推荐列表缓存 TTL（含默认值兜底，避免配置缺失导致永久缓存）。
func recommendCacheTTL() time.Duration {
	ttl := global.AppConfig.Cache.RecommendTTL
	if ttl <= 0 {
		ttl = 300
	}
	return time.Duration(ttl) * time.Second
}

// getOrLoad 包装 core.Cache.GetOrLoad；core.Cache 为 nil（缓存关闭）时直接回源兜底。
func getOrLoad(ctx context.Context, key string, dst any, loader cache.Loader, opts ...cache.Option) (bool, error) {
	if core.Cache == nil {
		value, found, err := loader(ctx)
		if err != nil || !found {
			return found, err
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return false, err
		}
		return true, json.Unmarshal(raw, dst)
	}
	return core.Cache.GetOrLoad(ctx, key, dst, loader, opts...)
}

// deleteCache 失效缓存；core.Cache 为 nil 时跳过。
func deleteCache(ctx context.Context, keys ...string) {
	if core.Cache != nil {
		_ = core.Cache.Delete(ctx, keys...)
	}
}

// addVideoBloom 新增视频 ID 到布隆过滤器（best-effort，失败仅记日志）。
func addVideoBloom(ctx context.Context, videoID int64) {
	if core.VideoBloom == nil {
		return
	}
	if err := core.VideoBloom.Add(ctx, videoInfoCacheKey(videoID)); err != nil {
		core.Logger.Error("add video bloom failed", zap.Int64("video_id", videoID), zap.Error(err))
	}
}

// BuildVideoBloom 从 videos 表全量构建布隆过滤器（元素为视频详情缓存 key）。
func BuildVideoBloom(ctx context.Context) {
	if core.VideoBloom == nil {
		return
	}
	var ids []int64
	if err := core.DB.Model(&mVideo.Video{}).Pluck("id", &ids).Error; err != nil {
		core.Logger.Error("load video ids for bloom failed", zap.Error(err))
		return
	}
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		items = append(items, videoInfoCacheKey(id))
	}
	if err := core.VideoBloom.Build(ctx, items); err != nil {
		core.Logger.Error("build video bloom failed", zap.Error(err))
		return
	}
	core.Logger.Info("video bloom built", zap.Int("count", len(items)))
}
