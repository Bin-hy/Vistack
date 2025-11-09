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

    Users []User `gorm:"foreignKey:RoleID" json:"users,omitempty"`
}

func (Role) TableName() string { return "roles" }

// Authority 对应 authorities 表
type Authority struct {
    ID             int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    ResourceMethod string    `gorm:"size:10;column:resource_method" json:"resource_method"`
    ResourceURI    string    `gorm:"size:255;column:resource_uri" json:"resource_uri"`
}

func (Authority) TableName() string { return "authorities" }

// RoleAuthority 对应 role_authority
type RoleAuthority struct {
    ID          int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    RoleID      int64     `gorm:"not null;column:role_id" json:"role_id"`
    AuthorityID int64     `gorm:"not null;column:authority_id" json:"authority_id"`

    Role      Role      `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"role"`
    Authority Authority `gorm:"foreignKey:AuthorityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"authority"`
}

func (RoleAuthority) TableName() string { return "role_authority" }

// UserAuthority 对应 user_authority
type UserAuthority struct {
    ID           int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    UserID       int64     `gorm:"not null;column:user_id" json:"user_id"`
    AuthorityID  int64     `gorm:"not null;column:authority_id" json:"authority_id"`
    GrantStatus  bool      `gorm:"default:false;column:grant_status" json:"grant_status"`
    Remark       *string   `gorm:"type:text;column:remark" json:"remark,omitempty"`

    User      User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
    Authority Authority `gorm:"foreignKey:AuthorityID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"authority"`
}

func (UserAuthority) TableName() string { return "user_authority" }