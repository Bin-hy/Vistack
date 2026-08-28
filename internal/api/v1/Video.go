package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/consts"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/core/message_queue/transcode"
	mq_video "github.com/binhy/vistack/internal/core/message_queue/video"
	"github.com/binhy/vistack/internal/global"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/binhy/vistack/pkg/timeutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 视频路由
type VideoApi struct{}

type Credentials struct {
	AccessKeyID     string    `json:"accessKey"`
	SecretAccessKey string    `json:"secretKey"`
	SessionToken    string    `json:"sessionToken"`
	Expiration      time.Time `json:"expiration"`
}

type VideoAuthorResponse struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type VideoInfoResponse struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	Description *string              `json:"description,omitempty"`
	CoverURL    string               `json:"cover_url"`
	Duration    int                  `json:"duration"`
	Status      string               `json:"status"`
	Visibility  string               `json:"visibility"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
	User        *VideoAuthorResponse `json:"user,omitempty"`
}

// VideoInitRequest 初始化分片上传请求
type VideoInitRequest struct {
	Filename  string `json:"filename" binding:"required"`
	MimeType  string `json:"mime_type"`
	FileHash  string `json:"file_hash" binding:"required"` // md5 hash值
	FileSize  int64  `json:"file_size"`
	ChunkSize int64  `json:"chunk_size" bingding:"required"` // 分块大小
}

type VideoInitResponse struct {
	UploadID  string `json:"upload_id"`
	ObjectKey string `json:"object_key"`
	Bucket    string `json:"bucket"`
	Uploaded  bool   `json:"uploaded"`           // 是否秒传成功
	VideoID   string `json:"video_id,omitempty"` // 秒传成功时的 VideoID
}

// InitVideoUpload 初始化分片上传
func (v *VideoApi) InitVideoUpload(c *gin.Context) {

	// 1. 获取文件名和类型
	var req VideoInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MimeType == "" {
		req.MimeType = "application/octet-stream"
	}

	userID := auth.GetUserID(c)
	ctx := c.Request.Context()

	// 0. 全局秒传检查 (Global Dedup)
	// 检查数据库中是否存在相同 Hash 且状态为 active 的文件
	var existingFile mFile.File
	if err := core.DB.Where("hash = ? AND status = ?", req.FileHash, mFile.FileStatusActive).First(&existingFile).Error; err == nil {
		// 命中秒传！复用该物理文件，增加引用计数
		tx := core.DB.Begin()

		// 0.1 增加引用计数
		if err := tx.Model(&existingFile).UpdateColumn("ref_count", gorm.Expr("ref_count + ?", 1)).Error; err != nil {
			tx.Rollback()
			core.Logger.Error("dedup update ref_count failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// 0.2 创建 UserVideo
		video := mVideo.Video{
			UserID: userID,
			Title:  req.Filename,
			Status: mVideo.VideoStatusProcessing, // 秒传成功，进入转码流程
		}

		if err := tx.Create(&video).Error; err != nil {
			tx.Rollback()
			core.Logger.Error("dedup create video failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// 0.3 创建 VideoSource 关联到同一个 FileID
		source := mVideo.VideoSource{
			VideoID:    video.ID,
			FileID:     existingFile.ID, // 复用 FileID
			UploadedAt: time.Now(),
		}
		if err := tx.Create(&source).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		// 0.4 创建转码任务 (每个用户视频独立的转码任务)
		transcodeTask := mVideo.VideoTranscode{
			VideoID: video.ID,
			Status:  mVideo.TranscodeStatusPending,
		}
		if err := tx.Create(&transcodeTask).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}

		tx.Commit()

		// 0.5 发送 Kafka 消息
		msg := transcode.TranscodeMessage{
			VideoID:     video.ID,
			TranscodeID: transcodeTask.ID,
			ObjectKey:   existingFile.ObjectKey,
		}
		msgBytes, _ := json.Marshal(msg)
		if err := core.SendKafkaMessage(context.Background(), string(consts.KafkaTopicTranscode), strconv.FormatInt(video.ID, 10), msgBytes); err != nil {
			core.Logger.Error("send kafka message failed", zap.Error(err))
			_ = core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", transcodeTask.ID).Update("status", mVideo.TranscodeStatusFailed)
			_ = transcode.AddTranscodeRetry(context.Background(), transcode.TranscodeRetryMessage{
				VideoID:     video.ID,
				TranscodeID: transcodeTask.ID,
				ObjectKey:   existingFile.ObjectKey,
				Attempt:     1,
			})
			c.JSON(http.StatusInternalServerError, gin.H{"error": "转码任务投递失败，请重试"})
			return
		}

		c.JSON(http.StatusOK, VideoInitResponse{
			Uploaded: true,
			VideoID:  strconv.FormatInt(video.ID, 10),
		})
		return
	}

	// 1. 续传检查 (Resume Check)
	// Check Redis for existing upload session
	// Key: upload_session:<user_id>:<file_hash>
	sessionKey := fmt.Sprintf("upload_session:%d:%s", userID, req.FileHash)

	if val, err := core.Redis.Get(ctx, sessionKey).Result(); err == nil && val != "" {
		var cachedResp VideoInitResponse
		if err := json.Unmarshal([]byte(val), &cachedResp); err == nil {
			// Verify if the upload is still valid in MinIO (optional, but good practice)
			c.JSON(http.StatusOK, cachedResp)
			return
		}
	}

	// 2. 生成 ObjectKey
	ext := filepath.Ext(req.Filename)
	newFileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	objectKey := fmt.Sprintf("raw/%s", newFileName)
	bucket := global.AppConfig.MinIO.Bucket

	// 3. 调用 MinIO Core 初始化 Multipart Upload
	uploadID, err := core.MinioCore.NewMultipartUpload(c.Request.Context(), bucket, objectKey, minio.PutObjectOptions{
		ContentType: req.MimeType,
	})
	if err != nil {
		core.Logger.Error("init multipart upload failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Init upload failed"})
		return
	}

	resp := VideoInitResponse{
		UploadID:  uploadID,
		ObjectKey: objectKey,
		Bucket:    bucket,
		Uploaded:  false,
	}

	// Cache the session in Redis for 24 hours
	if bytes, err := json.Marshal(resp); err == nil {
		core.Redis.Set(ctx, sessionKey, string(bytes), 24*time.Hour)
	}

	c.JSON(http.StatusOK, resp)
}

// GetUploadPartURLRequest 获取分片上传链接请求
type GetUploadPartURLRequest struct {
	UploadID   string `json:"upload_id" form:"upload_id" binding:"required"`
	ObjectKey  string `json:"object_key" form:"object_key" binding:"required"`
	PartNumber int    `json:"partNumber" form:"partNumber" binding:"required"`
}

type GetUploadPartURLResponse struct {
	URL string `json:"url"`
}

// GetUploadPartURL 获取分片上传 Presigned URL
func (v *VideoApi) GetUploadPartURL(c *gin.Context) {
	// 0. 获取参数
	var req GetUploadPartURLRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bucket := global.AppConfig.MinIO.Bucket

	// 1. 生成 Presigned URL
	// 过期时间设置为 1 小时
	expires := time.Hour

	// 注意：MinIO Go SDK 的 PresignedPutObject 通常用于普通 PUT，
	// 对于 Multipart Part Upload，需要特殊处理 query params
	// 但是 MinIO SDK 的 PresignedPutObject 也可以带参数。
	// 更底层的方法是 core.Presign

	// 使用 minio.Core 的 Presign 方法更灵活，或者使用 Client.PresignedUrl
	// 构造参数
	reqParams := make(url.Values)
	reqParams.Set("uploadId", req.UploadID)
	reqParams.Set("partNumber", strconv.Itoa(req.PartNumber))

	// 生成 URL
	u, err := core.MinioCorePublic.Presign(c.Request.Context(), "PUT", bucket, req.ObjectKey, expires, reqParams)
	if err != nil {
		core.Logger.Error("get presigned url failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Get upload url failed"})
		return
	}

	c.JSON(http.StatusOK, GetUploadPartURLResponse{
		URL: u.String(),
	})
}

// ListUploadedPartsRequest 列举已上传分片请求
type ListUploadedPartsRequest struct {
	UploadID  string `json:"upload_id" form:"upload_id" binding:"required"`
	ObjectKey string `json:"object_key" form:"object_key" binding:"required"`
}

type ListUploadedPartsResponse struct {
	Parts []minio.ObjectPart `json:"parts"`
}

// ListUploadedParts 列举已上传分片
func (v *VideoApi) ListUploadedParts(c *gin.Context) {
	var req ListUploadedPartsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bucket := global.AppConfig.MinIO.Bucket

	// 调用 MinIO Core List Object Parts
	// maxParts set to 10000 (max limit for S3)
	result, err := core.MinioCore.ListObjectParts(c.Request.Context(), bucket, req.ObjectKey, req.UploadID, 0, 10000)
	if err != nil {
		core.Logger.Error("list object parts failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "List parts failed"})
		return
	}

	parts := result.ObjectParts
	if parts == nil {
		parts = []minio.ObjectPart{}
	}

	c.JSON(http.StatusOK, ListUploadedPartsResponse{
		Parts: parts,
	})
}

type CompleteVideoUpload struct {
	UploadID  string               `json:"upload_id" binding:"required"`
	ObjectKey string               `json:"object_key" binding:"required"`
	Filename  string               `json:"filename" binding:"required"`
	FileHash  string               `json:"file_hash"`
	Parts     []minio.CompletePart `json:"parts" binding:"required"` // 需要前端传回所有分片的 ETag
}

type CompleteVideoUploadResponse struct {
	Msg     string `json:"msg"`
	VideoID int64  `json:"video_id,string"`
}

// CompleteVideoUpload 完成分片上传
func (v *VideoApi) CompleteVideoUpload(c *gin.Context) {
	// 0. 获取参数
	var req CompleteVideoUpload

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)

	bucket := global.AppConfig.MinIO.Bucket

	// 1. 完成 MinIO Multipart Upload
	_, err := core.MinioCore.CompleteMultipartUpload(c.Request.Context(), bucket, req.ObjectKey, req.UploadID, req.Parts, minio.PutObjectOptions{})
	if err != nil {
		core.Logger.Error("complete multipart upload failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Complete upload failed"})
		return
	}

	// 2. 数据库落库
	tx := core.DB.Begin()

	// 2.1 创建 Video 记录
	video := mVideo.Video{
		UserID: userID,
		Title:  req.Filename,
		Status: mVideo.VideoStatusProcessing, // 标记为处理中
	}
	if err := tx.Create(&video).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create video failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 2.2 创建 File 记录 (Raw Video)
	rawFile := mFile.File{
		Bucket:    bucket,
		ObjectKey: req.ObjectKey,
		Status:    mFile.FileStatusActive,
		RefType:   mFile.FileRefTypeVideoSource,
		MimeType:  "video/mp4", // 暂时假定
		Hash:      req.FileHash,
		RefCount:  1, // 初始引用计数为 1
	}
	if err := tx.Create(&rawFile).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create file failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 2.3 创建 VideoSource 记录
	source := mVideo.VideoSource{
		VideoID:    video.ID,
		FileID:     rawFile.ID,
		UploadedAt: time.Now(),
	}
	if err := tx.Create(&source).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create source failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 2.4 创建 VideoTranscode 任务
	transcodeTask := mVideo.VideoTranscode{
		VideoID: video.ID,
		Status:  mVideo.TranscodeStatusPending,
	}
	if err := tx.Create(&transcodeTask).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create transcode task failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	tx.Commit()

	// 3. 发送 Kafka 消息
	// 消息格式: {"video_id": 1, "transcode_id": 1, "object_key": "raw/xxx.mp4"}
	msg := transcode.TranscodeMessage{
		VideoID:     video.ID,
		TranscodeID: transcodeTask.ID,
		ObjectKey:   req.ObjectKey,
	}
	msgBytes, _ := json.Marshal(msg)

	// 使用 "transcode" 作为 Topic
	if err := core.SendKafkaMessage(context.Background(), string(consts.KafkaTopicTranscode), strconv.FormatInt(video.ID, 10), msgBytes); err != nil {
		// 投递失败：标记转码失败并进入重试队列，避免任务永久 pending
		core.Logger.Error("send kafka message failed", zap.Error(err))
		_ = core.DB.Model(&mVideo.VideoTranscode{}).Where("id = ?", transcodeTask.ID).Update("status", mVideo.TranscodeStatusFailed)
		_ = transcode.AddTranscodeRetry(context.Background(), transcode.TranscodeRetryMessage{
			VideoID:     video.ID,
			TranscodeID: transcodeTask.ID,
			ObjectKey:   req.ObjectKey,
			Attempt:     1,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "转码任务投递失败，请重试"})
		return
	}

	// Clear upload session
	if req.FileHash != "" {
		sessionKey := fmt.Sprintf("upload_session:%d:%s", userID, req.FileHash)
		_ = core.Redis.Del(context.Background(), sessionKey).Err()
	}

	c.JSON(http.StatusOK, CompleteVideoUploadResponse{
		Msg:     "success",
		VideoID: video.ID,
	})
}

// DeleteVideo 删除视频
func (v *VideoApi) DeleteVideo(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 1. 查询视频
	var video mVideo.Video
	if err := core.DB.First(&video, id).Error; err != nil {
		// 视频不存在，返回成功 (幂等)
		c.JSON(http.StatusOK, gin.H{"msg": "success"})
		return
	}

	// 检查权限
	if video.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// 2. 软删除：只标记状态，物理清理与引用计数交由 delete worker 处理
	if err := core.DB.Model(&video).Update("status", mVideo.VideoStatusDeleted).Error; err != nil {
		core.Logger.Error("update video status failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 3. 触发异步清理任务（消费者负责清理 dash/{video_id} 与引用计数）
	msg := mq_video.VideoDeleteMessage{
		VideoID: video.ID,
	}
	msgBytes, _ := json.Marshal(msg)
	if err := core.SendKafkaMessage(context.Background(), string(consts.KafkaTopicDeleteFile), strconv.FormatInt(video.ID, 10), msgBytes); err != nil {
		core.Logger.Error("send delete kafka message failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务投递失败，请重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}

// GetVideoInfo 获取视频信息
// 获取视频信息
func (v *VideoApi) GetVideoInfo(c *gin.Context) {
	idStr := c.Param("id")
	videoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}

	key := fmt.Sprintf("videoInfo:%d", videoID)
	ctx := c.Request.Context()
	val, err := core.Redis.Get(ctx, key).Result()
	if err == nil && val != "" {
		var cached VideoInfoResponse
		if e := json.Unmarshal([]byte(val), &cached); e == nil {
			c.JSON(http.StatusOK, cached)
			return
		}
	}

	// 过期只能由一个请求刷新，其他请求等待
	lockKey := fmt.Sprintf("videoInfoV2:%d-locked", videoID)
	ok, err := core.Redis.SetNX(ctx, lockKey, "1", 5*time.Second).Result()
	if err != nil {
		core.Logger.Error("set video info lock failed", zap.Error(err))
	}
	if !ok {
		val, err = core.Redis.Get(ctx, key).Result()
		if err == nil && val != "" {
			var cached VideoInfoResponse
			if e := json.Unmarshal([]byte(val), &cached); e == nil {
				c.JSON(http.StatusOK, cached)
				return
			}
		}
	}

	var video mVideo.Video
	if err := core.DB.Preload("CoverFile").First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	var coverURL string
	if video.CoverFile != nil {
		coverURL = video.CoverFile.PublicURL(core.GetPublicBaseURL())
	}

	author := resolveAuthor(c.Request.Context(), video.UserID)

	resp := VideoInfoResponse{
		ID:          strconv.FormatInt(video.ID, 10),
		Title:       video.Title,
		Description: video.Description,
		CoverURL:    coverURL,
		Duration:    video.Duration,
		Status:      string(video.Status),
		Visibility:  string(video.Visibility),
		CreatedAt:   video.CreatedAt,
		UpdatedAt:   video.UpdatedAt,
		User:        author,
	}

	bytes, _ := json.Marshal(resp)
	_ = core.Redis.Set(ctx, key, string(bytes), timeutil.RandomRangeExpire(5*time.Minute, 10*time.Minute)).Err()
	_ = core.Redis.Del(ctx, lockKey).Err()

	c.JSON(http.StatusOK, resp)
}

type VideoPageRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Keyword  string `form:"keyword" json:"keyword"`
}

type VideoListResponse struct {
	List     []VideoInfoResponse `json:"list"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// GetVideoPage 获取视频分页列表
