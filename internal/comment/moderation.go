package comment

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	"gorm.io/gorm"
)

// Moderator 图片内容安全审核器（可插拔）。
type Moderator interface {
	// Review 对评论附件的文件列表做审核，true=通过。
	Review(ctx context.Context, files []mFile.File) (bool, error)
}

// PassthroughModerator 桩实现：未接入第三方时默认通过。
type PassthroughModerator struct{}

func (PassthroughModerator) Review(ctx context.Context, files []mFile.File) (bool, error) {
	return true, nil
}

// ModerationMessage 审核队列消息。
type ModerationMessage struct {
	CommentID int64 `json:"comment_id"`
}

// EnqueueModeration 投递审核消息到 Kafka。
func (s *Service) EnqueueModeration(ctx context.Context, commentID int64) error {
	raw, err := json.Marshal(ModerationMessage{CommentID: commentID})
	if err != nil {
		return err
	}
	return core.SendKafkaMessage(ctx, string(consts.KafkaTopicCommentModeration), strconv.FormatInt(commentID, 10), raw)
}

// ProcessModeration 完整审核流程：读附件文件 → 调审核器 → 回写结果。
func (s *Service) ProcessModeration(ctx context.Context, commentID int64, mod Moderator) error {
	var c mSocial.VideoComment
	if err := s.db.WithContext(ctx).First(&c, commentID).Error; err != nil {
		return err
	}
	if c.Status != mSocial.CommentStatusPending {
		return nil // 非待审状态，幂等忽略
	}
	items, err := ParseAttachments(c.Attachments)
	if err != nil {
		return err
	}
	files := make([]mFile.File, 0, len(items))
	for _, it := range items {
		var f mFile.File
		if err := s.db.WithContext(ctx).First(&f, it.FileID).Error; err != nil {
			return err
		}
		files = append(files, f)
	}
	pass, err := mod.Review(ctx, files)
	if err != nil {
		return err
	}
	return s.ApplyModerationResult(ctx, commentID, pass)
}

// ApplyModerationResult 审核结果回写：通过→visible，拒绝→hidden 并回收附件引用。
func (s *Service) ApplyModerationResult(ctx context.Context, commentID int64, pass bool) error {
	var c mSocial.VideoComment
	if err := s.db.WithContext(ctx).First(&c, commentID).Error; err != nil {
		return err
	}
	if c.Status != mSocial.CommentStatusPending {
		return nil // 非待审状态，幂等忽略
	}

	target := mSocial.CommentStatusVisible
	if !pass {
		target = mSocial.CommentStatusHidden
	}

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&mSocial.VideoComment{}).Where("id = ?", commentID).
			Update("status", target).Error; err != nil {
			return err
		}
		// 拒绝的回复：回减根评论的 reply_count（创建时已 +1）
		if !pass && c.RootID != nil {
			if err := tx.Model(&mSocial.VideoComment{}).Where("id = ?", *c.RootID).
				UpdateColumn("reply_count", gorm.Expr("GREATEST(reply_count - 1, 0)")).Error; err != nil {
				return err
			}
		}
		if !pass && c.Attachments != "" {
			items, _ := ParseAttachments(c.Attachments)
			for _, a := range items {
				if err := tx.Model(&mFile.File{}).Where("id = ?", a.FileID).
					UpdateColumn("ref_count", gorm.Expr("GREATEST(ref_count - 1, 0)")).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 通过后的一级评论计入视频评论总数
	if pass && c.RootID == nil {
		_ = s.rdb.Incr(ctx, commentCountKey(c.VideoID)).Err()
	}
	s.invalidateListCache(ctx, c.VideoID)
	return nil
}
