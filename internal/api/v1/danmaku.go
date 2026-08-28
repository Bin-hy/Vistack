package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/danmaku"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var danmakuService *danmaku.Service

// SetDanmakuService 注入弹幕服务（由 role/api.go 装配）。
func SetDanmakuService(svc *danmaku.Service) {
	danmakuService = svc
}

// DanmakuApi 弹幕路由。
type DanmakuApi struct{}

type sendDanmakuRequest struct {
	Content    string  `json:"content" binding:"required"`
	TimeOffset float64 `json:"time_offset" binding:"required,gte=0"`
	Color      string  `json:"color"`
	Mode       int     `json:"mode"`
}

// SendDanmaku 发送弹幕（鉴权 + 限流）。
func (d *DanmakuApi) SendDanmaku(c *gin.Context) {
	if danmakuService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "danmaku service disabled"})
		return
	}
	videoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req sendDanmakuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Color == "" {
		req.Color = "#FFFFFF"
	}

	dm, err := danmakuService.Send(c.Request.Context(), videoID, userID, req.Content, req.TimeOffset, req.Color, req.Mode)
	if err != nil {
		if errors.Is(err, danmaku.ErrSensitive) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "弹幕包含敏感词"})
			return
		}
		core.Logger.Error("send danmaku failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          dm.ID,
		"time_offset": dm.TimeOffset,
		"content":     dm.Content,
		"color":       dm.Color,
		"mode":        dm.Mode,
	})
}

// GetDanmaku 按时间范围分段拉取弹幕（公开）。
func (d *DanmakuApi) GetDanmaku(c *gin.Context) {
	if danmakuService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "danmaku service disabled"})
		return
	}
	videoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return
	}
	start, _ := strconv.ParseFloat(c.DefaultQuery("start", "0"), 64)
	end, _ := strconv.ParseFloat(c.DefaultQuery("end", "60"), 64)
	if end <= start {
		end = start + 60
	}

	items, err := danmakuService.Fetch(c.Request.Context(), videoID, start, end)
	if err != nil {
		core.Logger.Error("fetch danmaku failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.Header("Cache-Control", "public, max-age=5")
	c.JSON(http.StatusOK, gin.H{"danmaku": items})
}
