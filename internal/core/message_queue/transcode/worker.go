package transcode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type TranscodeMessage struct {
	VideoID     int64  `json:"video_id"`
	TranscodeID int64  `json:"transcode_id"`
	ObjectKey   string `json:"object_key"`
	Attempt     int    `json:"attempt"`
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

	// 2. 下载原始视频到临时文件
	bucket := global.AppConfig.MinIO.Bucket
	tempDir := filepath.Join("temp", string(consts.KafkaTopicTranscode), fmt.Sprintf("%d", msg.VideoID))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.mp4")
	if err := core.Minio.FGetObject(ctx, bucket, msg.ObjectKey, inputPath, minio.GetObjectOptions{}); err != nil {
		core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Update("status", mVideo.TranscodeStatusFailed)
		attemptsKey := fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID)
		cnt, _ := core.Redis.Incr(ctx, attemptsKey).Result()
		core.Redis.Expire(ctx, attemptsKey, 24*time.Hour)
		if cnt <= 7 {
			_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: msg.VideoID, TranscodeID: msg.TranscodeID, ObjectKey: msg.ObjectKey, Attempt: int(cnt)})
		}
		return nil
	}

	// 2.1 获取视频时长
	durationSec, err := GetVideoDuration(inputPath)
	if err != nil {
		core.Logger.Warn("get video duration failed", zap.Error(err))
		// 不影响主流程，仅记录警告
	}

	var coverObjectKey string
	var coverSize int64
	snapshotTime := 1.0
	if durationSec > 10 {
		snapshotTime = 5
	} else if durationSec > 2 {
		snapshotTime = durationSec / 2
	}
	coverPath := filepath.Join(tempDir, "cover.jpg")
	if err := ExtractVideoFrame(inputPath, coverPath, snapshotTime); err != nil {
		core.Logger.Warn("extract cover frame failed", zap.Error(err))
	} else {
		info, statErr := os.Stat(coverPath)
		if statErr != nil {
			core.Logger.Warn("stat cover file failed", zap.Error(statErr))
		} else {
			coverObjectKey = fmt.Sprintf("covers/%d.jpg", msg.VideoID)
			_, putErr := core.Minio.FPutObject(ctx, bucket, coverObjectKey, coverPath, minio.PutObjectOptions{
				ContentType: "image/jpeg",
			})
			if putErr != nil {
				core.Logger.Warn("upload cover file failed", zap.Error(putErr))
			} else {
				coverSize = info.Size()
			}
		}
	}

	// 3. 执行转码 (DASH)
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	qualities, err := TranscodeToDASH(inputPath, outputDir)
	if err != nil {
		core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Update("status", mVideo.TranscodeStatusFailed)
		attemptsKey := fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID)
		cnt, _ := core.Redis.Incr(ctx, attemptsKey).Result()
		core.Redis.Expire(ctx, attemptsKey, 24*time.Hour)
		if cnt <= 7 {
			_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: msg.VideoID, TranscodeID: msg.TranscodeID, ObjectKey: msg.ObjectKey, Attempt: int(cnt)})
		}
		return nil
	}

	// 4. 上传结果到 MinIO (dash/{video_id}/)
	storageKeyPrefix := fmt.Sprintf("dash/%d", msg.VideoID)
	var manifestObjectKey string
	var manifestSize int64
	var mu sync.Mutex

	// 收集所有需要上传的文件
	var filesToUpload []string
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			filesToUpload = append(filesToUpload, path)
		}
		return nil
	})
	if err != nil {
		core.Logger.Error("walk output dir failed", zap.Error(err))
		return err
	}

	// 使用 errgroup 并发上传
	var g errgroup.Group
	g.SetLimit(10) // 限制最大并发数为 10

	for _, path := range filesToUpload {
		filePath := path // 闭包捕获
		g.Go(func() error {
			relPath, _ := filepath.Rel(outputDir, filePath)
			relPath = filepath.ToSlash(relPath)
			objectName := fmt.Sprintf("%s/%s", storageKeyPrefix, relPath)

			// 显式设置 MIME 类型
			contentType := "application/octet-stream"
			ext := filepath.Ext(filePath)
			switch ext {
			case ".mpd":
				contentType = "application/dash+xml"
			case ".m4s":
				contentType = "video/iso.segment"
			case ".mp4":
				contentType = "video/mp4"
			}

			// Upload
			_, err := core.Minio.FPutObject(ctx, bucket, objectName, filePath, minio.PutObjectOptions{
				ContentType: contentType,
			})
			if err != nil {
				return err
			}

			if ext == ".mpd" {
				// 获取文件大小
				if info, err := os.Stat(filePath); err == nil {
					mu.Lock()
					manifestObjectKey = objectName
					manifestSize = info.Size()
					mu.Unlock()
				}
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Update("status", mVideo.TranscodeStatusFailed)
		attemptsKey := fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID)
		cnt, _ := core.Redis.Incr(ctx, attemptsKey).Result()
		core.Redis.Expire(ctx, attemptsKey, 24*time.Hour)
		if cnt <= 7 {
			_ = AddTranscodeRetry(ctx, TranscodeRetryMessage{VideoID: msg.VideoID, TranscodeID: msg.TranscodeID, ObjectKey: msg.ObjectKey, Attempt: int(cnt)})
		}
		return nil
	}

	// 5. 更新数据库
	tx := core.DB.Begin()

	// 5.1 创建 Manifest File 记录
	manifestFile := mFile.File{
		Bucket:    bucket,
		ObjectKey: manifestObjectKey,
		Status:    mFile.FileStatusActive,
		RefType:   mFile.FileRefTypeVideoManifest,
		MimeType:  "application/dash+xml",
		Size:      manifestSize,
	}
	if err := tx.Create(&manifestFile).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create manifest file record failed", zap.Error(err))
		return err
	}

	var resolutions []string
	for _, q := range qualities {
		resolutions = append(resolutions, fmt.Sprintf("%dp", q.Height))
	}
	resolution := strings.Join(resolutions, ",")
	codec := "h264,aac"
	updates := map[string]interface{}{
		"status":           mVideo.TranscodeStatusCompleted,
		"manifest_file_id": manifestFile.ID,
		"resolution":       resolution,
		"codec":            codec,
		"updated_at":       time.Now(),
	}
	if err := tx.Model(&mVideo.VideoTranscode{}).Where("id = ?", msg.TranscodeID).Updates(updates).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("update transcode record failed", zap.Error(err))
		return err
	}

	var profileItems []map[string]string
	for _, q := range qualities {
		profileItems = append(profileItems, map[string]string{
			"resolution": fmt.Sprintf("%dp", q.Height),
		})
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
		core.Logger.Error("create manifest record failed", zap.Error(err))
		return err
	}

	var coverFile *mFile.File
	if coverObjectKey != "" && coverSize > 0 {
		cf := mFile.File{
			Bucket:    bucket,
			ObjectKey: coverObjectKey,
			Status:    mFile.FileStatusActive,
			RefType:   mFile.FileRefTypeVideoCover,
			MimeType:  "image/jpeg",
			Size:      coverSize,
		}
		if err := tx.Create(&cf).Error; err != nil {
			tx.Rollback()
			core.Logger.Error("create cover file record failed", zap.Error(err))
			return err
		}
		coverFile = &cf
	}

	// 5.4 更新 Video 状态为 published (或者保持 ready 等待审核，这里设为 published)
	videoUpdates := map[string]interface{}{
		"status": mVideo.VideoStatusPublished,
	}
	if durationSec > 0 {
		videoUpdates["duration"] = int(durationSec)
	}
	if coverFile != nil {
		videoUpdates["cover_file_id"] = coverFile.ID
	}

	if err := tx.Model(&mVideo.Video{}).Where("id = ?", msg.VideoID).Updates(videoUpdates).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("update video status failed", zap.Error(err))
		return err
	}

	tx.Commit()
	core.Redis.Del(ctx, fmt.Sprintf("attempts:transcode:%d", msg.TranscodeID))
	return nil
}
