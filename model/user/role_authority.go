package user

import (
    "github.com/google/uuid"
)

// RoleAuthority 对应 role_authority 表
type RoleAuthority struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    RoleID      int64     `gorm:"not null;column:role_id" json:"role_id"`
    AuthorityID int64     `gorm:"not null;column:authority_id" json:"authority_id"`

    // 关联（可选）
    Role      Role      `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"role"`
    Authority Authority `gorm:"foreignKey:AuthorityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"authority"`
}

func (RoleAuthority) TableName() string { return "role_authority" }