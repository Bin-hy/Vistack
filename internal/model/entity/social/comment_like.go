package social

import "time"

// CommentLike 对应 comment_likes（复合主键：评论 id + 用户 id）。
type CommentLike struct {
	CommentID int64     `gorm:"primaryKey;column:comment_id" json:"comment_id"`
	UserID    int64     `gorm:"primaryKey;column:user_id" json:"user_id"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (CommentLike) TableName() string { return "comment_likes" }
