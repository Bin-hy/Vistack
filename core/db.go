package core

import (
	"fmt"
	"net/url"

	"github.com/binhy/vistack/config"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB 初始化数据库连接（PostgreSQL）
func InitDB(cfg *config.AppConfig) {
	dsn := cfg.Database.DSN
	// 若未提供 dsn，则尝试根据 host/port/user/password/name 组装
	if dsn == "" && cfg.Database.Host != "" && cfg.Database.User != "" && cfg.Database.Name != "" {
		pass := url.QueryEscape(cfg.Database.Password)
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			cfg.Database.User,
			pass,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Name,
		)
	}
	if dsn == "" {
		if Logger != nil {
			Logger.Warn("DB DSN is empty, skip DB initialization")
		}
		return
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Errorf("failed to connect database: %w", err))
	}
	// 尝试 ping 底层连接
	if sqlDB, err := db.DB(); err == nil {
		if err := sqlDB.Ping(); err != nil {
			if Logger != nil {
				Logger.Warn("DB ping failed", zap.Error(err))
			}
		}
	}
	DB = db
}
