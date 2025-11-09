package social

import (
    "time"

    "github.com/binhy/vistack/model/user"
    "github.com/binhy/vistack/model/video"
)

// VideoLike 对应 video_likes（复合主键）
type VideoLike struct {
    VideoID   int64     `gorm:"primaryKey;column:video_id" json:"video_id"`
    UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

    Video video.Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  user.User   `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
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

    Video video.Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  *user.User  `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (VideoPlayLog) TableName() string { return "video_play_logs" }