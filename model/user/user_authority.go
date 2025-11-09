package user

import (
    "github.com/google/uuid"
)

// UserAuthority 对应 user_authority 表
type UserAuthority struct {
    ID           int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    UserID       int64     `gorm:"not null;column:user_id" json:"user_id"`
    AuthorityID  int64     `gorm:"not null;column:authority_id" json:"authority_id"`
    GrantStatus  bool      `gorm:"default:false;column:grand_status" json:"grand_status"`
    Remark       *string   `gorm:"type:text;column:remark" json:"remark,omitempty"`

    // 关联（可选）
    User      User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Authority Authority `gorm:"foreignKey:AuthorityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"authority"`
}

func (UserAuthority) TableName() string { return "user_authority" }