package audit

import (
	"time"

	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/pkg/snowflake"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLog 对应 audit_logs 表
type AuditLog struct {
	ID         int64      `gorm:"primaryKey;column:id" json:"id"`
	UserID     *int64     `gorm:"column:user_id" json:"user_id,omitempty"`
	Action     string     `gorm:"size:255;not null;column:action" json:"action"`
	TargetType *string    `gorm:"size:100;column:target_type" json:"target_type,omitempty"`
	TargetUUID *uuid.UUID `gorm:"type:uuid;column:target_uuid" json:"target_uuid,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`

	User *user.User `gorm:"foreignKey:UserID;constraint:false" json:"user,omitempty"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// BeforeCreate 钩子，创建前生成 ID
func (u *AuditLog) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
