package v1

import (
	"net/http"
	"strconv"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/interaction"
	mVideo "github.com/binhy/vistack/internal/model/entity/video"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var interactionService *interaction.Service

// SetInteractionService 注入点赞/收藏/播放量服务（由 role/api.go 装配）。
func SetInteractionService(svc *interaction.Service) {
	interactionService = svc
}

// SocialApi 社交互动路由。
type SocialApi struct{}

func socialVideoID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return 0, false
	}
	return id, true
}

func socialUserID(c *gin.Context) (int64, bool) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, false
	}
	return userID, true
}

// LikeVideo 点赞/取消点赞（toggle）。
func (s *SocialApi) LikeVideo(c *gin.Context) {
	if interactionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "interaction service disabled"})
		return
	}
	videoID, ok := socialVideoID(c)
	if !ok {
		return
	}
	userID, ok := socialUserID(c)
	if !ok {
		return
	}

	liked, count, err := interactionService.ToggleLike(c.Request.Context(), videoID, userID)
	if err != nil {
		core.Logger.Error("toggle like failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked, "like_count": count})
}

// FavoriteVideo 收藏/取消收藏（toggle）。
func (s *SocialApi) FavoriteVideo(c *gin.Context) {
	if interactionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "interaction service disabled"})
		return
	}
	videoID, ok := socialVideoID(c)
	if !ok {
		return
	}
	userID, ok := socialUserID(c)
	if !ok {
		return
	}

	favorited, count, err := interactionService.ToggleFavorite(c.Request.Context(), videoID, userID)
	if err != nil {
		core.Logger.Error("toggle favorite failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"favorited": favorited, "favorite_count": count})
}

// PlayVideo 播放上报（公开，计数 +1）。
func (s *SocialApi) PlayVideo(c *gin.Context) {
	if interactionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "interaction service disabled"})
		return
	}
	videoID, ok := socialVideoID(c)
	if !ok {
		return
	}

	count, err := interactionService.RecordPlay(c.Request.Context(), videoID)
	if err != nil {
		core.Logger.Error("record play failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败，请重试"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"play_count": count})
}

// GetVideoStats 视频三计数（公开；Redis 不可用回退 DB 冗余列）。
func (s *SocialApi) GetVideoStats(c *gin.Context) {
	videoID, ok := socialVideoID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	if interactionService != nil {
		if counts, err := interactionService.Counts(ctx, []int64{videoID}); err == nil {
			cc := counts[videoID]
			c.JSON(http.StatusOK, gin.H{
				"like_count":     cc.LikeCount,
				"favorite_count": cc.FavoriteCount,
				"play_count":     cc.PlayCount,
			})
			return
		} else {
			core.Logger.Warn("read counts from redis failed, fallback to db", zap.Int64("video_id", videoID), zap.Error(err))
		}
	}

	var video mVideo.Video
	if err := core.DB.Select("like_count", "favorite_count", "play_count").First(&video, videoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Video not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"like_count":     video.LikeCount,
		"favorite_count": video.FavoriteCount,
		"play_count":     video.PlayCount,
	})
}

// GetVideoInteraction 当前用户对视频的点赞/收藏状态（鉴权）。
func (s *SocialApi) GetVideoInteraction(c *gin.Context) {
	if interactionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "interaction service disabled"})
		return
	}
	videoID, ok := socialVideoID(c)
	if !ok {
		return
	}
	userID, ok := socialUserID(c)
	if !ok {
		return
	}

	liked, err1 := interactionService.IsLiked(c.Request.Context(), videoID, userID)
	favorited, err2 := interactionService.IsFavorited(c.Request.Context(), videoID, userID)
	if err1 != nil || err2 != nil {
		core.Logger.Error("get interaction status failed", zap.Int64("video_id", videoID), zap.Error(firstNonNil(err1, err2)))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked, "favorited": favorited})
}

type hotVideoItem struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CoverURL  string `json:"cover_url,omitempty"`
	PlayCount int64  `json:"play_count"`
	LikeCount int64  `json:"like_count"`
}

// GetHotVideos 热门榜单（公开，sort=play|like）。
func (s *SocialApi) GetHotVideos(c *gin.Context) {
	if interactionService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "interaction service disabled"})
		return
	}
	sort := c.DefaultQuery("sort", "play")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	ids, err := interactionService.Hot(c.Request.Context(), sort, limit)
	if err != nil {
		core.Logger.Error("get hot videos failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	if len(ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"videos": []hotVideoItem{}})
		return
	}

	var videos []mVideo.Video
	if err := core.DB.Where("id IN ?", ids).Preload("CoverFile").Find(&videos).Error; err != nil {
		core.Logger.Error("load hot videos failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	byID := make(map[int64]mVideo.Video, len(videos))
	for _, v := range videos {
		byID[v.ID] = v
	}
	counts, _ := interactionService.Counts(c.Request.Context(), ids)

	list := make([]hotVideoItem, 0, len(ids))
	publicURL := core.GetPublicBaseURL()
	for _, id := range ids {
		v, ok := byID[id]
		if !ok {
			continue
		}
		item := hotVideoItem{ID: id, Title: v.Title}
		if v.CoverFile != nil {
			item.CoverURL = v.CoverFile.PublicURL(publicURL)
		}
		if cc, ok := counts[id]; ok {
			item.PlayCount = cc.PlayCount
			item.LikeCount = cc.LikeCount
		}
		list = append(list, item)
	}
	c.JSON(http.StatusOK, gin.H{"videos": list})
}

func firstNonNil(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
