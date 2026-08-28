package v1

import (
	"net/http"
	"strconv"

	"github.com/binhy/vistack/internal/core"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// SensitiveWordApi 敏感词管理路由。
type SensitiveWordApi struct{}

// ListSensitiveWords 列出全部敏感词（鉴权）。
func (s *SensitiveWordApi) ListSensitiveWords(c *gin.Context) {
	if danmakuService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "danmaku service disabled"})
		return
	}
	words, err := danmakuService.ListSensitiveWords(c.Request.Context())
	if err != nil {
		core.Logger.Error("list sensitive words failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"words": words})
}

// AddSensitiveWord 新增敏感词（鉴权，写 DB 后实时重建自动机）。
func (s *SensitiveWordApi) AddSensitiveWord(c *gin.Context) {
	if danmakuService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "danmaku service disabled"})
		return
	}
	var req struct {
		Word string `json:"word" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := danmakuService.AddSensitiveWord(c.Request.Context(), req.Word); err != nil {
		core.Logger.Error("add sensitive word failed", zap.String("word", req.Word), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "添加失败（可能已存在）"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}

// DeleteSensitiveWord 删除敏感词（鉴权，删 DB 后实时重建自动机）。
func (s *SensitiveWordApi) DeleteSensitiveWord(c *gin.Context) {
	if danmakuService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "danmaku service disabled"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}
	if err := danmakuService.DeleteSensitiveWord(c.Request.Context(), id); err != nil {
		core.Logger.Error("delete sensitive word failed", zap.Int64("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"msg": "success"})
}
