package role

import (
	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/migrations"
	"go.uber.org/zap"
)

// RunMigrate 执行数据库迁移后退出（幂等，可重复执行）。
// 用于独立的一次性迁移任务（docker compose run / k8s Job），避免多副本 api 同时迁移产生竞争。
func RunMigrate(cfg *config.AppConfig) {
	core.InitDB(cfg)
	if core.DB == nil {
		panic("database not initialized, migration aborted")
	}
	if err := migrations.AutoMigrate(core.DB); err != nil {
		if core.Logger != nil {
			core.Logger.Error("migration failed", zap.Error(err))
		}
		panic(err)
	}
	if core.Logger != nil {
		core.Logger.Info("migration completed")
	}
}
