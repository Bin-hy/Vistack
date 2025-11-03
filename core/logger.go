package core

import (
    "strings"

    "github.com/binhy/vistack/config"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger 初始化 zap 日志
func InitLogger(cfg *config.AppConfig) {
    level := strings.ToLower(cfg.Logging.Level)
    var zapLevel zapcore.Level
    switch level {
    case "debug":
        zapLevel = zapcore.DebugLevel
    case "warn":
        zapLevel = zapcore.WarnLevel
    case "error":
        zapLevel = zapcore.ErrorLevel
    default:
        zapLevel = zapcore.InfoLevel
    }

    zapCfg := zap.NewProductionConfig()
    zapCfg.Level = zap.NewAtomicLevelAt(zapLevel)
    zapCfg.EncoderConfig.TimeKey = "time"
    zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

    l, err := zapCfg.Build()
    if err != nil {
        panic(err)
    }
    Logger = l
}

func SyncLogger() {
    if Logger != nil {
        _ = Logger.Sync()
    }
}