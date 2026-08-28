package comment

import (
	"context"
	"encoding/json"

	commentSvc "github.com/binhy/vistack/internal/comment"
	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"go.uber.org/zap"
)

var (
	moderator commentSvc.Moderator = commentSvc.PassthroughModerator{}
	svc       *commentSvc.Service
)

// SetModerator 注入自定义图片审核器（未来接第三方内容安全）。
func SetModerator(m commentSvc.Moderator) { moderator = m }

// StartCommentModerationWorker 启动评论图片审核消费者。
func StartCommentModerationWorker(ctx context.Context) {
	svc = commentSvc.NewService(core.Redis, core.DB, commentSvc.Options{Logger: core.Logger})
	if err := core.EnsureTopic(string(consts.KafkaTopicCommentModeration)); err != nil && core.Logger != nil {
		core.Logger.Error("ensure comment moderation topic failed", zap.Error(err))
	}
	core.StartKafkaConsumer(ctx, string(consts.KafkaTopicCommentModeration), handleModerationMessage)
}

func handleModerationMessage(ctx context.Context, key, value []byte) error {
	var msg commentSvc.ModerationMessage
	if err := json.Unmarshal(value, &msg); err != nil {
		return err
	}
	if svc == nil {
		return nil
	}
	if err := svc.ProcessModeration(ctx, msg.CommentID, moderator); err != nil {
		core.Logger.Error("process comment moderation failed",
			zap.Int64("comment_id", msg.CommentID), zap.Error(err))
		return err
	}
	return nil
}
