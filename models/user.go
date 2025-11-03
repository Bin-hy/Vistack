package models

import "gorm.io/gorm"

// User 简易用户模型（示例）
type User struct {
    gorm.Model
    Account     string `gorm:"uniqueIndex;size:120" json:"account"`
    Nickname    string `gorm:"size:120" json:"nickname,omitempty"`
    PasswordHash string `gorm:"size:255" json:"-"`
}
