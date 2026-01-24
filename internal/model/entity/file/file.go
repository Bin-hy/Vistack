package file

import (
	"fmt"
	"strings"
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

type FileStatus string

const (
	FileStatusActive   FileStatus = "active"
	FileStatusDeleting FileStatus = "deleting"
	FileStatusDeleted  FileStatus = "deleted"
)

type FileRefType string

const (
	FileRefTypeAvatar            FileRefType = "avatar"
	FileRefTypeVideoSource       FileRefType = "video_source"
	FileRefTypeTranscodeArtifact FileRefType = "transcode_artifact"
	FileRefTypeVideoManifest     FileRefType = "video_manifest"
	FileRefTypeVideoCover        FileRefType = "video_cover"
)

// File 对应 files 表，表示 MinIO 中的物理文件对象
// 必须通过 RefCount 管理生命周期，严禁直接删除
type File struct {
	ID        int64       `gorm:"primaryKey;column:id" json:"id"`
	Bucket    string      `gorm:"size:100;not null;column:bucket" json:"bucket"`
	ObjectKey string      `gorm:"type:text;not null;column:object_key" json:"object_key"`
	Status    FileStatus  `gorm:"size:20;not null;default:active;column:status" json:"status"` // active, deleting, deleted
	RefType   FileRefType `gorm:"size:50;column:ref_type" json:"ref_type"`                     // avatar, video_source, transcode_artifact
	MimeType  string      `gorm:"size:100;column:mime_type" json:"mime_type"`
	Hash      string      `gorm:"size:64;index;column:hash" json:"hash"` // 全局唯一标识 (SHA-256)
	Size      int64       `gorm:"column:size" json:"size"`
	RefCount  int         `gorm:"default:0;column:ref_count" json:"ref_count"` // 引用计数
	CreatedAt time.Time   `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt time.Time   `gorm:"column:updated_at;default:now()" json:"updated_at"`
}

func (File) TableName() string {
	return "files"
}

func (f *File) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == 0 {
		f.ID = snowflake.GenID()
	}
	return
}

// 返回文件的public URL
func (f *File) PublicURL(baseURL string) string {
	if f == nil {
		return ""
	}

	return fmt.Sprintf("%s/%s/%s",
		strings.TrimRight(baseURL, "/"),
		f.Bucket,
		f.ObjectKey)
}
