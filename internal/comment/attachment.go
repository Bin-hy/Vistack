package comment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	"gorm.io/gorm"
)

const maxAttachments = 9

var ErrTooManyAttachments = errors.New("too many attachments")

// ParseAttachments 反序列化附件 JSON（空串返回 nil）。
func ParseAttachments(raw string) ([]mSocial.CommentAttachment, error) {
	if raw == "" {
		return nil, nil
	}
	var items []mSocial.CommentAttachment
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func marshalAttachments(items []mSocial.CommentAttachment) (string, error) {
	if len(items) == 0 {
		return "[]", nil // 空数组是合法 JSON，避免 jsonb 列写入空串报错
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func validateAttachments(ctx context.Context, db *gorm.DB, items []mSocial.CommentAttachment) error {
	if len(items) > maxAttachments {
		return ErrTooManyAttachments
	}
	for _, it := range items {
		if it.FileID == 0 {
			return errors.New("attachment file_id is zero")
		}
		var f mFile.File
		if err := db.WithContext(ctx).First(&f, it.FileID).Error; err != nil {
			return fmt.Errorf("attachment file %d not found: %w", it.FileID, err)
		}
		if f.RefType != mFile.FileRefTypeCommentImage {
			return fmt.Errorf("attachment file %d has invalid ref_type", it.FileID)
		}
	}
	return nil
}
