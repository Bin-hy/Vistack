package migrations

import (
	"fmt"

	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	mTag "github.com/binhy/vistack/internal/model/entity/tag"
	mUser "github.com/binhy/vistack/internal/model/entity/user"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"gorm.io/gorm"
)

// AutoMigrate 执行数据库自动迁移
// 在这里集中维护所有模型的迁移顺序
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("gorm db is nil")
	}

	// 依次迁移模型（根据外键或依赖顺序排列）
	err := db.AutoMigrate(
		// RBAC
		&mUser.Role{},
		&mUser.User{},
		&mUser.UserProfile{},
		&mUser.Authority{},
		&mUser.RoleAuthority{},
		&mUser.UserAuthority{},

		// 文件
		&mFile.File{},

		// 视频主体
		&mVideo.Video{},
		&mVideo.VideoSource{},
		&mVideo.VideoTranscode{},
		&mVideo.VideoManifest{},

		// 标签
		&mTag.Tag{},
		&mTag.VideoTag{},

		// 社交互动
		&mSocial.VideoComment{},
		&mSocial.VideoLike{},
		&mSocial.VideoFavorite{},
		&mSocial.VideoPlayLog{},

		// 审计日志
		// &mAudit.AuditLog{},
	)

	if err != nil {
		return err
	}

	return initDefaultRoles(db)
}

func initDefaultRoles(db *gorm.DB) error {
	roles := []string{"superadmin", "admin", "user", "vipuser"}
	for _, name := range roles {
		// 使用 FirstOrCreate 保证幂等
		if err := db.Where(mUser.Role{Name: name}).FirstOrCreate(&mUser.Role{Name: name}).Error; err != nil {
			return err
		}
	}
	return nil
}
