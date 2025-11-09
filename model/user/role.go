package user

import (
    "github.com/google/uuid"
)

// Role 对应 roles 表
type Role struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    Name        string    `gorm:"size:50;uniqueIndex;column:name" json:"name"`
    Description *string   `gorm:"type:text;column:description" json:"description,omitempty"`

    // 关联（可选）
    Users []User `gorm:"foreignKey:RoleID" json:"users,omitempty"`
}

func (Role) TableName() string { return "roles" }