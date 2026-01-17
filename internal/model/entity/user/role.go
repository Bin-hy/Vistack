package user

import (
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// Role 对应 roles 表
type Role struct {
	ID          int64   `gorm:"primaryKey;column:id" json:"id"`
	Name        string  `gorm:"size:50;uniqueIndex;column:name" json:"name"`
	Description *string `gorm:"type:text;column:description" json:"description,omitempty"`

	// 关联（可选）
	Users []User `gorm:"foreignKey:RoleID" json:"users,omitempty"`
}

func (Role) TableName() string { return "roles" }

// BeforeCreate 钩子，创建前生成 ID
func (u *Role) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
