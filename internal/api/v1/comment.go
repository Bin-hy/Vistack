package v1

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/binhy/vistack/internal/comment"
	"github.com/binhy/vistack/internal/core"
	mFile "github.com/binhy/vistack/internal/model/entity/file"
	mSocial "github.com/binhy/vistack/internal/model/entity/social"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var commentService *comment.Service

// SetCommentService 注入评论服务（由 role/api.go 装配）。
func SetCommentService(svc *comment.Service) { commentService = svc }

// CommentApi 评论路由。
type CommentApi struct{}

type createCommentRequest struct {
	Content     string                      `json:"content"`
	ParentID    *int64                      `json:"parent_id"`
	ReplyToID   *int64                      `json:"reply_to_id"`
	Attachments []mSocial.CommentAttachment `json:"attachments"`
}

type commentAttachmentResponse struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type commentItemResponse struct {
	ID            int64                       `json:"id,string"`
	VideoID       int64                       `json:"video_id,string"`
	UserID        int64                       `json:"user_id,string"`
	RootID        *int64                      `json:"root_id,omitempty"`
	ParentID      *int64                      `json:"parent_id,omitempty"`
	ReplyToID     *int64                      `json:"reply_to_id,omitempty"`
	Content       string                      `json:"content"`
	Attachments   []commentAttachmentResponse `json:"attachments,omitempty"`
	Status        string                      `json:"status"`
	LikeCount     int64                       `json:"like_count"`
	ReplyCount    int64                       `json:"reply_count"`
	CreatedAt     time.Time                   `json:"created_at"`
	Deleted       bool                        `json:"deleted"`
	Author        *VideoAuthorResponse        `json:"author,omitempty"`
	ReplyToAuthor *VideoAuthorResponse        `json:"reply_to_author,omitempty"`
	Liked         bool                        `json:"liked"`
	Replies       []commentItemResponse       `json:"replies,omitempty"`
}

type commentListResponse struct {
	Comments []commentItemResponse `json:"comments"`
	Next     int64                 `json:"next_cursor,string"`
	Total    int64                 `json:"total,string"`
}

func commentVideoID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Video ID"})
		return 0, false
	}
	return id, true
}

func commentID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Comment ID"})
		return 0, false
	}
	return id, true
}

func commentCursor(c *gin.Context) int64 {
	n, _ := strconv.ParseInt(c.DefaultQuery("cursor", "0"), 10, 64)
	return n
}

func commentLimit(c *gin.Context) int {
	n, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if n <= 0 || n > 100 {
		n = 20
	}
	return n
}

// ListComments 一级评论列表（公开）。
func (a *CommentApi) ListComments(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	videoID, ok := commentVideoID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	userID := auth.GetUserID(c)

	roots, next, err := commentService.List(ctx, videoID, commentCursor(c), commentLimit(c))
	if err != nil {
		core.Logger.Error("list comments failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	items := a.buildItems(ctx, roots, userID, false)
	for i, root := range roots {
		replies, _, err := commentService.ListReplies(ctx, root.ID, 0, 2)
		if err == nil && len(replies) > 0 {
			items[i].Replies = a.buildItems(ctx, replies, userID, false)
		}
	}

	total, _ := commentService.CommentCount(ctx, videoID)
	c.JSON(http.StatusOK, commentListResponse{Comments: items, Next: next, Total: total})
}

// ListReplies 展开某根评论下的回复（公开）。
func (a *CommentApi) ListReplies(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	rootID, ok := commentID(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()
	userID := auth.GetUserID(c)

	replies, next, err := commentService.ListReplies(ctx, rootID, commentCursor(c), commentLimit(c))
	if err != nil {
		core.Logger.Error("list replies failed", zap.Int64("root_id", rootID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"comments": a.buildItems(ctx, replies, userID, false), "next_cursor": next})
}

// CreateComment 发表评论/回复（鉴权）。
func (a *CommentApi) CreateComment(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	videoID, ok := commentVideoID(c)
	if !ok {
		return
	}
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Content == "" && len(req.Attachments) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "评论内容不能为空"})
		return
	}

	created, err := commentService.Create(c.Request.Context(), comment.CreateInput{
		VideoID:     videoID,
		UserID:      userID,
		Content:     req.Content,
		ParentID:    req.ParentID,
		ReplyToID:   req.ReplyToID,
		Attachments: req.Attachments,
	})
	if err != nil {
		if err == comment.ErrSensitive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "评论包含敏感词"})
			return
		}
		if err == comment.ErrNotFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "回复的评论不存在"})
			return
		}
		if err == comment.ErrTooManyAttachments {
			c.JSON(http.StatusBadRequest, gin.H{"error": "图片数量超过限制（最多 9 张）"})
			return
		}
		core.Logger.Error("create comment failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发表失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          created.ID,
		"status":      created.Status,
		"content":     created.Content,
		"root_id":     created.RootID,
		"parent_id":   created.ParentID,
		"reply_to_id": created.ReplyToID,
	})
}

