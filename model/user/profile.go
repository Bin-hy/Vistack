package user

// UserProfile 对应 user_profiles 表
type UserProfile struct {
    ID        int64   `gorm:"primaryKey;column:id" json:"id"`
    UserID    int64   `gorm:"not null;column:user_id" json:"user_id"`
    Nickname  *string `gorm:"size:100;column:nickname" json:"nickname,omitempty"`
    AvatarURL *string `gorm:"type:text;column:avatar_url" json:"avatar_url,omitempty"`

    // 关联
    User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"user,omitempty"`
}

func (UserProfile) TableName() string { return "user_profiles" }
