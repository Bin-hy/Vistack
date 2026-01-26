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

	// Helper to determine secure setting based on endpoint scheme
	// If endpoint starts with http://, force secure=false
	// If endpoint starts with https://, force secure=true
	// Otherwise use config value
	getSecure := func(endpoint string, defaultSecure bool) (string, bool) {
		ep := endpoint
		secure := defaultSecure
		if strings.HasPrefix(endpoint, "http://") {
			ep = strings.TrimPrefix(endpoint, "http://")
			secure = false
		} else if strings.HasPrefix(endpoint, "https://") {
			ep = strings.TrimPrefix(endpoint, "https://")
			secure = true
		}
		return ep, secure
	}

	// Internal endpoint processing
	internalEndpoint, internalSecure := getSecure(minioConfig.Endpoint, minioConfig.Secure)

	// Initialize MinIO client object.
	client, err := minio.New(internalEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: internalSecure,
	})
	if Logger != nil {
		Logger.Info("MinIO Client Initialized", zap.String("endpoint", client.EndpointURL().String()))
	}
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to initialize MinIO client", zap.Error(err))
		}
		return
	}

	Minio = client

	// Initialize MinIO Core client for low-level access (Multipart Upload)
	coreClient, err := minio.NewCore(internalEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: internalSecure,
	})
	if err != nil {
		if Logger != nil {
			Logger.Error("Failed to initialize MinIO Core client", zap.Error(err))
		}
		return
	}
	MinioCore = coreClient

	// Initialize MinIO Public Core client for Presigned URLs
	rawPublicEndpoint := minioConfig.Endpoint
	if minioConfig.PublicEndpoint != "" {
		rawPublicEndpoint = minioConfig.PublicEndpoint
	}

	publicEndpoint, publicSecure := getSecure(rawPublicEndpoint, minioConfig.Secure)

	publicCoreClient, err := minio.NewCore(publicEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKey, minioConfig.SecretKey, ""),
		Secure: publicSecure,
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
			panic(fmt.Sprintf("Failed to connect to MinIO (BucketExists check failed): %v. Please check your configuration.", err))
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
	// 优先使用配置的 PublicEndpoint
	if MinioPublicEndpoint != "" {
		// 如果配置已经包含协议头（http:// 或 https://），直接返回，不重写
		if strings.Contains(MinioPublicEndpoint, "://") {
			return MinioPublicEndpoint
		}
		// 如果未包含协议头，则根据 MinIO Client 的 Public Core 状态来决定
		// 注意：这里我们使用 MinioCorePublic（它在 InitMinioClient 中已经根据 PublicEndpoint 和 Secure 初始化好了）
		if MinioCorePublic != nil {
			// MinioCorePublic.EndpointURL() 会正确反映 scheme (http/https)
			return MinioCorePublic.EndpointURL().String()
		}
		// Fallback: 如果 MinioCorePublic 未初始化，根据全局配置拼装（默认 http）
		return fmt.Sprintf("http://%s", MinioPublicEndpoint)
	}

	if Minio == nil {
		return ""
	}
	// Minio.EndpointURL() returns a copy of the URL structure
	u := Minio.EndpointURL()
	return u.String()
}

// GetInternalBaseURL 获取 MinIO 内网基础 URL（用于服务端直接连接）
func GetInternalBaseURL() string {
	if Minio == nil {
		return ""
	}
	u := Minio.EndpointURL()
	fmt.Printf("Internal")

	return u.String()
}
