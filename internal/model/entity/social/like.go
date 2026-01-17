package social

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// VideoLike 对应 video_likes（复合主键）
type VideoLike struct {
	VideoID   int64     `gorm:"primaryKey;column:video_id" json:"video_id"`
	UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

	Video video.Video `gorm:"foreignKey:VideoID;constraint:false" json:"video"`
	User  user.User   `gorm:"foreignKey:UserID;constraint:false" json:"user"`
}

func (VideoLike) TableName() string { return "video_likes" }

// VideoPlayLog 对应 video_play_logs 表
type VideoPlayLog struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	VideoID   int64     `gorm:"not null;column:video_id" json:"video_id"`
	UserID    *int64    `gorm:"column:user_id" json:"user_id,omitempty"`
	PlayedAt  time.Time `gorm:"column:played_at" json:"played_at"`
	IPAddress *string   `gorm:"type:inet;column:ip_address" json:"ip_address,omitempty"`
	UserAgent *string   `gorm:"type:text;column:user_agent" json:"user_agent,omitempty"`

	Video video.Video `gorm:"foreignKey:VideoID;constraint:false" json:"video"`
	User  *user.User  `gorm:"foreignKey:UserID;constraint:false" json:"user,omitempty"`
}

func (VideoPlayLog) TableName() string { return "video_play_logs" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoPlayLog) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
