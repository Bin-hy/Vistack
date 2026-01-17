package core

import (
	"fmt"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/pkg/snowflake"
	"go.uber.org/zap"
)

// InitSnowflake initializes the snowflake node
func InitSnowflake(cfg *config.AppConfig) {
	nodeID := cfg.Snowflake.NodeID
	if nodeID == 0 {
		nodeID = 1 // default to 1 if not configured
	}
	if err := snowflake.Init(nodeID); err != nil {
		// 核心组件初始化失败应立即终止，避免后续产生运行时 Panic
		panic(fmt.Sprintf("init snowflake failed: %v", err))
	} else {
		if Logger != nil {
			Logger.Info("snowflake initialized", zap.Int64("node_id", nodeID))
		}
	}
}
