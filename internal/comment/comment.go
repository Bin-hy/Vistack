package comment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/binhy/vistack/internal/danmaku"
	entityDanmaku "github.com/binhy/vistack/internal/model/entity/danmaku"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	"github.com/binhy/vistack/pkg/snowflake"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 领域错误。
var (
	ErrSensitive = errors.New("sensitive word")
	ErrNotFound  = errors.New("comment not found")
	ErrForbidden = errors.New("forbidden")
)

// Options 构造参数。
type Options struct {
	FlushInterval time.Duration
	FlushBatch    int
	Logger        *zap.Logger
}

// CreateInput 发表评论/回复入参。
type CreateInput struct {
	VideoID     int64
	UserID      int64
	Content     string
	ParentID    *int64 // 回复时传；一级评论为 nil
	ReplyToID   *int64 // 精确 @ 对象，可为 nil
	Attachments []mSocial.CommentAttachment
}

// Service 视频评论服务。
type Service struct {
	rdb    *redis.Client
	db     *gorm.DB
	filter *danmaku.SensitiveFilter
	opts   Options
}

func NewService(rdb *redis.Client, db *gorm.DB, opts Options) *Service {
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 5 * time.Second
	}
	if opts.FlushBatch <= 0 {
		opts.FlushBatch = 200
	}
	return &Service{
		rdb:    rdb,
		db:     db,
		filter: danmaku.NewSensitiveFilter(nil),
		opts:   opts,
	}
}

func (s *Service) log() *zap.Logger { return s.opts.Logger }

// LoadSensitiveWords 从 DB 加载敏感词并重建 AC 自动机。
func (s *Service) LoadSensitiveWords(ctx context.Context) error {
	var words []entityDanmaku.SensitiveWord
	if err := s.db.WithContext(ctx).Find(&words).Error; err != nil {
		return err
	}
	list := make([]string, 0, len(words))
	for _, w := range words {
		list = append(list, w.Word)
	}
	s.filter.Reload(list)
	return nil
}

