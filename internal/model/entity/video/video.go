package video

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

type VideoStatus string

const (
	VideoStatusUploaded   VideoStatus = "uploaded"
	VideoStatusProcessing VideoStatus = "processing"
	VideoStatusPublished  VideoStatus = "published"
	VideoStatusFailed     VideoStatus = "failed"
	VideoStatusDeleted    VideoStatus = "deleted"
)

type VideoVisibility string

const (
	VideoVisibilityPublic   VideoVisibility = "public"
	VideoVisibilityPrivate  VideoVisibility = "private"
	VideoVisibilityUnlisted VideoVisibility = "unlisted"
)

// Video 对应 videos 表
type Video struct {
	ID          int64           `gorm:"primaryKey;column:id" json:"id"`
	UserID      int64           `gorm:"not null;column:user_id" json:"user_id"`
	Title       string          `gorm:"size:255;not null;column:title" json:"title"`
	Description *string         `gorm:"type:text;column:description" json:"description,omitempty"`
	CoverFileID *int64          `gorm:"column:cover_file_id" json:"cover_file_id,omitempty"`
	Duration    int             `gorm:"default:0;column:duration" json:"duration"`
	Status      VideoStatus     `gorm:"size:20;default:uploaded;column:status" json:"status"`
	Visibility  VideoVisibility `gorm:"size:20;default:public;column:visibility" json:"visibility"`
	CreatedAt   time.Time       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at" json:"updated_at"`

	// 关联
	User       *user.User       `gorm:"foreignKey:UserID;constraint:false" json:"user,omitempty"`
	CoverFile  *file.File       `gorm:"foreignKey:CoverFileID;constraint:false" json:"cover_file,omitempty"`
	Sources    []VideoSource    `gorm:"foreignKey:VideoID" json:"sources,omitempty"`
	Transcodes []VideoTranscode `gorm:"foreignKey:VideoID" json:"transcodes,omitempty"`
	Manifests  []VideoManifest  `gorm:"foreignKey:VideoID" json:"manifests,omitempty"`
}

func (Video) TableName() string { return "videos" }

// BeforeCreate 钩子，创建前生成 ID
func (v *Video) BeforeCreate(tx *gorm.DB) (err error) {
	if v.ID == 0 {
		v.ID = snowflake.GenID()
	}
	return
}
