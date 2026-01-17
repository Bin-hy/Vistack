package storage

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
)

// UploadFile 上传文件到 MinIO
// subDir: 存储子目录 (例如 "avatar", "video")
// 返回: (objectName, fullURL, error)
func UploadFile(ctx context.Context, file *multipart.FileHeader, subDir string) (string, string, error) {
	// 1. 打开文件
	src, err := file.Open()
	if err != nil {
		return "", "", err
	}
	defer src.Close()

	// 2. 生成唯一文件名
	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	objectName := fmt.Sprintf("%s/%s", subDir, newFileName)

	// 3. 获取配置
	bucketName := global.AppConfig.MinIO.Bucket

	// 4. 执行上传
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err = core.Minio.PutObject(ctx, bucketName, objectName, src, file.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", "", err
	}

	// 5. 拼接完整访问 URL
	// 格式: http(s)://endpoint/bucket/objectName
	fullURL := fmt.Sprintf("%s/%s/%s", core.GetPublicBaseURL(), bucketName, objectName)

	return objectName, fullURL, nil
}

// UploadLocalFile 上传本地文件到 MinIO
// filePath: 本地文件路径
// objectName: MinIO 中的对象键 (例如 "videos/123/manifest.mpd")
// contentType: 文件类型
// 返回: (fullURL, error)
func UploadLocalFile(ctx context.Context, filePath, objectName, contentType string) (string, error) {
	bucketName := global.AppConfig.MinIO.Bucket

	_, err := core.Minio.FPutObject(ctx, bucketName, objectName, filePath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", err
	}

	fullURL := fmt.Sprintf("%s/%s/%s", core.GetPublicBaseURL(), bucketName, objectName)

	return fullURL, nil
}

// MarkObjectAsReplaced 标记对象为已替换 (status=replaced)
// MinIO 生命周期规则会自动清理这些对象
func MarkObjectAsReplaced(ctx context.Context, objectName string) error {
	bucketName := global.AppConfig.MinIO.Bucket

	tagMap := map[string]string{
		"status": "replaced",
	}

	ot, err := tags.NewTags(tagMap, true)
	if err != nil {
		return err
	}

	return core.Minio.PutObjectTagging(ctx, bucketName, objectName, ot, minio.PutObjectTaggingOptions{})
}
