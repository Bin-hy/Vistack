package social

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// CommentStatus 评论状态机：visible / pending / hidden / deleted。
type CommentStatus string

const (
	CommentStatusVisible CommentStatus = "visible" // 可见
	CommentStatusPending CommentStatus = "pending" // 含图待审
	CommentStatusHidden  CommentStatus = "hidden"  // 审核拒绝
	CommentStatusDeleted CommentStatus = "deleted" // 软删除（占位）
)

// CommentAttachment 评论附件（图片/表情包），有序，序列化后存于 VideoComment.Attachments JSONB。
type CommentAttachment struct {
	Type   string `json:"type"`    // "image"（图片）| "sticker"（表情包 GIF/PNG）
	FileID int64  `json:"file_id"` // 引用 files.id
}

// VideoComment 对应 video_comments 表。
type VideoComment struct {
	ID          int64         `gorm:"primaryKey;column:id" json:"id"`
	VideoID     int64         `gorm:"not null;column:video_id" json:"video_id"`
	UserID      int64         `gorm:"not null;column:user_id" json:"user_id"`
	RootID      *int64        `gorm:"column:root_id" json:"root_id,omitempty"`           // 根评论 id，nil=一级评论
	ParentID    *int64        `gorm:"column:parent_id" json:"parent_id,omitempty"`       // 直接父评论 id
	ReplyToID   *int64        `gorm:"column:reply_to_id" json:"reply_to_id,omitempty"`   // 被精确回复评论 id
	ReplyToUID  *int64        `gorm:"column:reply_to_uid" json:"reply_to_uid,omitempty"` // 被回复用户 id（冗余）
	Content     string        `gorm:"type:text;not null;column:content" json:"content"`
	Attachments string        `gorm:"type:jsonb;default:'[]';column:attachments" json:"attachments,omitempty"` // JSON: []CommentAttachment
	Status      CommentStatus `gorm:"size:20;not null;default:visible;column:status" json:"status"`
	LikeCount   int64         `gorm:"default:0;column:like_count" json:"like_count"`
	ReplyCount  int64         `gorm:"default:0;column:reply_count" json:"reply_count"`
	CreatedAt   time.Time     `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time     `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt   *time.Time    `gorm:"index;column:deleted_at" json:"-"`

	Video video.Video `gorm:"foreignKey:VideoID;constraint:false" json:"-"`
	User  user.User   `gorm:"foreignKey:UserID;constraint:false" json:"-"`
}

func (VideoComment) TableName() string { return "video_comments" }

// BeforeCreate 钩子，创建前生成 ID
func (u *VideoComment) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
