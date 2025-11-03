package core

import (
	"fmt"

	"github.com/binhy/vistack/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var Minio *minio.Client

// InitMinio 初始化 MinIO 客户端
func InitMinio(cfg *config.AppConfig) {
	if cfg.MinIO.Endpoint == "" {
		if Logger != nil {
			Logger.Warn("MinIO endpoint is empty, skip MinIO initialization")
		}
		return
	}
	client, err := minio.New(cfg.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinIO.AccessKey, cfg.MinIO.SecretKey, ""),
		Secure: cfg.MinIO.Secure,
	})
	if err != nil {
		panic(fmt.Errorf("failed to init minio: %w", err))
	}
	Minio = client
}
