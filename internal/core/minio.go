package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/binhy/vistack/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"go.uber.org/zap"
)

var Minio *minio.Client
var MinioCore *minio.Core
var MinioCorePublic *minio.Core
var MinioPublicEndpoint string

// InitMinioClient 初始化 MinIO 客户端
func InitMinioClient(cfg *config.AppConfig) {
	minioConfig := cfg.MinIO
	fmt.Printf("OnInitMinioClient, minioConfig: %+v\n", minioConfig)
	if minioConfig.Endpoint == "" {
		if Logger != nil {
			Logger.Warn("MinIO endpoint is not configured, skipping initialization")
		}
		return
	}

	MinioPublicEndpoint = minioConfig.PublicEndpoint

	// Initialize MinIO client object.
	client, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: minioConfig.Secure,
	})
	fmt.Printf("OnInitMinioClient, minioConfig.Endpoint: %s\n", client.EndpointURL())
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to initialize MinIO client", zap.Error(err))
		}
		return
	}

	Minio = client

	// Initialize MinIO Core client for low-level access (Multipart Upload)
	coreClient, err := minio.NewCore(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: minioConfig.Secure,
	})
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to initialize MinIO Core client", zap.Error(err))
		}
		return
	}
	MinioCore = coreClient

	// Initialize MinIO Public Core client for Presigned URLs
	publicEndpoint := minioConfig.Endpoint
	if minioConfig.PublicEndpoint != "" {
		publicEndpoint = minioConfig.PublicEndpoint
	}
	publicCoreClient, err := minio.NewCore(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: minioConfig.Secure,
	})
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to initialize MinIO Public Core client", zap.Error(err))
		}
		// Fallback to internal core
		MinioCorePublic = coreClient
	} else {
		MinioCorePublic = publicCoreClient
	}

	if Logger != nil {
		Logger.Info("MinIO client initialized successfully")
	}

	// Check if bucket exists
	if minioConfig.Bucket != "" {
		ctx := context.Background()
		exists, err := Minio.BucketExists(ctx, minioConfig.Bucket)
		if err != nil {
			if Logger != nil {
				Logger.Error("Failed to check if bucket exists", zap.Error(err))
			}
			return
		}

		if !exists {
			err = Minio.MakeBucket(ctx, minioConfig.Bucket, minio.MakeBucketOptions{})
			if err != nil {
				if Logger != nil {
					Logger.Error("Failed to create bucket", zap.String("bucket", minioConfig.Bucket), zap.Error(err))
				}
				return
			}
			if Logger != nil {
				Logger.Info("Bucket created successfully", zap.String("bucket", minioConfig.Bucket))
			}
		}

		// Set bucket policy: only avatars/ prefix is public
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Principal": "*",
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/avatars/*","arn:aws:s3:::%s/covers/*"]
				}
			]
		}`, minioConfig.Bucket, minioConfig.Bucket)

		if err := Minio.SetBucketPolicy(ctx, minioConfig.Bucket, policy); err != nil {
			if Logger != nil {
				Logger.Error("Failed to set bucket policy", zap.Error(err))
			}
		} else {
			if Logger != nil {
				Logger.Info("Bucket policy set to public read-only", zap.String("bucket", minioConfig.Bucket))
			}
		}

		// Set bucket lifecycle
		lifecycleConfig := lifecycle.NewConfiguration()
		rule := lifecycle.Rule{
			ID:     "expire-replaced-objects",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: 1,
			},
			RuleFilter: lifecycle.Filter{
				Tag: lifecycle.Tag{
					Key:   "status",
					Value: "replaced",
				},
			},
		}
		lifecycleConfig.Rules = []lifecycle.Rule{rule}

		if err := Minio.SetBucketLifecycle(ctx, minioConfig.Bucket, lifecycleConfig); err != nil {
			if Logger != nil {
				Logger.Error("Failed to set bucket lifecycle", zap.Error(err))
			}
		} else {
			if Logger != nil {
				Logger.Info("Bucket lifecycle configured successfully")
			}
		}
	}
}

func GetMinioObjectPublicURL(bucket, objectKey string) string {
	baseURL := GetPublicBaseURL()
	return fmt.Sprintf("%s/%s/%s", baseURL, bucket, objectKey)
}

// GetPublicBaseURL 获取 MinIO 公网基础 URL
func GetPublicBaseURL() string {
	if MinioPublicEndpoint != "" {
		if strings.Contains(MinioPublicEndpoint, "://") {
			return MinioPublicEndpoint
		}
		return fmt.Sprintf("http://%s", MinioPublicEndpoint)
	}

	if Minio == nil {
		return ""
	}
	// Minio.EndpointURL() returns a copy of the URL structure
	u := Minio.EndpointURL()
	return u.String()
}
