package social

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// VideoComment 对应 video_comments 表
type VideoComment struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	VideoID   int64     `gorm:"not null;column:video_id" json:"video_id"`
	UserID    int64     `gorm:"not null;column:user_id" json:"user_id"`
	ParentID  *int64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Content   string    `gorm:"type:text;not null;column:content" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

	Video  video.Video   `gorm:"foreignKey:VideoID;constraint:false" json:"video"`
	User   user.User     `gorm:"foreignKey:UserID;constraint:false" json:"user"`
	Parent *VideoComment `gorm:"foreignKey:ParentID;constraint:false" json:"parent,omitempty"`
}

func (VideoComment) TableName() string { return "video_comments" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoComment) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
