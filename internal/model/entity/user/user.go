package user

import (
	"time"

	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// User 对应 users 表
type User struct {
	ID           int64     `gorm:"primaryKey;column:id" json:"id"`
	Username     string    `gorm:"size:50;uniqueIndex;column:username" json:"username"`
	Email        *string   `gorm:"size:100;column:email" json:"email,omitempty"`
	PasswordHash string    `gorm:"type:text;not null;column:password_hash" json:"-"`
	RoleID       int64     `gorm:"not null;column:role_id" json:"role_id"`
	Status       string    `gorm:"size:20;default:active;column:status" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`

	// 关联
	Role    *Role        `gorm:"foreignKey:RoleID;constraint:false" json:"role,omitempty"`
	Profile *UserProfile `gorm:"foreignKey:UserID;constraint:false" json:"profile,omitempty"`
}

func (User) TableName() string { return "users" }

// BeforeCreate 钩子，创建前生成 ID
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
