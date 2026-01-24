package video

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type VideoDeleteMessage struct {
	VideoID int64 `json:"video_id"`
}

func StartVideoDeleteWorker(ctx context.Context) {
	core.StartKafkaConsumer(ctx, string(consts.KafkaTopicDeleteFile), handleVideoDeleteMessage)
}

func handleVideoDeleteMessage(ctx context.Context, key, value []byte) error {
	start := time.Now()

	core.Logger.Info("▶️ handleVideoDeleteMessage start",
		zap.ByteString("key", key),
		zap.ByteString("raw_value", value),
	)

	// 解析消息
	var msg VideoDeleteMessage
	if err := json.Unmarshal(value, &msg); err != nil {
		core.Logger.Error("❌ unmarshal delete message failed", zap.Error(err))
		return err
	}

	core.Logger.Info("📩 onMessage delete video",
		zap.Int64("video_id", msg.VideoID),
	)

	// 加载视频
	var video mVideo.Video
	if err := core.DB.First(&video, msg.VideoID).Error; err != nil {
		core.Logger.Warn("⚠️ video not found, skip",
			zap.Int64("video_id", msg.VideoID),
			zap.Error(err),
		)
		return nil
	}

	core.Logger.Info("🎞️ video loaded",
		zap.Int64("video_id", video.ID),
		zap.String("status", string(video.Status)),
	)

	// ---------------- TX1: 软删除 & 减少 ref_count ----------------
	tx := core.DB.Begin()
	if tx.Error != nil {
		core.Logger.Error("❌ begin tx failed", zap.Error(tx.Error))
		return tx.Error
	}

	// 软删除视频
	if err := tx.Model(&video).Update("status", mVideo.VideoStatusDeleted).Error; err != nil {
		core.Logger.Error("❌ update video status failed",
			zap.Int64("video_id", video.ID),
			zap.Error(err),
		)
		tx.Rollback()
		return err
	}

	// 加载视频源
	var sources []mVideo.VideoSource
	if err := tx.Where("video_id = ?", video.ID).Find(&sources).Error; err != nil {
		core.Logger.Error("❌ find video sources failed",
			zap.Int64("video_id", video.ID),
			zap.Error(err),
		)
		tx.Rollback()
		return err
	}

	core.Logger.Info("📦 video sources loaded",
		zap.Int64("video_id", video.ID),
		zap.Int("source_count", len(sources)),
	)

	// 处理文件 ref_count 并记录需要物理删除的 file_id
	fileIDsToDelete := make([]int64, 0)
	filesToDelete := make([]mFile.File, 0)
	allZero := true

	processFile := func(fileID int64) error {
		if err := tx.Model(&mFile.File{}).
			Where("id = ?", fileID).
			UpdateColumn("ref_count", gorm.Expr("ref_count - 1")).Error; err != nil {
			return err
		}

		var f mFile.File
		if err := tx.First(&f, fileID).Error; err != nil {
			return err
		}

		if f.RefCount <= 0 {
			if err := tx.Model(&f).Update("status", mFile.FileStatusDeleting).Error; err != nil {
				return err
			}
			fileIDsToDelete = append(fileIDsToDelete, f.ID)
			filesToDelete = append(filesToDelete, f)
		} else {
			allZero = false
		}
		return nil
	}

	// 视频源文件
	for _, s := range sources {
		if err := processFile(s.FileID); err != nil {
			core.Logger.Error("❌ process source file failed",
				zap.Int64("file_id", s.FileID),
				zap.Error(err),
			)
			tx.Rollback()
			return err
		}
	}

	// 封面文件
	if video.CoverFileID != nil && *video.CoverFileID != 0 {
		if err := processFile(*video.CoverFileID); err != nil {
			core.Logger.Error("❌ process cover file failed",
				zap.Int64("file_id", *video.CoverFileID),
				zap.Error(err),
			)
			tx.Rollback()
			return err
		}
	}

	// 清单文件 VideoManifest 关联文件
	var manifests []mVideo.VideoManifest
	if err := tx.Where("video_id = ?", video.ID).Find(&manifests).Error; err != nil {
		core.Logger.Error("❌ find manifests failed",
			zap.Int64("video_id", video.ID),
			zap.Error(err),
		)
		tx.Rollback()
		return err
	}

	for _, mf := range manifests {
		if err := processFile(mf.FileID); err != nil {
			core.Logger.Error("❌ process manifest file failed",
				zap.Int64("file_id", mf.FileID),
				zap.Error(err),
			)
			tx.Rollback()
			return err
		}
	}

	if err := tx.Commit().Error; err != nil {
		core.Logger.Error("❌ tx1 commit failed", zap.Error(err))
		return err
	}

	core.Logger.Info("✅ tx1 committed",
		zap.Int64("video_id", video.ID),
		zap.Bool("all_zero", allZero),
	)

	// ---------------- MinIO + TX2: 物理删除 ----------------
	if !allZero {
		core.Logger.Info("⏭️ skip physical delete, ref_count not zero",
			zap.Int64("video_id", video.ID),
		)
		return nil
	}

	// TX2: 删除数据库记录
	tx2 := core.DB.Begin()
	if tx2.Error != nil {
		core.Logger.Error("❌ begin tx2 failed", zap.Error(tx2.Error))
		return tx2.Error
	}

	// 先删依赖 files 的业务表，避免外键约束
	tx2.Where("video_id = ?", video.ID).Delete(&mVideo.VideoTranscode{})
	tx2.Where("video_id = ?", video.ID).Delete(&mVideo.VideoManifest{})
	tx2.Where("video_id = ?", video.ID).Delete(&mVideo.VideoSource{})
	// 删除 video 行，解除对 cover_file_id 的外键引用
	tx2.Delete(&video)
	// 最后删除 files（仅删除 ref_count<=0 的）
	if len(fileIDsToDelete) > 0 {
		tx2.Where("id IN ? AND ref_count <= 0", fileIDsToDelete).Delete(&mFile.File{})
	}
	if err := tx2.Commit().Error; err != nil {
		core.Logger.Error("❌ tx2 commit failed", zap.Error(err))
		return err
	}

	// 成功提交后再删除 MinIO 对象，避免 DB 回滚导致只删存储
	for _, f := range filesToDelete {
		if err := core.Minio.RemoveObject(ctx, f.Bucket, f.ObjectKey, minio.RemoveObjectOptions{}); err != nil {
			core.Logger.Error("❌ remove object failed",
				zap.Int64("file_id", f.ID),
				zap.Error(err),
			)
		} else {
			core.Logger.Info("🗑️ object removed", zap.Int64("file_id", f.ID))
		}
	}

	// 删除 DASH 分片目录
	dashPrefix := fmt.Sprintf("dash/%d/", video.ID)
	bucket := global.AppConfig.MinIO.Bucket
	core.Logger.Info("🗑️ remove dash segment objects",
		zap.String("bucket", bucket),
		zap.String("prefix", dashPrefix),
	)
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range core.Minio.ListObjects(ctx, bucket, minio.ListObjectsOptions{
			Prefix:    dashPrefix,
			Recursive: true,
		}) {
			if obj.Err != nil {
				core.Logger.Error("❌ list dash object error",
					zap.String("prefix", dashPrefix),
					zap.Error(obj.Err),
				)
				continue
			}
			objectsCh <- obj
		}
	}()
	for err := range core.Minio.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		core.Logger.Error("❌ remove dash object failed",
			zap.String("object", err.ObjectName),
			zap.Error(err.Err),
		)
	}

	core.Logger.Info("✅ video delete done",
		zap.Int64("video_id", video.ID),
		zap.Duration("cost", time.Since(start)),
	)

	return nil
}
