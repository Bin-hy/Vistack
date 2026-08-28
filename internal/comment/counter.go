package comment

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

const pendingKey = "vistack:comment:like:pending"

// likeEvent 待落库点赞事件。
type likeEvent struct {
	CommentID int64 `json:"comment_id"`
	UserID    int64 `json:"user_id"`
	Liked     bool  `json:"liked"`
}

var toggleLikeScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
	redis.call('SREM', KEYS[1], ARGV[1])
	redis.call('RPUSH', KEYS[2], ARGV[3])
	return {0, redis.call('SCARD', KEYS[1])}
else
	redis.call('SADD', KEYS[1], ARGV[1])
	redis.call('RPUSH', KEYS[2], ARGV[2])
	return {1, redis.call('SCARD', KEYS[1])}
end`)

// ToggleLike 点赞/取消点赞，返回 (liked, like_count, error)。
func (s *Service) ToggleLike(ctx context.Context, commentID, userID int64) (bool, int64, error) {
	var c mSocial.VideoComment
	if err := s.db.WithContext(ctx).Select("id", "status").First(&c, commentID).Error; err != nil {
		return false, 0, ErrNotFound
	}
	if c.Status != mSocial.CommentStatusVisible {
		return false, 0, ErrNotFound
	}

	on, _ := json.Marshal(likeEvent{CommentID: commentID, UserID: userID, Liked: true})
	off, _ := json.Marshal(likeEvent{CommentID: commentID, UserID: userID, Liked: false})

	vals, err := toggleLikeScript.Run(ctx, s.rdb,
		[]string{likeKey(commentID), pendingKey},
		strconv.FormatInt(userID, 10),
		string(on),
		string(off),
	).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(vals) != 2 {
		return false, 0, errors.New("unexpected toggle result")
	}
	return vals[0] == 1, vals[1], nil
}

// IsLiked 判断用户是否已赞某评论。
func (s *Service) IsLiked(ctx context.Context, commentID, userID int64) (bool, error) {
	return s.rdb.SIsMember(ctx, likeKey(commentID), userID).Result()
}

// LikeCount 读点赞数（Redis 优先，DB 回退）。
func (s *Service) LikeCount(ctx context.Context, commentID int64) int64 {
	if n, err := s.rdb.SCard(ctx, likeKey(commentID)).Result(); err == nil {
		return n
	}
	var c mSocial.VideoComment
	if err := s.db.WithContext(ctx).Select("like_count").First(&c, commentID).Error; err == nil {
		return c.LikeCount
	}
	return 0
}

// StartFlusher 后台异步把点赞事件落库（comment_likes + like_count）。
func (s *Service) StartFlusher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.opts.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := s.flushLikes(ctx, s.opts.FlushBatch)
				if err != nil && s.log() != nil {
					s.log().Error("flush comment likes failed", zap.Error(err))
					continue
				}
				if n > 0 && s.log() != nil {
					s.log().Info("flushed comment likes", zap.Int("count", n))
				}
			}
		}
	}()
}

func (s *Service) flushLikes(ctx context.Context, batch int) (int, error) {
	raws, err := s.rdb.LPopCount(ctx, pendingKey, batch).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	// 净效果：同 (comment,user) 取最后一次
	net := map[[2]int64]bool{}
	for _, raw := range raws {
		var e likeEvent
		if json.Unmarshal([]byte(raw), &e) != nil {
			continue
		}
		net[[2]int64{e.CommentID, e.UserID}] = e.Liked
	}

	for k, liked := range net {
		if liked {
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).
				Create(&mSocial.CommentLike{CommentID: k[0], UserID: k[1], CreatedAt: time.Now()}).Error; err != nil {
				return 0, err
			}
		} else if err := s.db.WithContext(ctx).
			Where("comment_id = ? AND user_id = ?", k[0], k[1]).
			Delete(&mSocial.CommentLike{}).Error; err != nil {
			return 0, err
		}
	}

	// 以 Redis 计数为权威回写 like_count
	ids := map[int64]struct{}{}
	for k := range net {
		ids[k[0]] = struct{}{}
	}
	for id := range ids {
		n, err := s.rdb.SCard(ctx, likeKey(id)).Result()
		if err != nil {
			continue
		}
		if err := s.db.WithContext(ctx).Model(&mSocial.VideoComment{}).Where("id = ?", id).
			Update("like_count", n).Error; err != nil && s.log() != nil {
			s.log().Error("sync comment like_count failed", zap.Int64("comment_id", id), zap.Error(err))
		}
	}
	return len(raws), nil
}
