package video

import (
    "time"

    "github.com/google/uuid"
    "github.com/binhy/vistack/model/user"
)

// Video 对应 videos 表
type Video struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    UserID      int64     `gorm:"not null;column:user_id" json:"user_id"`
    Title       string    `gorm:"size:255;not null;column:title" json:"title"`
    Description *string   `gorm:"type:text;column:description" json:"description,omitempty"`
    CoverURL    *string   `gorm:"type:text;column:cover_url" json:"cover_url,omitempty"`
    Duration    int       `gorm:"default:0;column:duration" json:"duration"`
    Status      string    `gorm:"size:20;default:uploaded;column:status" json:"status"`
    Visibility  string    `gorm:"size:20;default:public;column:visibility" json:"visibility"`
    CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
    UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`

    // 关联
    User       user.User        `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Sources    []VideoSource    `gorm:"foreignKey:VideoID" json:"sources,omitempty"`
    Transcodes []VideoTranscode `gorm:"foreignKey:VideoID" json:"transcodes,omitempty"`
}

func (Video) TableName() string { return "videos" }