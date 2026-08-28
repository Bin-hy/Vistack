package danmaku

import (
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// SensitiveWord 敏感词（AC 自动机关键词，管理端维护）。
type SensitiveWord struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Word      string    `gorm:"size:100;not null;uniqueIndex;column:word" json:"word"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SensitiveWord) TableName() string { return "sensitive_words" }

// BeforeCreate 生成雪花 ID。
func (s *SensitiveWord) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == 0 {
		s.ID = snowflake.GenID()
	}
	return nil
}
