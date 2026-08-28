package danmaku

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	entity "github.com/binhy/vistack/internal/model/entity/danmaku"
	"github.com/binhy/vistack/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrSensitive 命中敏感词。
var ErrSensitive = errors.New("sensitive word")

// Options 构造参数。
type Options struct {
	LocalCacheSize     int
	LocalCacheTTL      time.Duration
	CacheControlMaxAge int
	Logger             *zap.Logger
}

// Service 点播弹幕服务。
type Service struct {
	rdb    *redis.Client
	db     *gorm.DB
	filter *SensitiveFilter
	local  *LocalCache
	opts   Options
}

func NewService(rdb *redis.Client, db *gorm.DB, opts Options) *Service {
	if opts.LocalCacheSize <= 0 {
		opts.LocalCacheSize = 1024
	}
	if opts.LocalCacheTTL <= 0 {
		opts.LocalCacheTTL = 2 * time.Second
	}
	if opts.CacheControlMaxAge <= 0 {
		opts.CacheControlMaxAge = 5
	}
	return &Service{
		rdb:    rdb,
		db:     db,
		filter: NewSensitiveFilter(nil),
		local:  NewLocalCache(opts.LocalCacheSize, opts.LocalCacheTTL),
		opts:   opts,
	}
}

func (s *Service) log() *zap.Logger { return s.opts.Logger }

// Send 发送弹幕：敏感词过滤 → 写 Redis ZSet（实时）→ 投 Kafka（异步落库）。
func (s *Service) Send(ctx context.Context, videoID, userID int64, content string, timeOffset float64, color string, mode int) (*entity.Danmaku, error) {
	if s.filter.Contains(content) {
		return nil, ErrSensitive
	}

	d := &entity.Danmaku{
		ID:         snowflake.GenID(),
		VideoID:    videoID,
		UserID:     userID,
		Content:    content,
		TimeOffset: timeOffset,
		Color:      color,
		Mode:       mode,
		CreatedAt:  time.Now(),
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	// 实时写 Redis ZSet（member 为弹幕 JSON，score 为时间轴位置）
	if err := s.rdb.ZAdd(ctx, danmakuKey(videoID), redis.Z{Score: timeOffset, Member: string(raw)}).Err(); err != nil {
		if s.log() != nil {
			s.log().Error("danmaku zadd failed", zap.Int64("video_id", videoID), zap.Error(err))
		}
	}

	// 异步落库 Kafka（失败不阻断，弹幕已实时可见）
	if err := core.SendKafkaMessage(ctx, string(consts.KafkaTopicDanmaku), strconv.FormatInt(videoID, 10), raw); err != nil {
		if s.log() != nil {
			s.log().Warn("danmaku kafka send failed", zap.Int64("video_id", videoID), zap.Error(err))
		}
	}
	return d, nil
}

// Fetch 按时间范围拉取弹幕（按 time_offset 升序）：本地 LRU → Redis ZSet → DB。
func (s *Service) Fetch(ctx context.Context, videoID int64, start, end float64) ([]entity.Danmaku, error) {
	lkey := localKey(videoID, start, end)

	// 1. 本地缓存
	if items, ok := s.local.Get(lkey); ok {
		return items, nil
	}

	// 2. Redis ZSet
	raws, err := s.rdb.ZRangeByScore(ctx, danmakuKey(videoID), &redis.ZRangeBy{
		Min: strconv.FormatFloat(start, 'f', -1, 64),
		Max: strconv.FormatFloat(end, 'f', -1, 64),
	}).Result()
	if err == nil && len(raws) > 0 {
		items := parseDanmaku(raws)
		s.local.Set(lkey, items)
		return items, nil
	}

	// 3. DB 回源
	list := make([]entity.Danmaku, 0)
	if err := s.db.WithContext(ctx).
		Where("video_id = ? AND time_offset >= ? AND time_offset <= ?", videoID, start, end).
		Order("time_offset asc").
		Find(&list).Error; err != nil {
		return nil, err
	}
	s.local.Set(lkey, list)
	return list, nil
}

// LoadSensitiveWords 从 DB 加载全部敏感词并重建自动机。
func (s *Service) LoadSensitiveWords(ctx context.Context) error {
	var words []entity.SensitiveWord
	if err := s.db.WithContext(ctx).Find(&words).Error; err != nil {
		return err
	}
	list := make([]string, 0, len(words))
	for _, w := range words {
		list = append(list, w.Word)
	}
	s.filter.Reload(list)
	if s.log() != nil {
		s.log().Info("sensitive words loaded", zap.Int("count", len(list)))
	}
	return nil
}

// ListSensitiveWords 列出全部敏感词。
func (s *Service) ListSensitiveWords(ctx context.Context) ([]entity.SensitiveWord, error) {
	var words []entity.SensitiveWord
	if err := s.db.WithContext(ctx).Order("created_at asc").Find(&words).Error; err != nil {
		return nil, err
	}
	return words, nil
}

// AddSensitiveWord 新增敏感词并重建自动机。
func (s *Service) AddSensitiveWord(ctx context.Context, word string) error {
	if word == "" {
		return errors.New("empty word")
	}
	if err := s.db.WithContext(ctx).Create(&entity.SensitiveWord{Word: word}).Error; err != nil {
		return err
	}
	return s.LoadSensitiveWords(ctx)
}

// DeleteSensitiveWord 删除敏感词并重建自动机。
func (s *Service) DeleteSensitiveWord(ctx context.Context, id int64) error {
	if err := s.db.WithContext(ctx).Delete(&entity.SensitiveWord{}, id).Error; err != nil {
		return err
	}
	return s.LoadSensitiveWords(ctx)
}

func localKey(videoID int64, start, end float64) string {
	return strconv.FormatInt(videoID, 10) + ":" + strconv.FormatFloat(start, 'f', -1, 64) + ":" + strconv.FormatFloat(end, 'f', -1, 64)
}

func parseDanmaku(raws []string) []entity.Danmaku {
	list := make([]entity.Danmaku, 0, len(raws))
	for _, raw := range raws {
		var d entity.Danmaku
		if json.Unmarshal([]byte(raw), &d) == nil {
			list = append(list, d)
		}
	}
	return list
}
