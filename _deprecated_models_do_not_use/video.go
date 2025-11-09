package models

import (
    "time"

    "github.com/google/uuid"
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
    User        User            `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Sources     []VideoSource   `gorm:"foreignKey:VideoID" json:"sources,omitempty"`
    Transcodes  []VideoTranscode `gorm:"foreignKey:VideoID" json:"transcodes,omitempty"`
}

func (Video) TableName() string { return "videos" }

// VideoSource 对应 video_sources 表
type VideoSource struct {
    ID         int64     `gorm:"primaryKey;column:id" json:"id"`
    VideoID    int64     `gorm:"not null;column:video_id" json:"video_id"`
    UUID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    Bucket     string    `gorm:"size:100;not null;column:bucket" json:"bucket"`
    ObjectPath string    `gorm:"type:text;not null;column:object_path" json:"object_path"`
    Size       *int64    `gorm:"column:size" json:"size,omitempty"`
    MimeType   *string   `gorm:"size:100;column:mime_type" json:"mime_type,omitempty"`
    UploadedAt time.Time `gorm:"column:uploaded_at" json:"uploaded_at"`

    Video Video `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
}

func (VideoSource) TableName() string { return "video_sources" }

// VideoTranscode 对应 video_transcodes 表
type VideoTranscode struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    VideoID     int64     `gorm:"not null;column:video_id" json:"video_id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    Status      string    `gorm:"size:20;default:pending;column:status" json:"status"`
    Resolution  *string   `gorm:"size:20;column:resolution" json:"resolution,omitempty"`
    Codec       *string   `gorm:"size:50;column:codec" json:"codec,omitempty"`
    ManifestURL *string   `gorm:"type:text;column:manifest_url" json:"manifest_url,omitempty"`
    CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
    UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`

    Video     Video          `gorm:"foreignKey:VideoID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"video"`
    Segments  []VideoSegment `gorm:"foreignKey:TranscodeID" json:"segments,omitempty"`
}

func (VideoTranscode) TableName() string { return "video_transcodes" }

// VideoSegment 对应 video_segments 表
type VideoSegment struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    TranscodeID int64     `gorm:"not null;column:transcode_id" json:"transcode_id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    SegmentIndex int      `gorm:"column:segment_index" json:"segment_index"`
    Duration    *float64  `gorm:"column:duration" json:"duration,omitempty"`
    SegmentURL  string    `gorm:"type:text;not null;column:segment_url" json:"segment_url"`

    Transcode VideoTranscode `gorm:"foreignKey:TranscodeID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"transcode"`
}

func (VideoSegment) TableName() string { return "video_segments" }