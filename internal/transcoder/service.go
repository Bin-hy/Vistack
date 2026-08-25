package transcoder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/binhy/vistack/internal/core"
	transcoderpb "github.com/binhy/vistack/internal/transcoder/pb/transcoder/v1"
	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
)

// Service 实现 TranscoderService gRPC 服务
type Service struct {
	transcoderpb.UnimplementedTranscoderServiceServer
}

func NewService() *Service {
	return &Service{}
}

// ProcessVideo 下载原始视频 → 探测 → 抽封面 → DASH 转码 → 写回 MinIO
func (s *Service) ProcessVideo(ctx context.Context, req *transcoderpb.ProcessVideoRequest) (*transcoderpb.ProcessVideoResponse, error) {
	if req.GetBucket() == "" || req.GetObjectKey() == "" {
		return nil, fmt.Errorf("bucket and object_key are required")
	}

	tempDir, err := os.MkdirTemp("", "transcode-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.mp4")
	if err := core.Minio.FGetObject(ctx, req.GetBucket(), req.GetObjectKey(), inputPath, minio.GetObjectOptions{}); err != nil {
		return nil, fmt.Errorf("download input failed: %w", err)
	}

	// 探测时长
	durationSec, err := GetVideoDuration(inputPath)
	if err != nil {
		if core.Logger != nil {
			core.Logger.Warn("get video duration failed", zap.Error(err))
		}
		durationSec = 0
	}

	// 抽封面帧
	snapshotTime := req.GetCoverTimeSeconds()
	if snapshotTime <= 0 {
		snapshotTime = 1.0
		if durationSec > 10 {
			snapshotTime = 5
		} else if durationSec > 2 {
			snapshotTime = durationSec / 2
		}
	}

	var coverObjectKey string
	var coverSize int64
	if req.GetCoverObjectKey() != "" {
		coverPath := filepath.Join(tempDir, "cover.jpg")
		if err := ExtractVideoFrame(inputPath, coverPath, snapshotTime); err != nil {
			if core.Logger != nil {
				core.Logger.Warn("extract cover frame failed", zap.Error(err))
			}
		} else if info, statErr := os.Stat(coverPath); statErr == nil {
			if _, putErr := core.Minio.FPutObject(ctx, req.GetBucket(), req.GetCoverObjectKey(), coverPath, minio.PutObjectOptions{ContentType: "image/jpeg"}); putErr != nil {
				if core.Logger != nil {
					core.Logger.Warn("upload cover failed", zap.Error(putErr))
				}
			} else {
				coverObjectKey = req.GetCoverObjectKey()
				coverSize = info.Size()
			}
		}
	}

	// 转码
	outputDir := filepath.Join(tempDir, "output")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir failed: %w", err)
	}

	preferred := ResolveQualities(0, 0, req.GetQualityHeights())
	qualities, err := TranscodeToDASH(inputPath, outputDir, preferred)
	if err != nil {
		return nil, err
	}

	// 上传产物
	var manifestObjectKey string
	var manifestSize int64

	var filesToUpload []string
	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			filesToUpload = append(filesToUpload, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk output dir failed: %w", err)
	}

	for _, fp := range filesToUpload {
		rel, _ := filepath.Rel(outputDir, fp)
		rel = filepath.ToSlash(rel)
		objectName := fmt.Sprintf("%s/%s", req.GetOutputPrefix(), rel)

		contentType := "application/octet-stream"
		switch filepath.Ext(fp) {
		case ".mpd":
			contentType = "application/dash+xml"
		case ".m4s":
			contentType = "video/iso.segment"
		case ".mp4":
			contentType = "video/mp4"
		}

		if _, err := core.Minio.FPutObject(ctx, req.GetBucket(), objectName, fp, minio.PutObjectOptions{ContentType: contentType}); err != nil {
			return nil, fmt.Errorf("upload %s failed: %w", rel, err)
		}

		if filepath.Ext(fp) == ".mpd" {
			if info, statErr := os.Stat(fp); statErr == nil {
				manifestObjectKey = objectName
				manifestSize = info.Size()
			}
		}
	}

	profiles := make([]*transcoderpb.QualityProfile, 0, len(qualities))
	for _, q := range qualities {
		profiles = append(profiles, &transcoderpb.QualityProfile{
			Height:     int32(q.Height),
			Resolution: fmt.Sprintf("%dp", q.Height),
		})
	}

	return &transcoderpb.ProcessVideoResponse{
		DurationSeconds:    durationSec,
		ManifestObjectKey:  manifestObjectKey,
		ManifestSize:       manifestSize,
		CoverObjectKey:     coverObjectKey,
		CoverSize:          coverSize,
		Profiles:           profiles,
	}, nil
}
