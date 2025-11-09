package social

import (
    "time"

    "github.com/google/uuid"
    "github.com/binhy/vistack/model/user"
    "github.com/binhy/vistack/model/video"
)

// VideoComment 对应 video_comments 表
type VideoComment struct {
    ID        int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    VideoID   int64     `gorm:"not null;column:video_id" json:"video_id"`
    UserID    int64     `gorm:"not null;column:user_id" json:"user_id"`
    ParentID  *int64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
    Content   string    `gorm:"type:text;not null;column:content" json:"content"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

    Video  video.Video     `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User   user.User       `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Parent *VideoComment   `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parent,omitempty"`
}

func (VideoComment) TableName() string { return "video_comments" }