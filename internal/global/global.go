package global

import (
	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/pkg/auth"
)

// 全局值
var (
	AppConfig    config.AppConfig
	TokenManager *auth.TokenManager
)
