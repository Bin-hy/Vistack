package interaction

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

// pairKey 用于净效果去重。
type pairKey struct {
	videoID int64
	userID  int64
}

// popEvents 从待处理队列批量弹出事件（LPOP，最多 batch 条）。
func (s *Service) popEvents(ctx context.Context, batch int) ([]Event, error) {
	if batch <= 0 {
		batch = s.opts.FlushBatch
	}
	raws, err := s.rdb.LPopCount(ctx, pendingKey, batch).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	events := make([]Event, 0, len(raws))
	for _, raw := range raws {
		var e Event
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			if s.log() != nil {
				s.log().Warn("skip bad interaction event", zap.String("raw", raw), zap.Error(err))
			}
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

// applyEvents 将事件净效果应用到 DB（幂等，可安全重试）。
func (s *Service) applyEvents(ctx context.Context, events []Event) error {
	likeNet := map[pairKey]bool{} // true=like, false=unlike
	favNet := map[pairKey]bool{}
	plays := make([]mSocial.VideoPlayLog, 0)
	playSeen := map[int64]struct{}{}

	for _, e := range events {
		switch e.Type {
		case EventLike, EventUnlike:
			likeNet[pairKey{e.VideoID, e.UserID}] = e.Type == EventLike
		case EventFavorite, EventUnfavorite:
			favNet[pairKey{e.VideoID, e.UserID}] = e.Type == EventFavorite
		case EventPlay:
			if _, ok := playSeen[e.ID]; !ok {
				playSeen[e.ID] = struct{}{}
				plays = append(plays, mSocial.VideoPlayLog{ID: e.ID, VideoID: e.VideoID, PlayedAt: time.Unix(e.At, 0)})
			}
		}
	}

	for k, liked := range likeNet {
		if liked {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
				Create(&mSocial.VideoLike{VideoID: k.videoID, UserID: k.userID, CreatedAt: time.Now()}).Error; err != nil {
				return err
			}
		} else if err := s.db.WithContext(ctx).
			Where("video_id = ? AND user_id = ?", k.videoID, k.userID).
			Delete(&mSocial.VideoLike{}).Error; err != nil {
			return err
		}
	}

	for k, favorited := range favNet {
		if favorited {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
				Create(&mSocial.VideoFavorite{VideoID: k.videoID, UserID: k.userID, CreatedAt: time.Now()}).Error; err != nil {
				return err
			}
		} else if err := s.db.WithContext(ctx).
			Where("video_id = ? AND user_id = ?", k.videoID, k.userID).
			Delete(&mSocial.VideoFavorite{}).Error; err != nil {
			return err
		}
	}

	if len(plays) > 0 {
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&plays).Error; err != nil {
			return err
		}
	}
	return nil
}

// syncCounts 以 Redis 计数为权威，回写 videos 冗余计数列（Redis 不可用则跳过）。
func (s *Service) syncCounts(ctx context.Context, videoIDs []int64) {
	seen := map[int64]struct{}{}
	for _, id := range videoIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			s.syncOneCount(ctx, id)
		}
	}
}

func (s *Service) syncOneCount(ctx context.Context, videoID int64) {
	like, err1 := s.rdb.SCard(ctx, likeKey(videoID)).Result()
	fav, err2 := s.rdb.SCard(ctx, favKey(videoID)).Result()
	playStr, err3 := s.rdb.Get(ctx, playKey(videoID)).Result()
	if err1 != nil || err2 != nil || (err3 != nil && err3 != redis.Nil) {
		if s.log() != nil {
			s.log().Warn("sync count skipped (redis unavailable)", zap.Int64("video_id", videoID))
		}
		return
	}
	play, _ := strconv.ParseInt(playStr, 10, 64)
	if err := s.db.Model(&mVideo.Video{}).Where("id = ?", videoID).
		Updates(map[string]interface{}{
			"like_count":     like,
			"favorite_count": fav,
			"play_count":     play,
		}).Error; err != nil && s.log() != nil {
		s.log().Error("sync count failed", zap.Int64("video_id", videoID), zap.Error(err))
	}
}

// FlushPending 处理一批待落库事件，返回处理条数。
func (s *Service) FlushPending(ctx context.Context, batch int) (int, error) {
	events, err := s.popEvents(ctx, batch)
	if err != nil {
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}
	if err := s.applyEvents(ctx, events); err != nil {
		return 0, err
	}
	videoIDs := make([]int64, 0, len(events))
	for _, e := range events {
		videoIDs = append(videoIDs, e.VideoID)
	}
	s.syncCounts(ctx, videoIDs)
	return len(events), nil
}

// StartFlusher 启动后台异步落库 goroutine（定时批量 drain 队列）。
func (s *Service) StartFlusher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.opts.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.FlushPending(ctx, s.opts.FlushBatch)
				if err != nil {
					if s.log() != nil {
						s.log().Error("flush interaction events failed", zap.Error(err))
					}
					continue
				}
				if n > 0 && s.log() != nil {
					s.log().Info("flushed interaction events", zap.Int("count", n))
				}
			}
		}
	}()
}
