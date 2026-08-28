package danmaku

import (
	"context"
	"encoding/json"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	entity "github.com/binhy/vistack/internal/model/entity/danmaku"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

// StartDanmakuWorker 启动弹幕落库消费者（按弹幕 ID 幂等，重复消费不重复落库）。
func StartDanmakuWorker(ctx context.Context) {
	if err := core.EnsureTopic(string(consts.KafkaTopicDanmaku)); err != nil && core.Logger != nil {
		core.Logger.Error("ensure danmaku topic failed", zap.Error(err))
	}
	core.StartKafkaConsumer(ctx, string(consts.KafkaTopicDanmaku), func(ctx context.Context, key, value []byte) error {
		var d entity.Danmaku
		if err := json.Unmarshal(value, &d); err != nil {
			return err
		}
		return core.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&d).Error
	})
}
