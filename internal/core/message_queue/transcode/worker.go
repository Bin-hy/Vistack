package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/internal/transcoder"
	transcoderpb "github.com/binhy/vistack/internal/transcoder/pb/transcoder/v1"
	"go.uber.org/zap"
)

const transcodeCallTimeout = 25 * time.Minute

type TranscodeMessage struct {
	VideoID     int64  `json:"video_id"`
	TranscodeID int64  `json:"transcode_id"`
	ObjectKey   string `json:"object_key"`
	Attempt     int    `json:"attempt"`
}

var transcoderClient *transcoder.Client

// SetTranscoderClient 注入 transcoder gRPC 客户端（worker 角色启动时调用）
func SetTranscoderClient(c *transcoder.Client) {
	transcoderClient = c
}

// StartTranscodeWorker 启动转码消费者
func StartTranscodeWorker(ctx context.Context) {
	core.StartKafkaConsumer(ctx, string(consts.KafkaTopicTranscode), handleTranscodeMessage)
}

func handleTranscodeMessage(ctx context.Context, key, value []byte) error {
	var msg TranscodeMessage
	if err := json.Unmarshal(value, &msg); err != nil {
		return fmt.Errorf("unmarshal msg failed: %w", err)
	}

	core.Logger.Info("Processing transcode task", zap.Int64("video_id", msg.VideoID))

	// 幂等：已完成则跳过
	var tc mVideo.VideoTranscode
	if err := core.DB.First(&tc, msg.TranscodeID).Error; err == nil {
		if tc.Status == mVideo.TranscodeStatusCompleted {
			return nil
		}
	}

	leaseKey := fmt.Sprintf("lease:transcode:%d", msg.TranscodeID)
	ok, _ := core.Redis.SetNX(ctx, leaseKey, "1", 30*time.Minute).Result()
	if !ok {
		return nil
	}
	defer core.Redis.Del(ctx, leaseKey)

	if err := core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Update("status", mVideo.TranscodeStatusProcessing).Error; err != nil {
		return err
	}

	if transcoderClient == nil {
		return markFailed(ctx, msg, fmt.Errorf("transcoder client not initialized"))
	}

	req := &transcoderpb.ProcessVideoRequest{
		Bucket:           global.AppConfig.MinIO.Bucket,
		ObjectKey:        msg.ObjectKey,
		OutputPrefix:     fmt.Sprintf("dash/%d", msg.VideoID),
		CoverObjectKey:   fmt.Sprintf("covers/%d.jpg", msg.VideoID),
		CoverTimeSeconds: 0,
	}

	callCtx, cancel := context.WithTimeout(ctx, transcodeCallTimeout)
	defer cancel()

	resp, err := transcoderClient.ProcessVideo(callCtx, req)
	if err != nil {
		return markFailed(ctx, msg, err)
	}

	return persistTranscodeResult(ctx, msg, resp)
}

// markFailed 标记失败并按指数退避重试；返回 nil 避免 Kafka 重复投递
func markFailed(ctx context.Context, msg TranscodeMessage, cause error) error {
	core.Logger.Error("transcode failed", zap.Int64("video_id", msg.VideoID), zap.Error(cause))
	_ = core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Update("status", mVideo.TranscodeStatusFailed)

	attemptsKey := fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID)
	cnt, _ := core.Redis.Incr(ctx, attemptsKey).Result()
	core.Redis.Expire(ctx, attemptsKey, 24*time.Hour)
	if cnt <= 7 {
		_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: msg.VideoID, TranscodeID: msg.TranscodeID, ObjectKey: msg.ObjectKey, Attempt: int(cnt)})
	}
	return nil
}

// persistTranscodeResult 将 transcoder 返回的结果写入数据库
func persistTranscodeResult(ctx context.Context, msg TranscodeMessage, resp *transcoderpb.ProcessVideoResponse) error {
	bucket := global.AppConfig.MinIO.Bucket
	tx := core.DB.Begin()

	manifestFile := mFile.File{
		Bucket:    bucket,
		ObjectKey: resp.GetManifestObjectKey(),
		Status:    mFile.FileStatusActive,
		RefType:   mFile.FileRefTypeVideoManifest,
		MimeType:  "application/dash+xml",
		Size:      resp.GetManifestSize(),
	}
	if err := tx.Create(&manifestFile).Error; err != nil {
		tx.Rollback()
		return err
	}

	var resolutions []string
	profileItems := make([]map[string]string, 0, len(resp.GetProfiles()))
	for _, p := range resp.GetProfiles() {
		resolutions = append(resolutions, p.GetResolution())
		profileItems = append(profileItems, map[string]string{"resolution": p.GetResolution()})
	}

	updates := map[string]interface{}{
		"status":           mVideo.TranscodeStatusCompleted,
		"manifest_file_id": manifestFile.ID,
		"resolution":       strings.Join(resolutions, ","),
		"codec":            "h264,aac",
		"updated_at":       time.Now(),
	}
	if err := tx.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	profilesJSON, _ := json.Marshal(profileItems)
	manifest := mVideo.VideoManifest{
		VideoID:  msg.VideoID,
		Protocol: "dash",
		FileID:   manifestFile.ID,
		Profiles: string(profilesJSON),
		Status:   mVideo.ManifestStatusReady,
	}
	if err := tx.Create(&manifest).Error; err != nil {
		tx.Rollback()
		return err
	}

	var coverFile *mFile.File
	if resp.GetCoverObjectKey() != "" && resp.GetCoverSize() > 0 {
		cf := mFile.File{
			Bucket:    bucket,
			ObjectKey: resp.GetCoverObjectKey(),
			Status:    mFile.FileStatusActive,
			RefType:   mFile.FileRefTypeVideoCover,
			MimeType:  "image/jpeg",
			Size:      resp.GetCoverSize(),
		}
		if err := tx.Create(&cf).Error; err != nil {
			tx.Rollback()
			return err
		}
		coverFile = &cf
	}

	videoUpdates := map[string]interface{}{
		"status": mVideo.VideoStatusPublished,
	}
	if resp.GetDurationSeconds() > 0 {
		videoUpdates["duration"] = int(resp.GetDurationSeconds())
	}
	if coverFile != nil {
		videoUpdates["cover_file_id"] = coverFile.ID
	}
	if err := tx.Model(&mVideo.Video{}).Where("id = ?", msg.VideoID).Updates(videoUpdates).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	core.Redis.Del(ctx, fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID))
	return nil
}