// Create 发表评论/回复：敏感词过滤 → 推导级联 → 落库 → 计数 → 审核投递。
func (s *Service) Create(ctx context.Context, in CreateInput) (*mSocial.VideoComment, error) {
	if s.filter.Contains(in.Content) {
		return nil, ErrSensitive
	}
	if err := validateAttachments(ctx, s.db, in.Attachments); err != nil {
		return nil, err
	}

	var rootID, replyToID, replyToUID *int64
	parentID := in.ParentID

	if in.ParentID != nil {
		var parent mSocial.VideoComment
		if err := s.db.WithContext(ctx).First(&parent, *in.ParentID).Error; err != nil {
			return nil, ErrNotFound
		}
		if parent.Status != mSocial.CommentStatusVisible {
			return nil, ErrNotFound
		}
		if parent.RootID != nil {
			rootID = parent.RootID
		} else {
			rootID = &parent.ID
		}
	}

	if in.ReplyToID != nil {
		var rt mSocial.VideoComment
		if err := s.db.WithContext(ctx).First(&rt, *in.ReplyToID).Error; err != nil {
			return nil, ErrNotFound
		}
		if rt.Status != mSocial.CommentStatusVisible {
			return nil, ErrNotFound
		}
		if rootID != nil {
			rtRoot := rt.RootID
			if rtRoot == nil {
				rtRoot = &rt.ID
			}
			if *rtRoot != *rootID {
				return nil, errors.New("reply_to not in same thread")
			}
		}
		replyToID = in.ReplyToID
		uid := rt.UserID
		replyToUID = &uid
	}

	rawAtt, err := marshalAttachments(in.Attachments)
	if err != nil {
		return nil, err
	}

	status := mSocial.CommentStatusVisible
	if len(in.Attachments) > 0 {
		status = mSocial.CommentStatusPending
	}

	now := time.Now()
	c := &mSocial.VideoComment{
		ID:          snowflake.GenID(),
		VideoID:     in.VideoID,
		UserID:      in.UserID,
		RootID:      rootID,
		ParentID:    parentID,
		ReplyToID:   replyToID,
		ReplyToUID:  replyToUID,
		Content:     in.Content,
		Attachments: rawAtt,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(c).Error; err != nil {
			return err
		}
		for _, a := range in.Attachments {
			if err := tx.Model(&mFile.File{}).Where("id = ?", a.FileID).
				UpdateColumn("ref_count", gorm.Expr("ref_count + 1")).Error; err != nil {
				return err
			}
		}
		if rootID != nil {
			if err := tx.Model(&mSocial.VideoComment{}).Where("id = ?", *rootID).
				UpdateColumn("reply_count", gorm.Expr("reply_count + 1")).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 可见的一级评论：视频评论总数 +1（Redis，best-effort）
	if rootID == nil && status == mSocial.CommentStatusVisible {
		_ = s.rdb.Incr(ctx, commentCountKey(in.VideoID)).Err()
	}
	s.invalidateListCache(ctx, in.VideoID)

	if len(in.Attachments) > 0 {
		if err := s.EnqueueModeration(ctx, c.ID); err != nil && s.log() != nil {
			s.log().Warn("enqueue moderation failed", zap.Int64("comment_id", c.ID), zap.Error(err))
		}
	}

	return c, nil
}

type listCacheResult struct {
	Comments []mSocial.VideoComment `json:"comments"`
	Next     int64                  `json:"next"`
}

// List 一级评论列表（游标分页；首屏带 Redis 缓存）。
func (s *Service) List(ctx context.Context, videoID, cursor int64, limit int) ([]mSocial.VideoComment, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if cursor == 0 {
		key := listCacheKey(videoID)
		if raw, err := s.rdb.Get(ctx, key).Result(); err == nil {
			var r listCacheResult
			if json.Unmarshal([]byte(raw), &r) == nil {
				return r.Comments, r.Next, nil
			}
		}
	}

	roots, next, err := s.queryRoots(ctx, videoID, cursor, limit)
	if err != nil {
		return nil, 0, err
	}
	if cursor == 0 {
		s.cacheList(ctx, videoID, roots, next)
	}
	return roots, next, nil
}

func (s *Service) queryRoots(ctx context.Context, videoID, cursor int64, limit int) ([]mSocial.VideoComment, int64, error) {
	q := s.db.WithContext(ctx).
		Where("video_id = ? AND root_id IS NULL AND (status = ? OR (status = ? AND reply_count > 0))",
			videoID, mSocial.CommentStatusVisible, mSocial.CommentStatusDeleted)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var roots []mSocial.VideoComment
	if err := q.Order("id desc").Limit(limit).Find(&roots).Error; err != nil {
		return nil, 0, err
	}
	var next int64
	if len(roots) == limit {
		next = roots[len(roots)-1].ID
	}
	return roots, next, nil
}

// ListReplies 展开某根评论下的全部回复。
func (s *Service) ListReplies(ctx context.Context, rootID, cursor int64, limit int) ([]mSocial.VideoComment, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	q := s.db.WithContext(ctx).Where("root_id = ? AND status = ?", rootID, mSocial.CommentStatusVisible)
	if cursor > 0 {
		q = q.Where("id < ?", cursor)
	}
	var replies []mSocial.VideoComment
	if err := q.Order("id desc").Limit(limit).Find(&replies).Error; err != nil {
		return nil, 0, err
	}
	var next int64
	if len(replies) == limit {
		next = replies[len(replies)-1].ID
	}
	return replies, next, nil
}

// CommentCount 视频评论总数（Redis 优先，DB 回退）。
func (s *Service) CommentCount(ctx context.Context, videoID int64) (int64, error) {
	if n, err := s.rdb.Get(ctx, commentCountKey(videoID)).Int64(); err == nil {
		return n, nil
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&mSocial.VideoComment{}).
		Where("video_id = ? AND root_id IS NULL AND status = ?", videoID, mSocial.CommentStatusVisible).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Delete 软删除评论（校验归属，保留楼中楼）。
func (s *Service) Delete(ctx context.Context, commentID, userID int64) error {
	var c mSocial.VideoComment
	if err := s.db.WithContext(ctx).First(&c, commentID).Error; err != nil {
		return ErrNotFound
	}
	if c.UserID != userID {
		return ErrForbidden
	}
	if c.Status == mSocial.CommentStatusDeleted {
		return nil
	}

	now := time.Now()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":      mSocial.CommentStatusDeleted,
			"content":     "",
			"attachments": "[]",
			"deleted_at":  now,
		}
		if err := tx.Model(&mSocial.VideoComment{}).Where("id = ?", commentID).Updates(updates).Error; err != nil {
			return err
		}
		if c.Attachments != "" {
			items, _ := ParseAttachments(c.Attachments)
			for _, a := range items {
				if err := tx.Model(&mFile.File{}).Where("id = ?", a.FileID).
					UpdateColumn("ref_count", gorm.Expr("GREATEST(ref_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
		}
		if c.RootID != nil {
			if err := tx.Model(&mSocial.VideoComment{}).Where("id = ?", *c.RootID).
				UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if c.RootID == nil {
		_ = s.rdb.Decr(ctx, commentCountKey(c.VideoID)).Err()
	}
	s.invalidateListCache(ctx, c.VideoID)
	return nil
}

func (s *Service) cacheList(ctx context.Context, videoID int64, roots []mSocial.VideoComment, next int64) {
	raw, err := json.Marshal(listCacheResult{Comments: roots, Next: next})
	if err != nil {
		return
	}
	if err := s.rdb.Set(ctx, listCacheKey(videoID), string(raw), 60*time.Second).Err(); err != nil && s.log() != nil {
		s.log().Warn("cache comment list failed", zap.Int64("video_id", videoID), zap.Error(err))
	}
}

func (s *Service) invalidateListCache(ctx context.Context, videoID int64) {
	_ = s.rdb.Del(ctx, listCacheKey(videoID)).Err()
}
