package file

import (
	"fmt"
	"strings"
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

type File struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	Bucket    string    `gorm:"size:100;not null;column:bucket" json:"bucket"`
	ObjectKey string    `gorm:"type:text;not null;column:object_key" json:"object_key"`
	Status    string    `gorm:"size:20;not null;default:active;column:status" json:"status"` // active, inactive, replaced, deleted
	RefType   string    `gorm:"size:50;column:ref_type" json:"ref_type"`                     // avatar, video, etc.
	RefID     int64     `gorm:"column:ref_id" json:"ref_id"`
	MimeType  string    `gorm:"size:100;column:mime_type" json:"mime_type"`
	Size      int64     `gorm:"column:size" json:"size"`
	CreatedAt time.Time `gorm:"column:created_at;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;default:now()" json:"updated_at"`
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
