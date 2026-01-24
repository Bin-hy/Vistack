package video

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

type TranscodeStatus string

const (
	TranscodeStatusPending    TranscodeStatus = "pending"
	TranscodeStatusProcessing TranscodeStatus = "processing"
	TranscodeStatusCompleted  TranscodeStatus = "completed"
	TranscodeStatusFailed     TranscodeStatus = "failed"
)

// VideoSource 对应 video_sources 表
type VideoSource struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	VideoID    int64     `gorm:"not null;column:video_id" json:"video_id"`
	FileID     int64     `gorm:"not null;column:file_id" json:"file_id"`
	UploadedAt time.Time `gorm:"column:uploaded_at" json:"uploaded_at"`

	Video *Video     `gorm:"foreignKey:VideoID;constraint:false" json:"video,omitempty"`
	File  *file.File `gorm:"foreignKey:FileID;constraint:false" json:"file"`
}

func (VideoSource) TableName() string { return "video_sources" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoSource) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}

// VideoTranscode 对应 video_transcodes 表
type VideoTranscode struct {
	ID             int64           `gorm:"primaryKey;column:id" json:"id"`
	VideoID        int64           `gorm:"not null;column:video_id" json:"video_id"`
	Status         TranscodeStatus `gorm:"size:20;default:pending;column:status" json:"status"`
	Resolution     *string         `gorm:"size:100;column:resolution" json:"resolution,omitempty"`
	Codec          *string         `gorm:"size:50;column:codec" json:"codec,omitempty"`
	ManifestFileID *int64          `gorm:"column:manifest_file_id" json:"manifest_file_id,omitempty"`
	CreatedAt      time.Time       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"column:updated_at" json:"updated_at"`

	Video        *Video     `gorm:"foreignKey:VideoID;constraint:false" json:"video,omitempty"`
	ManifestFile *file.File `gorm:"foreignKey:ManifestFileID;constraint:false" json:"manifest_file,omitempty"`
}

func (VideoTranscode) TableName() string { return "video_transcodes" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoTranscode) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
