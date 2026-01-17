package user

import (
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// RoleAuthority 对应 role_authority 表
type RoleAuthority struct {
	ID          int64 `gorm:"primaryKey;column:id" json:"id"`
	RoleID      int64 `gorm:"not null;column:role_id" json:"role_id"`
	AuthorityID int64 `gorm:"not null;column:authority_id" json:"authority_id"`

	// 关联（可选）
	Role      Role      `gorm:"foreignKey:RoleID;constraint:false" json:"role"`
	Authority Authority `gorm:"foreignKey:AuthorityID;constraint:false" json:"authority"`
}

func (RoleAuthority) TableName() string { return "role_authority" }

// BeforeCreate 钩子，创建前生成 ID
func (u *RoleAuthority) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
