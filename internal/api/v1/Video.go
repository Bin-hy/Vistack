package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/core"
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
	"gorm.io/gorm/clause"
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
	Filename string `json:"filename" binding:"required"`
	MimeType string `json:"mime_type"`
}
type VideoInitResponse struct {
	UploadID  string `json:"upload_id"`
	ObjectKey string `json:"object_key"`
	Bucket    string `json:"bucket"`
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

	c.JSON(http.StatusOK, VideoInitResponse{
		UploadID:  uploadID,
		ObjectKey: objectKey,
		Bucket:    bucket,
	})
}

// UploadVideoPartRequest 上传分片请求
type UploadVideoPartRequest struct {
	UploadID   string `json:"upload_id" binding:"required"`
	ObjectKey  string `json:"object_key" binding:"required"`
	PartNumber int    `json:"part_number" binding:"required"`
}

type UploadVideoPartResponse struct {
	ETag string `json:"etag"`
}

// UploadVideoPart 上传分片
func (v *VideoApi) UploadVideoPart(c *gin.Context) {
	// 0. 获取参数
	uploadID := c.PostForm("upload_id")
	objectKey := c.PostForm("object_key")
	partNumberStr := c.PostForm("part_number")

	if uploadID == "" || objectKey == "" || partNumberStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing parameters"})
		return
	}

	partNumber, err := strconv.Atoi(partNumberStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid part_number"})
		return
	}

	// 1. 获取上传的文件分片
	file, err := c.FormFile("chunk")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Get chunk failed"})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Open chunk failed"})
		return
	}
	defer src.Close()

	bucket := global.AppConfig.MinIO.Bucket

	// 2. 上传分片到 MinIO
	part, err := core.MinioCore.PutObjectPart(c.Request.Context(), bucket, objectKey, uploadID, partNumber, src, file.Size, minio.PutObjectPartOptions{})
	if err != nil {
		core.Logger.Error("upload part failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload part failed"})
		return
	}

	c.JSON(http.StatusOK, UploadVideoPartResponse{
		ETag: part.ETag,
	})
}

type CompleteVideoUpload struct {
	UploadID  string               `json:"upload_id" binding:"required"`
	ObjectKey string               `json:"object_key" binding:"required"`
	Filename  string               `json:"filename" binding:"required"`
	Parts     []minio.CompletePart `json:"parts" binding:"required"` // 需要前端传回所有分片的 ETag
}

type TranscodeMessage struct {
	VideoID     int64  `json:"video_id"`
	TranscodeID int64  `json:"transcode_id"`
	ObjectKey   string `json:"object_key"`
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
		Status: "processing", // 标记为处理中
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
		Status:    "active",
		RefType:   "video_source",
		RefID:     video.ID,
		MimeType:  "video/mp4", // 暂时假定
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
	transcode := mVideo.VideoTranscode{
		VideoID: video.ID,
		Status:  "pending",
	}
	if err := tx.Create(&transcode).Error; err != nil {
		tx.Rollback()
		core.Logger.Error("create transcode task failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	tx.Commit()

	// 3. 发送 Kafka 消息
	// 消息格式: {"video_id": 1, "transcode_id": 1, "object_key": "raw/xxx.mp4"}
	msg := TranscodeMessage{
		VideoID:     video.ID,
		TranscodeID: transcode.ID,
		ObjectKey:   req.ObjectKey,
	}
	msgBytes, _ := json.Marshal(msg)

	// 使用 "transcode" 作为 Topic
	if err := core.SendKafkaMessage(context.Background(), "transcode", strconv.FormatInt(video.ID, 10), msgBytes); err != nil {
		// 发送失败不回滚数据库，可以由定时任务补救
		core.Logger.Error("send kafka message failed", zap.Error(err))
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
		// 视频不存在，返回成功
		c.JSON(http.StatusOK, gin.H{"msg": "success"})
		return
	}

	// 检查权限
	if video.UserID != userID {
		// 这里简化处理，实际可能有管理员权限
		c.JSON(http.StatusForbidden, gin.H{"error": "Permission denied"})
		return
	}

	// 2. 删除 MinIO 文件 (Dash 目录, Raw 文件, Cover)
	bucket := global.AppConfig.MinIO.Bucket

	// 2.1 删除 Dash 目录 (dash/{video_id}/)
	dashPrefix := fmt.Sprintf("dash/%d/", video.ID)
	deleteObjects(bucket, dashPrefix)

	// 2.2 删除 Raw 文件 (通过 Source 关联)
	var sources []mVideo.VideoSource
	core.DB.Preload("File").Where("video_id = ?", video.ID).Find(&sources)
	for _, s := range sources {
		if s.File != nil {
			deleteObject(bucket, s.File.ObjectKey)
		}
	}

	// 2.3 删除封面
	if video.CoverFileID != nil {
		var cover mFile.File
		if err := core.DB.First(&cover, *video.CoverFileID).Error; err == nil {
			deleteObject(bucket, cover.ObjectKey)
		}
	}

	// 3. 删除数据库记录 (级联删除)
	if err := core.DB.Select(clause.Associations).Delete(&video).Error; err != nil {
		core.Logger.Error("delete video record failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}

func deleteObject(bucket, key string) {
	if err := core.Minio.RemoveObject(context.Background(), bucket, key, minio.RemoveObjectOptions{}); err != nil {
		core.Logger.Error("remove object failed", zap.String("key", key), zap.Error(err))
	}
}

func deleteObjects(bucket, prefix string) {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for object := range core.Minio.ListObjects(context.Background(), bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if object.Err != nil {
				continue
			}
			objectsCh <- object
		}
	}()

	opts := minio.RemoveObjectsOptions{GovernanceBypass: true}
	for err := range core.Minio.RemoveObjects(context.Background(), bucket, objectsCh, opts) {
		core.Logger.Error("remove objects failed", zap.Error(err.Err))
	}
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
	if err := core.DB.Preload("CoverFile").Preload("User.Profile.Avatar").First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}

	var coverURL string
	if video.CoverFile != nil {
		coverURL = video.CoverFile.PublicURL(core.GetPublicBaseURL())
	}

	var author *VideoAuthorResponse
	if video.User != nil {
		nickname := video.User.Username
		if video.User.Profile != nil && video.User.Profile.Nickname != nil && *video.User.Profile.Nickname != "" {
			nickname = *video.User.Profile.Nickname
		}

		var avatarURL string
		if video.User.Profile != nil && video.User.Profile.Avatar != nil {
			avatarURL = video.User.Profile.Avatar.PublicURL(core.GetPublicBaseURL())
		}
		fmt.Println("avatarURL:", avatarURL)
		author = &VideoAuthorResponse{
			ID:        strconv.FormatInt(video.User.ID, 10),
			Nickname:  nickname,
			AvatarURL: avatarURL,
		}
	}

	resp := VideoInfoResponse{
		ID:          strconv.FormatInt(video.ID, 10),
		Title:       video.Title,
		Description: video.Description,
		CoverURL:    coverURL,
		Duration:    video.Duration,
		Status:      video.Status,
		Visibility:  video.Visibility,
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

	db := core.DB.Model(&mVideo.Video{}).Where("user_id = ?", userID)

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
			Status:      video.Status,
			Visibility:  video.Visibility,
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
		video.Visibility = *req.Visibility
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
	if err := core.DB.Preload("File").Where("video_id = ? AND protocol = ? AND status = ?", videoID, "dash", "ready").First(&manifest).Error; err != nil {
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
		stsEndpoint := core.GetPublicBaseURL()

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
	c.JSON(http.StatusOK, gin.H{
		"videos": []mVideo.Video{},
	})
}