// 获取用户视频分页列表
func (v *VideoApi) GetSelfVideoPage(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 1. 解析查询参数
	var req VideoPageRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 2. 分页查询用户视频
	var videos []mVideo.Video
	var total int64

	// 这里用 in 禁止用 != , 因为无法使用索引优化
	db := core.DB.Model(&mVideo.Video{}).Where("user_id = ? and status IN ?", userID, []mVideo.VideoStatus{
		mVideo.VideoStatusPublished,
		mVideo.VideoStatusUploaded,
		mVideo.VideoStatusProcessing,
	})

	if req.Keyword != "" {
		db = db.Where("title LIKE ?", "%"+req.Keyword+"%")
	}

	if err := db.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Count failed"})
		return
	}

	offset := (req.Page - 1) * req.PageSize
	if err := db.Preload("CoverFile").Order("created_at DESC").Limit(req.PageSize).Offset(offset).Find(&videos).Error; err != nil {
		core.Logger.Error("get video list failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Query failed"})
		return
	}

	// 3. 构造返回数据
	var list []VideoInfoResponse
	publicURL := core.GetPublicBaseURL()

	for _, video := range videos {
		var coverURL string
		if video.CoverFile != nil {
			coverURL = video.CoverFile.PublicURL(publicURL)
		}
		list = append(list, VideoInfoResponse{
			ID:          strconv.FormatInt(video.ID, 10),
			Title:       video.Title,
			Description: video.Description,
			CoverURL:    coverURL,
			Duration:    video.Duration,
			Status:      string(video.Status),
			Visibility:  string(video.Visibility),
			CreatedAt:   video.CreatedAt,
			UpdatedAt:   video.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, VideoListResponse{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
}

type PutVideoInfoRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	CoverFileID *int64  `json:"cover_file_id,string"`
	Visibility  *string `json:"visibility"`
}

// PutVideoInfo 更新视频信息
// 更新视频信息
func (v *VideoApi) PutVideoInfo(c *gin.Context) {
	idStr := c.Param("id")
	videoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}

	var req PutVideoInfoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var video mVideo.Video
	if err := core.DB.First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	if video.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// Update fields
	if req.Title != nil {
		video.Title = *req.Title
	}
	if req.Description != nil {
		video.Description = req.Description
	}
	if req.CoverFileID != nil {
		// Optional: Check if CoverFileID exists and belongs to user or is public
		video.CoverFileID = req.CoverFileID
	}
	if req.Visibility != nil {
		video.Visibility = mVideo.VideoVisibility(*req.Visibility)
	}

	if err := core.DB.Save(&video).Error; err != nil {
		core.Logger.Error("update video failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Update failed"})
		return
	}

	// Clear cache
	key := fmt.Sprintf("videoInfo:%d", videoID)
	_ = core.Redis.Del(c.Request.Context(), key).Err()

	c.JSON(http.StatusOK, gin.H{"msg": "success", "video": video})
}

