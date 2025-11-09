package migrations

import (
	"fmt"

	mAudit "github.com/binhy/vistack/model/audit"
	mSocial "github.com/binhy/vistack/model/social"
	mTag "github.com/binhy/vistack/model/tag"
	mUser "github.com/binhy/vistack/model/user"
	mVideo "github.com/binhy/vistack/model/video"
	"gorm.io/gorm"
)

// AutoMigrate 执行数据库自动迁移
// 在这里集中维护所有模型的迁移顺序
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}

	// 确保 uuid 生成函数可用（PostgreSQL）
	db.Exec("CREATE EXTENSION IF NOT EXISTS pgcrypto;")

	// 依次迁移模型（根据外键或依赖顺序排列）
	return db.AutoMigrate(
		// RBAC
		&mUser.Role{},
		&mUser.User{},
		&mUser.UserProfile{},
		&mUser.Authority{},
		&mUser.RoleAuthority{},
		&mUser.UserAuthority{},

		// 视频主体
		&mVideo.Video{},
		&mVideo.VideoSource{},
		&mVideo.VideoTranscode{},
		&mVideo.VideoSegment{},

		// 标签
		&mTag.Tag{},
		&mTag.VideoTag{},

		// 社交互动
		&mSocial.VideoComment{},
		&mSocial.VideoLike{},
		&mSocial.VideoFavorite{},
		&mSocial.VideoPlayLog{},

		// 审计日志
		&mAudit.AuditLog{},
	)
}
