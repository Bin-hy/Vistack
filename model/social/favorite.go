package social

import (
    "time"

    "github.com/binhy/vistack/model/user"
    "github.com/binhy/vistack/model/video"
)

// VideoFavorite 对应 video_favorites（复合主键）
type VideoFavorite struct {
    VideoID   int64     `gorm:"primaryKey;column:video_id" json:"video_id"`
    UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

    Video video.Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  user.User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
}

func (VideoFavorite) TableName() string { return "video_favorites" }