package migrations

import (
	"fmt"

	"github.com/binhy/vistack/models"
	"gorm.io/gorm"
)

// AutoMigrate 执行数据库自动迁移
// 在这里集中维护所有模型的迁移顺序
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}

	// 依次迁移模型（根据外键或依赖顺序排列）
	return db.AutoMigrate(
		&models.User{},
	)
}
