package transcode

import (
	"context"
	"fmt"
	"time"

	"github.com/binhy/vistack/internal/core"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
)

// StartTranscodeWatchdog
func StartTranscodeWatchdog(ctx context.Context) {
	t := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				var list []mVideo.VideoTranscode
				threshold := time.Now().Add(-15 * time.Minute)
				// 状态出于 Processing 超过15分钟的了
				if err := core.DB.Where("status = ? AND updated_at < ?", mVideo.TranscodeStatusProcessing, threshold).Find(&list).Error; err != nil {
					continue
				}
				// CREATE INDEX idx_transcode_status_update_at ON video_transcodes (status, updated_at, video_id)

				for _, tc := range list {
					leaseKey := fmt.Sprintf("lease:transcode:%d", tc.ID)
					if _, err := core.Redis.Get(ctx, leaseKey).Result(); err == nil {
						continue
					}
					var src mVideo.VideoSource
					if err := core.DB.Where("video_id = ?", tc.VideoID).Order("uploaded_at ASC").First(&src).Error; err != nil {
						continue
					}
					var f mFile.File
					if err := core.DB.First(&f, src.FileID).Error; err != nil {
						continue
					}
					attemptsKey := fmt.Sprintf("attempts:transcode:%d", tc.ID)
					cnt, _ := core.Redis.Incr(ctx, attemptsKey).Result()
					core.Redis.Expire(ctx, attemptsKey, 24*time.Hour)
					if cnt > 7 {
						continue // 重试 cnt 次后 抛弃该任务
					}
					_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: tc.VideoID, TranscodeID: tc.ID, ObjectKey: f.ObjectKey, Attempt: int(cnt)})
				}

				// pending 超时兜底：消息丢失导致任务未进入 processing
				var pendingList []mVideo.VideoTranscode
				pendingThreshold := time.Now().Add(-10 * time.Minute)
				if err := core.DB.Where("status = ? AND updated_at < ?", mVideo.TranscodeStatusPending, pendingThreshold).Find(&pendingList).Error; err == nil {
					for _, tc := range pendingList {
						var src mVideo.VideoSource
						if err := core.DB.Where("video_id = ?", tc.VideoID).Order("uploaded_at ASC").First(&src).Error; err != nil {
							continue
						}
						var f mFile.File
						if err := core.DB.First(&f, src.FileID).Error; err != nil {
							continue
						}
						// 触碰 updated_at 避免下个周期重复投递
						_ = core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", tc.ID).Update("updated_at", time.Now())
						_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: tc.VideoID, TranscodeID: tc.ID, ObjectKey: f.ObjectKey, Attempt: 1})
					}
				}
			}
		}
	}()
}
