package user

import (
	"github.com/google/uuid"
)

// Authority 对应 authorities 表
type Authority struct {
	ID             int64     `gorm:"primaryKey;column:id" json:"id"`
	UUID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();unique;column:uuid" json:"uuid"`
	ResourceMethod string    `gorm:"size:10;column:resource_method" json:"resource_method"`
	ResourceURI    string    `gorm:"size:255;column:resource_uri" json:"resource_uri"`
}

func (Authority) TableName() string { return "authorities" }
