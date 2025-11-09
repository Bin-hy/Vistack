package video

import (
    "github.com/google/uuid"
)

// VideoSegment 对应 video_segments 表
type VideoSegment struct {
    ID           int64     `gorm:"primaryKey;column:id" json:"id"`
    TranscodeID  int64     `gorm:"not null;column:transcode_id" json:"transcode_id"`
    UUID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    SegmentIndex int       `gorm:"column:segment_index" json:"segment_index"`
    Duration     *float64  `gorm:"column:duration" json:"duration,omitempty"`
    SegmentURL   string    `gorm:"type:text;not null;column:segment_url" json:"segment_url"`

    Transcode VideoTranscode `gorm:"foreignKey:TranscodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"transcode"`
}

func (VideoSegment) TableName() string { return "video_segments" }