// GetVideoMdp 获取视频mdp
func (v *VideoApi) GetVideoMdp(c *gin.Context) {
	idStr := c.Param("id")
	videoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}

	var video mVideo.Video
	if err := core.DB.First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	// 1. 查询 VideoManifest 找到 manifest.mpd
	var manifest mVideo.VideoManifest
	if err := core.DB.Preload("File").Where("video_id = ? AND protocol = ? AND status = ?", videoID, "dash", mVideo.ManifestStatusReady).First(&manifest).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manifest not found"})
		return
	}

	if manifest.File == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Manifest file missing"})
		return
	}

	// 2. 从 MinIO 读取 MPD 内容
	object, err := core.Minio.GetObject(c.Request.Context(), manifest.File.Bucket, manifest.File.ObjectKey, minio.GetObjectOptions{})
	if err != nil {
		core.Logger.Error("get mpd object failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Get MPD failed"})
		return
	}
	defer object.Close()

	// 3. 设置 Content-Type 并返回内容
	c.Header("Content-Type", "application/dash+xml")
	// 可以设置 Cache-Control
	c.Header("Cache-Control", "public, max-age=3600")

	// 将流 Copy 到 Response
	if _, err := object.Stat(); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MPD file not found in storage"})
		return
	}

	stat, _ := object.Stat()
	c.DataFromReader(http.StatusOK, stat.Size, "application/dash+xml", object, nil)
}

