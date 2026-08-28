package core

import (
	"fmt"
	"hash/fnv"
	"os"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/pkg/snowflake"
	"go.uber.org/zap"
)

// InitSnowflake 初始化 snowflake 节点；node_id <= 0 时自动从实例标识派生
func InitSnowflake(cfg *config.AppConfig) {
	nodeID := cfg.Snowflake.NodeID
	if nodeID <= 0 {
		nodeID = deriveNodeID()
	}
	if err := snowflake.Init(nodeID); err != nil {
		// 核心组件初始化失败应立即终止，避免后续产生运行时 Panic
		panic(fmt.Sprintf("init snowflake failed: %v", err))
	}
	if Logger != nil {
		Logger.Info("snowflake initialized", zap.Int64("node_id", nodeID))
	}
}

// deriveNodeID 从实例标识（POD_IP 优先，回退 hostname）哈希派生 0..1023 的节点号
func deriveNodeID() int64 {
	id := os.Getenv("POD_IP")
	if id == "" {
		if h, err := os.Hostname(); err == nil {
			id = h
		}
	}
	if id == "" {
		return 1
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return int64(h.Sum32() % 1024)
}