// ToggleLike 评论点赞/取消（鉴权）。
func (a *CommentApi) ToggleLike(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	commentID, ok := commentID(c)
	if !ok {
		return
	}
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	liked, count, err := commentService.ToggleLike(c.Request.Context(), commentID, userID)
	if err != nil {
		if err == comment.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
			return
		}
		core.Logger.Error("toggle comment like failed", zap.Int64("comment_id", commentID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"liked": liked, "like_count": count})
}

// DeleteComment 删除评论（鉴权 + 归属校验）。
func (a *CommentApi) DeleteComment(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	commentID, ok := commentID(c)
	if !ok {
		return
	}
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := commentService.Delete(c.Request.Context(), commentID, userID)
	if err != nil {
		if err == comment.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "评论不存在"})
			return
		}
		if err == comment.ErrForbidden {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权删除该评论"})
			return
		}
		core.Logger.Error("delete comment failed", zap.Int64("comment_id", commentID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// CommentCount 视频评论总数（公开）。
func (a *CommentApi) CommentCount(c *gin.Context) {
	if commentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "comment service disabled"})
		return
	}
	videoID, ok := commentVideoID(c)
	if !ok {
		return
	}
	total, err := commentService.CommentCount(c.Request.Context(), videoID)
	if err != nil {
		core.Logger.Error("comment count failed", zap.Int64("video_id", videoID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"total": total})
}

// buildItems 把评论实体转为响应，批量填充作者、附件 URL 与点赞状态。
func (a *CommentApi) buildItems(ctx context.Context, comments []mSocial.VideoComment, userID int64, _ bool) []commentItemResponse {
	if len(comments) == 0 {
		return []commentItemResponse{}
	}
	authorIDs := map[int64]struct{}{}
	fileIDs := map[int64]struct{}{}
	for i := range comments {
		authorIDs[comments[i].UserID] = struct{}{}
		if comments[i].ReplyToUID != nil {
			authorIDs[*comments[i].ReplyToUID] = struct{}{}
		}
		if comments[i].Attachments != "" {
			atts, _ := comment.ParseAttachments(comments[i].Attachments)
			for _, at := range atts {
				fileIDs[at.FileID] = struct{}{}
			}
		}
	}
	authors := resolveCommentAuthors(ctx, keysOf(authorIDs))
	fileURLs := resolveFileURLs(ctx, keysOf(fileIDs))

	items := make([]commentItemResponse, 0, len(comments))
	for i := range comments {
		items = append(items, a.toItem(ctx, comments[i], userID, authors, fileURLs))
	}
	return items
}

func (a *CommentApi) toItem(ctx context.Context, cm mSocial.VideoComment, userID int64, authors map[int64]*VideoAuthorResponse, fileURLs map[int64]string) commentItemResponse {
	item := commentItemResponse{
		ID:         cm.ID,
		VideoID:    cm.VideoID,
		UserID:     cm.UserID,
		RootID:     cm.RootID,
		ParentID:   cm.ParentID,
		ReplyToID:  cm.ReplyToID,
		Content:    cm.Content,
		Status:     string(cm.Status),
		LikeCount:  cm.LikeCount,
		ReplyCount: cm.ReplyCount,
		CreatedAt:  cm.CreatedAt,
		Deleted:    cm.Status == mSocial.CommentStatusDeleted,
		Author:     authors[cm.UserID],
	}
	if cm.ReplyToUID != nil {
		item.ReplyToAuthor = authors[*cm.ReplyToUID]
	}
	if cm.Attachments != "" {
		atts, _ := comment.ParseAttachments(cm.Attachments)
		for _, at := range atts {
			item.Attachments = append(item.Attachments, commentAttachmentResponse{
				Type: at.Type,
				URL:  fileURLs[at.FileID],
			})
		}
	}
	if userID != 0 {
		if liked, err := commentService.IsLiked(ctx, cm.ID, userID); err == nil {
			item.Liked = liked
		}
	}
	return item
}

func keysOf(m map[int64]struct{}) []int64 {
	ids := make([]int64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

// resolveCommentAuthors 批量查询评论作者信息。
func resolveCommentAuthors(ctx context.Context, ids []int64) map[int64]*VideoAuthorResponse {
	result := make(map[int64]*VideoAuthorResponse)
	if userClient == nil || len(ids) == 0 {
		return result
	}
	infos, err := userClient.GetUserInfos(ctx, ids)
	if err != nil {
		core.Logger.Error("get comment user infos failed", zap.Error(err))
		return result
	}
	for id, info := range infos {
		result[id] = toAuthor(info)
	}
	return result
}

// resolveFileURLs 批量查询附件文件的公网 URL。
func resolveFileURLs(ctx context.Context, ids []int64) map[int64]string {
	result := make(map[int64]string)
	if len(ids) == 0 {
		return result
	}
	var files []mFile.File
	if err := core.DB.WithContext(ctx).Where("id IN ?", ids).Find(&files).Error; err != nil {
		return result
	}
	base := core.GetPublicBaseURL()
	for _, f := range files {
		result[f.ID] = f.PublicURL(base)
	}
	return result
}