// GetVideoSegmentsSignature 获取视频切片鉴权信息 (STS 方案)
func (v *VideoApi) GetVideoSegmentsSignature(c *gin.Context) {
	idStr := c.Param("id")
	videoID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}

	var video mVideo.Video
	if err := core.DB.First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	key := fmt.Sprintf("video:sts:%d", videoID)
	val, err := core.Redis.Get(c.Request.Context(), key).Result()

	var credsMap Credentials

	if err == nil && val != "" {
		// Redis 中有缓存，直接使用
		_ = json.Unmarshal([]byte(val), &credsMap)
	} else {
		bucket := global.AppConfig.MinIO.Bucket
		prefix := fmt.Sprintf("dash/%d/", videoID)

		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Action": ["s3:GetObject"],
					"Resource": ["arn:aws:s3:::%s/%s*"]
				}
			]
		}`, bucket, prefix)

		// 使用 STS AssumeRole 获取临时凭证
		// 注意：需要使用 Root Credentials 或者有权限的 User 来 AssumeRole
		stsOpts := credentials.STSAssumeRoleOptions{
			AccessKey: global.AppConfig.MinIO.AccessKey,
			SecretKey: global.AppConfig.MinIO.SecretKey,
			Policy:    policy,
		}

		// 构造 STS Endpoint (通常与 S3 Endpoint 相同)
		// 假设 Endpoint 配置为 "minio:9000" 或 "http://minio:9000"
		// NewSTSAssumeRole 需要完整的 URL
		// 使用内网地址连接 STS，避免公网 SSL 握手问题
		stsEndpoint := core.GetInternalBaseURL()

		stsCreds, err := credentials.NewSTSAssumeRole(stsEndpoint, stsOpts)
		if err != nil {
			core.Logger.Error("create sts provider failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "STS init failed"})
			return
		}
		// 获取凭证值
		v, err := stsCreds.Get()
		if err != nil {
			core.Logger.Error("get sts credentials failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Get STS failed"})
			return
		}

		expiration := time.Now().Add(30 * time.Minute)

		credsMap = Credentials{
			AccessKeyID:     v.AccessKeyID,
			SecretAccessKey: v.SecretAccessKey,
			SessionToken:    v.SessionToken,
			Expiration:      expiration,
		}

		bytes, _ := json.Marshal(credsMap)
		redisTTL := time.Until(time.Now().Add(25 * time.Minute))
		if redisTTL <= 0 {
			redisTTL = 5 * time.Minute
		}
		if err := core.Redis.Set(c.Request.Context(), key, string(bytes), redisTTL).Err(); err != nil {
			core.Logger.Error("save sts credentials failed", zap.Error(err))
		}
	}

	// 4. 返回给客户端
	// 客户端需要使用这些凭证构造请求头 (AWS4-HMAC-SHA256)
	// BaseURL 指向 MinIO 直接地址

	// 构造 MinIO 公网访问 BaseURL
	// 假设 MinIO Endpoint 对外可达，或者配置了 PublicURL
	// 如果在 Docker 内，Host 可能是 minio:9000，客户端无法访问
	// 实际项目中通常配置一个 Public Domain 指向 MinIO
	// BaseURL: http://minio.com/bucket/dash/{video_id}/
	baseURL := fmt.Sprintf("%s/%s/dash/%d/", core.GetPublicBaseURL(), global.AppConfig.MinIO.Bucket, videoID)

	c.JSON(http.StatusOK, gin.H{
		"base_url":    baseURL,
		"credentials": credsMap,
	})
}

// GetVideoRecommend 获取用户视频推荐,
// v1: 查询前20条按照时间desc的视频
func (v *VideoApi) GetVideoRecommend(c *gin.Context) {
	var videos []mVideo.Video
	// 查询条件：已发布，公开，按时间倒序，取前20
	err := core.DB.Model(&mVideo.Video{}).
		Where("status = ?", mVideo.VideoStatusPublished).
		Where("visibility = ?", mVideo.VideoVisibilityPublic).
		Order("created_at desc").
		Limit(20).
		Preload("CoverFile").
		Find(&videos).Error

	if err != nil {
		core.Logger.Error("get recommend videos failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取推荐视频失败"})
		return
	}

	// 批量查询作者信息（经 auth 服务）
	authors := resolveAuthors(c.Request.Context(), videos)

	var respList []VideoInfoResponse
	for _, video := range videos {
		var coverURL string
		if video.CoverFile != nil {
			coverURL = video.CoverFile.PublicURL(core.GetPublicBaseURL())
		}

		respList = append(respList, VideoInfoResponse{
			ID:          strconv.FormatInt(video.ID, 10),
			Title:       video.Title,
			Description: video.Description,
			CoverURL:    coverURL,
			Duration:    video.Duration,
			Status:      string(video.Status),
			Visibility:  string(video.Visibility),
			CreatedAt:   video.CreatedAt,
			UpdatedAt:   video.UpdatedAt,
			User:        authors[video.UserID],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"videos": respList,
	})
}
