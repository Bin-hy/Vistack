package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/middlewares"
	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/internal/model/entity/user"
	authpkg "github.com/binhy/vistack/pkg/auth"
	"github.com/binhy/vistack/pkg/hashutil"
	"github.com/binhy/vistack/pkg/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Handler 承载 auth 服务的 HTTP 接口：注册/登录/用户资料/改密/JWKS。
type Handler struct {
	tm *authpkg.TokenManager
}

func NewHandler(tm *authpkg.TokenManager) *Handler {
	return &Handler{tm: tm}
}

// RegisterRoutes 注册 auth 服务对外路由（路径与迁移前 api 保持一致）。
// validator 用于受保护路由的本地验签：auth 服务内部用 TokenManager（私钥），api 用 TokenVerifier（JWKS）。
func RegisterRoutes(r *gin.Engine, h *Handler, validator authpkg.TokenValidator) {
	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
	}

	userGroup := r.Group("/api/v1")
	userGroup.Use(middlewares.AuthMiddleware(validator))
	{
		userGroup.GET("/user/info", h.GetUserInfo)
		userGroup.PUT("/user/profile", h.UpdateProfileDirect)
		userGroup.PUT("/user/password", h.UpdateUserPassword)
	}

	r.GET("/.well-known/jwks.json", h.JWKS)
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email" binding:"omitempty,email"`
	Nickname string `json:"nickname"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserInfo struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Email     *string   `json:"email,omitempty"`
	AvatarURL string    `json:"avatar_url,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type LoginResponse struct {
	Token string    `json:"token"`
	User  *UserInfo `json:"user"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type UpdateProfileDirectRequest struct {
	Nickname string `form:"nickname" binding:"omitempty,min=2,max=20"`
}

// Register 用户注册（用户名/昵称/邮箱唯一性校验、默认角色、bcrypt）
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var exists int64
	if err := core.DB.Model(&user.User{}).Where("username = ?", req.Username).Count(&exists).Error; err != nil {
		core.Logger.Error("check username failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}
	if exists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		return
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	var nicknameExists int64
	if err := core.DB.Model(&user.UserProfile{}).Where("nickname = ?", nickname).Count(&nicknameExists).Error; err != nil {
		core.Logger.Error("check nickname failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}
	if nicknameExists > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Nickname already exists"})
		return
	}

	hash, err := hashutil.HashPassword(req.Password)
	if err != nil {
		core.Logger.Error("hash password failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	var defaultRole user.Role
	if err := core.DB.Where("name = ?", "user").First(&defaultRole).Error; err != nil {
		core.Logger.Error("default role not found", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal configuration error: default role not found"})
		return
	}

	newUser := user.User{
		Username:     req.Username,
		PasswordHash: hash,
		RoleID:       defaultRole.ID,
		Status:       user.UserStatusActive,
		Profile: &user.UserProfile{
			Nickname: &nickname,
		},
	}
	if req.Email != "" {
		newUser.Email = &req.Email
	}

	if err := core.DB.Create(&newUser).Error; err != nil {
		core.Logger.Error("create user failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Register success", "user_id": newUser.ID})
}

// Login 用户登录：校验密码、RS256 签发 token、返回用户信息
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var targetUser user.User
	if err := core.DB.Where("username = ?", req.Username).First(&targetUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		} else {
			core.Logger.Error("query user failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		}
		return
	}

	if !hashutil.CheckPasswordHash(req.Password, targetUser.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	token, err := h.tm.GenerateToken(targetUser.ID)
	if err != nil {
		core.Logger.Error("generate token failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	if err := core.DB.Preload("Role").Preload("Profile").Preload("Profile.Avatar").First(&targetUser, targetUser.ID).Error; err != nil {
		core.Logger.Error("load user with profile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  buildUserInfo(&targetUser),
	})
}

// GetUserInfo 获取当前用户信息
func (h *Handler) GetUserInfo(c *gin.Context) {
	userID := authpkg.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var currentUser user.User
	if err := core.DB.Preload("Role").Preload("Profile").Preload("Profile.Avatar").First(&currentUser, userID).Error; err != nil {
		core.Logger.Error("get user info failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": buildUserInfo(&currentUser)})
}

// UpdateProfileDirect 合并头像上传与资料更新
func (h *Handler) UpdateProfileDirect(c *gin.Context) {
	var req UpdateProfileDirectRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := authpkg.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	fileHeader, err := c.FormFile("avatar")
	hasFile := err == nil
	if hasFile && fileHeader.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 5MB)"})
		return
	}

	tx := core.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var newFileID int64
	var objectName string
	var fullURL string

	if hasFile {
		objectName, fullURL, err = storage.UploadFile(c.Request.Context(), fileHeader, "avatars")
		if err != nil {
			tx.Rollback()
			core.Logger.Error("upload avatar failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload failed"})
			return
		}

		newFile := file.File{
			Bucket:    global.AppConfig.MinIO.Bucket,
			ObjectKey: objectName,
			Status:    "active",
			RefType:   "avatar",
			MimeType:  fileHeader.Header.Get("Content-Type"),
			Size:      fileHeader.Size,
		}
		if err := tx.Create(&newFile).Error; err != nil {
			tx.Rollback()
			core.Logger.Error("create file record failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Save file record failed"})
			return
		}
		newFileID = newFile.ID
	}

	var currentProfile user.UserProfile
	if err := tx.Where("user_id = ?", userID).First(&currentProfile).Error; err != nil {
		// profile 不存在时后续逻辑会创建
	}

	if req.Nickname != "" && (currentProfile.Nickname == nil || *currentProfile.Nickname != req.Nickname) {
		var count int64
		if err := tx.Model(&user.UserProfile{}).
			Where("nickname = ? AND user_id != ?", req.Nickname, userID).
			Count(&count).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}
		if count > 0 {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Nickname already taken"})
			return
		}
	}

	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if hasFile {
		updates["avatar_file_id"] = newFileID
	}

	if len(updates) > 0 {
		result := tx.Model(&user.UserProfile{}).Where("user_id = ?", userID).Updates(updates)
		if result.Error != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}
		if result.RowsAffected == 0 {
			profile := user.UserProfile{UserID: userID}
			if req.Nickname != "" {
				profile.Nickname = &req.Nickname
			}
			if hasFile {
				profile.AvatarFileID = &newFileID
			}
			if err := tx.Create(&profile).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create profile"})
				return
			}
		}
	}

	if hasFile && currentProfile.AvatarFileID != nil {
		if err := tx.Model(&file.File{}).
			Where("id = ?", *currentProfile.AvatarFileID).
			Update("status", "replaced").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update old avatar status"})
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	if hasFile && currentProfile.AvatarFileID != nil {
		oldID := *currentProfile.AvatarFileID
		go func(fileID int64) {
			var f file.File
			if err := core.DB.First(&f, fileID).Error; err == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = storage.MarkObjectAsReplaced(ctx, f.ObjectKey)
			}
		}(oldID)
	}

	resp := gin.H{"message": "Profile updated successfully"}
	if hasFile {
		resp["avatar_url"] = fullURL
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateUserPassword 修改密码
func (h *Handler) UpdateUserPassword(c *gin.Context) {
	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := authpkg.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var currentUser user.User
	if err := core.DB.Select("id, password_hash").First(&currentUser, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !hashutil.CheckPasswordHash(req.OldPassword, currentUser.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Incorrect old password"})
		return
	}

	newHash, err := hashutil.HashPassword(req.NewPassword)
	if err != nil {
		core.Logger.Error("hash password failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	}

	if err := core.DB.Model(&user.User{}).Where("id = ?", userID).Update("password_hash", newHash).Error; err != nil {
		core.Logger.Error("update password failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

// JWKS 返回 RS256 公钥的 JWKS
func (h *Handler) JWKS(c *gin.Context) {
	jwks, err := h.tm.PublicJWKS()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/json", jwks)
}

func buildUserInfo(u *user.User) *UserInfo {
	nickname := ""
	if u.Profile != nil && u.Profile.Nickname != nil {
		nickname = *u.Profile.Nickname
	}
	if nickname == "" {
		nickname = u.Username
	}
	avatarURL := ""
	if u.Profile != nil && u.Profile.Avatar != nil {
		f := u.Profile.Avatar
		if f.Bucket != "" && f.ObjectKey != "" {
			avatarURL = f.PublicURL(core.GetPublicBaseURL())
		}
	}
	roleName := ""
	if u.Role.ID != 0 {
		roleName = u.Role.Name
	}
	return &UserInfo{
		ID:        u.ID,
		Username:  u.Username,
		Nickname:  nickname,
		Email:     u.Email,
		AvatarURL: avatarURL,
		Role:      roleName,
		CreatedAt: u.CreatedAt,
	}
}
