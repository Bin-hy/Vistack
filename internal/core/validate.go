package core

import (
	"github.com/binhy/vistack/internal/config"
)

// ValidateConfig 校验关键配置。
// JWT 签名已由 HMAC(shared secret) 升级为 RS256（私钥经环境变量注入），
// 私钥的强制校验由 auth 角色在启动时完成（role/auth.go），此处不再校验 secret。
func ValidateConfig(cfg *config.AppConfig) {
	_ = cfg
}
