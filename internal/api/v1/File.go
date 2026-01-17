package v1

import (
	"net/http"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/pkg/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type FileApi struct{}

type FileUploadedResponse struct {
	FileID     int64  `json:"file_id,string"`
	URL        string `json:"url"`
	ObjectName string `json:"object_name"`
	FileName   string `json:"filename"`
}

// AvatarUpload 头像文件上传, 返回文件路径
func (f *FileApi) AvatarUpload(c *gin.Context) {
	// 1. 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Get file failed"})
		return
	}

	// 2. 校验文件大小 (例如限制 5MB)
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 5MB)"})
		return
	}

	// 3. 上传到 MinIO 的 "avatars" 目录
	objectName, fullURL, err := storage.UploadFile(c.Request.Context(), file, "avatars")
	if err != nil {
		core.Logger.Error("upload avatar failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload failed"})
		return
	}

	// 4. 创建文件记录
	newFile := mFile.File{
		Bucket:    global.AppConfig.MinIO.Bucket,
		ObjectKey: objectName,
		Status:    "active",
		RefType:   "avatar", // 暂时标记为 avatar，关联 ID 在用户更新时设置
		MimeType:  file.Header.Get("Content-Type"),
		Size:      file.Size,
	}

	if err := core.DB.Create(&newFile).Error; err != nil {
		core.Logger.Error("create file record failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Save file record failed"})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, FileUploadedResponse{
		FileID:     newFile.ID,
		URL:        fullURL,
		ObjectName: objectName,
		FileName:   file.Filename,
	})
}

// CoverUpload 封面文件上传, 返回文件路径
func (f *FileApi) CoverUpload(c *gin.Context) {
	// 1. 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Get file failed"})
		return
	}

	// 2. 校验文件大小 (例如限制 5MB)
	if file.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 5MB)"})
		return
	}

	// 3. 上传到 MinIO 的 "covers" 目录
	objectName, fullURL, err := storage.UploadFile(c.Request.Context(), file, "covers")
	if err != nil {
		core.Logger.Error("upload cover failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload failed"})
		return
	}

	// 4. 创建文件记录
	newFile := mFile.File{
		Bucket:    global.AppConfig.MinIO.Bucket,
		ObjectKey: objectName,
		Status:    "active",
		RefType:   "cover", // 暂时标记为 cover，关联 ID 在视频更新时设置，关联视频 ID
		MimeType:  file.Header.Get("Content-Type"),
		Size:      file.Size,
	}

	if err := core.DB.Create(&newFile).Error; err != nil {
		core.Logger.Error("create file record failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Save file record failed"})
		return
	}

	// 5. 返回结果
	c.JSON(http.StatusOK, FileUploadedResponse{
		FileID:     newFile.ID,
		URL:        fullURL,
		ObjectName: objectName,
		FileName:   file.Filename,
	})
}
