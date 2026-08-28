package migrations

import (
	"fmt"

	"gorm.io/gorm"

	mDanmaku "github.com/binhy/vistack/internal/model/entity/danmaku"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	mTag "github.com/binhy/vistack/internal/model/entity/tag"
	mUser "github.com/binhy/vistack/internal/model/entity/user"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
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

		// 弹幕
		&mDanmaku.Danmaku{},
		&mDanmaku.SensitiveWord{},

		// 审计日志
		// &mAudit.AuditLog{},
	)

	if err != nil {
		return err
	}

	// 显式调整 video_transcodes.resolution 列长度（PostgreSQL）
	// AutoMigrate 不会自动变更已有列的长度，需手动 ALTER
	if err := db.Exec(`ALTER TABLE video_transcodes ALTER COLUMN resolution TYPE VARCHAR(100);`).Error; err != nil {
		// 忽略错误（例如非 PG 或列已是目标类型），不中断迁移
	}

	// CREATE INDEX idx_transcode_status_update_at ON video_transcodes (status, updated_at, video_id, id)
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_transcode_status_update_at ON video_transcodes (status, updated_at, video_id, id);`).Error; err != nil {
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
