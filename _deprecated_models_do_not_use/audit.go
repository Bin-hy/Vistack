package models

import (
    "time"

    "github.com/google/uuid"
)

// AuditLog 对应 audit_logs 表
type AuditLog struct {
    ID         int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID       uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    UserID     *int64    `gorm:"column:user_id" json:"user_id,omitempty"`
    Action     string    `gorm:"size:255;not null;column:action" json:"action"`
    TargetType *string   `gorm:"size:100;column:target_type" json:"target_type,omitempty"`
    TargetUUID *uuid.UUID `gorm:"type:uuid;column:target_uuid" json:"target_uuid,omitempty"`
    CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`

    User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (AuditLog) TableName() string { return "audit_logs" }