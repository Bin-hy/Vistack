package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type EventType string

const (
	EventLike       EventType = "like"
	EventUnlike     EventType = "unlike"
	EventFavorite   EventType = "favorite"
	EventUnfavorite EventType = "unfavorite"
	EventPlay       EventType = "play"
)

// Event 待落库交互事件（Redis List 元素）。
type Event struct {
	ID      int64     `json:"id"` // snowflake，幂等去重
	Type    EventType `json:"type"`
	VideoID int64     `json:"video_id"`
	UserID  int64     `json:"user_id"`
	At      int64     `json:"at"`
}

// Counts 视频三计数。
type Counts struct {
	LikeCount     int64
	FavoriteCount int64
	PlayCount     int64
}

// Options 构造参数。
type Options struct {
	FlushInterval   time.Duration
	FlushBatch      int
	LeaderboardSize int
	Logger          *zap.Logger
}

// Service 点赞/收藏/播放量计数 + 榜单服务。
type Service struct {
	rdb  *redis.Client
	db   *gorm.DB
	opts Options
}

func NewService(rdb *redis.Client, db *gorm.DB, opts Options) *Service {
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 5 * time.Second
	}
	if opts.FlushBatch <= 0 {
		opts.FlushBatch = 200
	}
	if opts.LeaderboardSize <= 0 {
		opts.LeaderboardSize = 50
	}
	return &Service{rdb: rdb, db: db, opts: opts}
}

func (s *Service) log() *zap.Logger { return s.opts.Logger }

// toggleLikeScript 原子：SISMEMBER→SADD/SREM + 更新点赞榜 + 推事件。返回 {liked, count}。
var toggleLikeScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
	redis.call('SREM', KEYS[1], ARGV[1])
	redis.call('ZINCRBY', KEYS[2], -1, ARGV[2])
	redis.call('RPUSH', KEYS[3], ARGV[4])
	return {0, redis.call('SCARD', KEYS[1])}
else
	redis.call('SADD', KEYS[1], ARGV[1])
	redis.call('ZINCRBY', KEYS[2], 1, ARGV[2])
	redis.call('RPUSH', KEYS[3], ARGV[3])
	return {1, redis.call('SCARD', KEYS[1])}
end`)

// toggleFavScript 原子：收藏/取消（不参与榜单）。返回 {favorited, count}。
var toggleFavScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then
	redis.call('SREM', KEYS[1], ARGV[1])
	redis.call('RPUSH', KEYS[2], ARGV[3])
	return {0, redis.call('SCARD', KEYS[1])}
else
	redis.call('SADD', KEYS[1], ARGV[1])
	redis.call('RPUSH', KEYS[2], ARGV[2])
	return {1, redis.call('SCARD', KEYS[1])}
end`)

// playScript 原子：播放 +1 + 更新播放榜 + 推事件。返回新计数。
var playScript = redis.NewScript(`
redis.call('INCR', KEYS[1])
redis.call('ZINCRBY', KEYS[2], 1, ARGV[1])
redis.call('RPUSH', KEYS[3], ARGV[2])
return redis.call('GET', KEYS[1])
`)

func (s *Service) newEvent(t EventType, videoID, userID int64) Event {
	return Event{ID: snowflake.GenID(), Type: t, VideoID: videoID, UserID: userID, At: time.Now().Unix()}
}

func marshalEvent(e Event) string {
	b, _ := json.Marshal(e)
	return string(b)
}

// ToggleLike 点赞/取消点赞。返回是否已赞(true=已赞)与最新点赞数。
func (s *Service) ToggleLike(ctx context.Context, videoID, userID int64) (bool, int64, error) {
	on := s.newEvent(EventLike, videoID, userID)
	off := s.newEvent(EventUnlike, videoID, userID)

	vals, err := toggleLikeScript.Run(ctx, s.rdb,
		[]string{likeKey(videoID), hotLikeKey, pendingKey},
		strconv.FormatInt(userID, 10),
		strconv.FormatInt(videoID, 10),
		marshalEvent(on),
		marshalEvent(off),
	).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(vals) != 2 {
		return false, 0, fmt.Errorf("unexpected toggle result: %v", vals)
	}
	return vals[0] == 1, vals[1], nil
}

// ToggleFavorite 收藏/取消收藏。
func (s *Service) ToggleFavorite(ctx context.Context, videoID, userID int64) (bool, int64, error) {
	on := s.newEvent(EventFavorite, videoID, userID)
	off := s.newEvent(EventUnfavorite, videoID, userID)

	vals, err := toggleFavScript.Run(ctx, s.rdb,
		[]string{favKey(videoID), pendingKey},
		strconv.FormatInt(userID, 10),
		marshalEvent(on),
		marshalEvent(off),
	).Int64Slice()
	if err != nil {
		return false, 0, err
	}
	if len(vals) != 2 {
		return false, 0, fmt.Errorf("unexpected toggle result: %v", vals)
	}
	return vals[0] == 1, vals[1], nil
}

// RecordPlay 播放上报 +1，返回最新播放量。
func (s *Service) RecordPlay(ctx context.Context, videoID int64) (int64, error) {
	ev := s.newEvent(EventPlay, videoID, 0)
	count, err := playScript.Run(ctx, s.rdb,
		[]string{playKey(videoID), hotPlayKey, pendingKey},
		strconv.FormatInt(videoID, 10),
		marshalEvent(ev),
	).Int64()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Counts 批量读取视频三计数。
func (s *Service) Counts(ctx context.Context, videoIDs []int64) (map[int64]Counts, error) {
	res := make(map[int64]Counts, len(videoIDs))
	if len(videoIDs) == 0 {
		return res, nil
	}

	pipe := s.rdb.Pipeline()
	likeCmds := make([]*redis.IntCmd, 0, len(videoIDs))
	favCmds := make([]*redis.IntCmd, 0, len(videoIDs))
	playCmds := make([]*redis.StringCmd, 0, len(videoIDs))
	for _, id := range videoIDs {
		likeCmds = append(likeCmds, pipe.SCard(ctx, likeKey(id)))
		favCmds = append(favCmds, pipe.SCard(ctx, favKey(id)))
		playCmds = append(playCmds, pipe.Get(ctx, playKey(id)))
	}
	_, _ = pipe.Exec(ctx)

	for i, id := range videoIDs {
		if likeCmds[i].Err() != nil || favCmds[i].Err() != nil {
			return nil, fmt.Errorf("read like/fav count failed: %w", firstErr(likeCmds[i].Err(), favCmds[i].Err()))
		}
		play := int64(0)
		if playCmds[i].Err() == nil {
			play, _ = strconv.ParseInt(playCmds[i].Val(), 10, 64)
		} else if !errors.Is(playCmds[i].Err(), redis.Nil) {
			return nil, fmt.Errorf("read play count failed: %w", playCmds[i].Err())
		}
		res[id] = Counts{
			LikeCount:     likeCmds[i].Val(),
			FavoriteCount: favCmds[i].Val(),
			PlayCount:     play,
		}
	}
	return res, nil
}

// IsLiked 判断用户是否已赞。
func (s *Service) IsLiked(ctx context.Context, videoID, userID int64) (bool, error) {
	return s.rdb.SIsMember(ctx, likeKey(videoID), userID).Result()
}

// IsFavorited 判断用户是否已收藏。
func (s *Service) IsFavorited(ctx context.Context, videoID, userID int64) (bool, error) {
	return s.rdb.SIsMember(ctx, favKey(videoID), userID).Result()
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
