package models

import (
    "time"

    "github.com/google/uuid"
)

// User 对应 users 表
type User struct {
    ID           int64     `gorm:"primaryKey;column:id" json:"id"`
    UUID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
    Username     string    `gorm:"size:50;uniqueIndex;column:username" json:"username"`
    Email        *string   `gorm:"size:100;column:email" json:"email,omitempty"`
    PasswordHash string    `gorm:"type:text;not null;column:password_hash" json:"-"`
    RoleID       int64     `gorm:"not null;column:role_id" json:"role_id"`
    Status       string    `gorm:"size:20;default:active;column:status" json:"status"`
    CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`

    // 关联
    Role     Role         `gorm:"foreignKey:RoleID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"role"`
    Profile  UserProfile  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"profile"`
    Videos   []Video      `gorm:"foreignKey:UserID" json:"videos,omitempty"`
}

func (User) TableName() string { return "users" }

// UserProfile 对应 user_profiles 表
type UserProfile struct {
    ID        int64   `gorm:"primaryKey;column:id" json:"id"`
    UserID    int64   `gorm:"not null;uniqueIndex;column:user_id" json:"user_id"`
    Nickname  *string `gorm:"size:100;column:nickname" json:"nickname,omitempty"`
    AvatarURL *string `gorm:"type:text;column:avatar_url" json:"avatar_url,omitempty"`

    // 关联
    User User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user"`
}

func (UserProfile) TableName() string { return "user_profiles" }
