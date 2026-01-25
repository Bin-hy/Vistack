package video

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

type ManifestStatus string

const (
	ManifestStatusReady  ManifestStatus = "ready"
	ManifestStatusFailed ManifestStatus = "failed"
)

// VideoManifest 对应 video_manifest 表
type VideoManifest struct {
	ID        int64          `gorm:"primaryKey;column:id" json:"id"`
	VideoID   int64          `gorm:"not null;column:video_id" json:"video_id"`
	Protocol  string         `gorm:"size:20;not null;column:protocol" json:"protocol"` // dash, hls
	FileID    int64          `gorm:"not null;column:file_id" json:"file_id"`
	Profiles  string         `gorm:"type:jsonb;column:profiles" json:"profiles,omitempty"`
	Status    ManifestStatus `gorm:"size:20;default:ready;column:status" json:"status"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updated_at"`

	Video *Video     `gorm:"foreignKey:VideoID;constraint:false" json:"video,omitempty"`
	File  *file.File `gorm:"foreignKey:FileID;constraint:false" json:"file,omitempty"`
}

func (VideoManifest) TableName() string { return "video_manifest" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoManifest) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
