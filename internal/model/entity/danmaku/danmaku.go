package danmaku

import (
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// Danmaku 弹幕，绑定视频时间轴（TimeOffset 为相对视频起点的秒数）。
type Danmaku struct {
	ID         int64     `gorm:"primaryKey;column:id" json:"id"`
	VideoID    int64     `gorm:"not null;index;column:video_id" json:"video_id"`
	UserID     int64     `gorm:"not null;column:user_id" json:"user_id"`
	Content    string    `gorm:"type:text;not null;column:content" json:"content"`
	TimeOffset float64   `gorm:"column:time_offset" json:"time_offset"`
	Color      string    `gorm:"size:20;column:color" json:"color,omitempty"`
	Mode       int       `gorm:"column:mode" json:"mode"` // 0滚动 1顶部 2底部
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Danmaku) TableName() string { return "danmakus" }

// BeforeCreate 生成雪花 ID。
func (d *Danmaku) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == 0 {
		d.ID = snowflake.GenID()
	}
	return nil
}
