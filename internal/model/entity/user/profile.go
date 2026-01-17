package user

import (
	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// UserProfile 对应 user_profiles 表
type UserProfile struct {
	ID           int64   `gorm:"primaryKey;column:id" json:"id"`
	UserID       int64   `gorm:"not null;column:user_id" json:"user_id"`
	Nickname     *string `gorm:"size:100;column:nickname;unique" json:"nickname,omitempty"`
	AvatarFileID *int64  `gorm:"column:avatar_file_id" json:"avatar_file_id,omitempty"`

	// 关联
	User   *User      `gorm:"foreignKey:UserID;constraint:false" json:"user,omitempty"`
	Avatar *file.File `gorm:"foreignKey:AvatarFileID;constraint:false" json:"avatar,omitempty"`
}

func (UserProfile) TableName() string { return "user_profiles" }

// BeforeCreate 钩子，创建前生成 ID
func (u *UserProfile) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
