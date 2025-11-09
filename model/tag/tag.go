package tag

import (
    "github.com/google/uuid"
    "github.com/binhy/vistack/model/video"
)

// Tag 对应 tags 表
type Tag struct {
    ID   int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    Name string    `gorm:"size:50;uniqueIndex;column:name" json:"name"`
}

func (Tag) TableName() string { return "tags" }

// VideoTag 对应视频与标签的关联（复合主键）
type VideoTag struct {
    VideoID int64 `gorm:"primaryKey;column:video_id" json:"video_id"`
    TagID   int64 `gorm:"primaryKey;column:tag_id" json:"tag_id"`

    Video video.Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    Tag   Tag         `gorm:"foreignKey:TagID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"tag"`
}

func (VideoTag) TableName() string { return "video_tags" }