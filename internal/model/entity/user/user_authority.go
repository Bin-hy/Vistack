package user

import (
	"github.com/binhy/vistack/pkg/snowflake"
	"gorm.io/gorm"
)

// UserAuthority 对应 user_authority 表
type UserAuthority struct {
	ID          int64   `gorm:"primaryKey;column:id" json:"id"`
	UserID      int64   `gorm:"not null;column:user_id" json:"user_id"`
	AuthorityID int64   `gorm:"not null;column:authority_id" json:"authority_id"`
	GrantStatus bool    `gorm:"default:false;column:grand_status" json:"grand_status"`
	Remark      *string `gorm:"type:text;column:remark" json:"remark,omitempty"`

	// 关联（可选）
	User      User      `gorm:"foreignKey:UserID;constraint:false" json:"user"`
	Authority Authority `gorm:"foreignKey:AuthorityID;constraint:false" json:"authority"`
}

func (UserAuthority) TableName() string { return "user_authority" }

// BeforeCreate 钩子，创建前生成 ID
func (u *UserAuthority) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == 0 {
		u.ID = snowflake.GenID()
	}
	return
}
