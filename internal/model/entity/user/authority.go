package user

import (
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// Authority 对应 authorities 表
type Authority struct {
	ID             int64  `gorm:"primaryKey;column:id" json:"id"`
	ResourceMethod string `gorm:"size:10;column:resource_method" json:"resource_method"`
	ResourceURI    string `gorm:"size:255;column:resource_uri" json:"resource_uri"`
}

func (Authority) TableName() string { return "authorities" }

// BeforeCreate 钩子，创建前生成 ID
func (u *Authority) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
