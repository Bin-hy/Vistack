package models

import (
    "time"

    "github.com/google/uuid"
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

    Video   Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User    User  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Parent  *VideoComment `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parent,omitempty"`
}

func (VideoComment) TableName() string { return "video_comments" }

// VideoLike 对应 video_likes（复合主键）
type VideoLike struct {
    VideoID   int64     `gorm:"primaryKey;column:video_id" json:"video_id"`
    UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

    Video Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  User  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
}

func (VideoLike) TableName() string { return "video_likes" }

// VideoFavorite 对应 video_favorites（复合主键）
type VideoFavorite struct {
    VideoID   int64     `gorm:"primaryKey;column:video_id" json:"video_id"`
    UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
    CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`

    Video Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  User  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
}

func (VideoFavorite) TableName() string { return "video_favorites" }

// VideoPlayLog 对应 video_play_logs 表
type VideoPlayLog struct {
    ID        int64     `gorm:"primaryKey;column:id" json:"id"`
    VideoID   int64     `gorm:"not null;column:video_id" json:"video_id"`
    UserID    *int64    `gorm:"column:user_id" json:"user_id,omitempty"`
    PlayedAt  time.Time `gorm:"column:played_at" json:"played_at"`
    IPAddress *string   `gorm:"type:inet;column:ip_address" json:"ip_address,omitempty"`
    UserAgent *string   `gorm:"type:text;column:user_agent" json:"user_agent,omitempty"`

    Video Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    User  *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (VideoPlayLog) TableName() string { return "video_play_logs" }