package v1

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/global"
	"github.com/binhy/vistack/internal/model/entity/file"
	"github.com/binhy/vistack/internal/model/entity/user"
	"github.com/binhy/vistack/pkg/auth"
	"github.com/binhy/vistack/pkg/hashutil"
	"github.com/binhy/vistack/pkg/storage"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserApi struct {
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

type UpdateProfileRequest struct {
	Nickname     string `json:"nickname" binding:"omitempty,min=2,max=20"`
	AvatarFileID int64  `json:"avatar_file_id" binding:"omitempty"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// 用户注册
func (u *UserApi) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// 检查用户名是否存在
	var exists int64
	if err := core.DB.Model(&user.User{}).Where("username = ?", req.Username).Count(&exists).Error; err != nil {
		core.Logger.Error("check username failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}
	if exists > 0 {
		c.JSON(400, gin.H{"error": "Username already exists"})
		return
	}

	// 处理昵称逻辑：如果未提供则使用用户名
	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	// 检查昵称唯一性
	var nicknameExists int64
	if err := core.DB.Model(&user.UserProfile{}).Where("nickname = ?", nickname).Count(&nicknameExists).Error; err != nil {
		core.Logger.Error("check nickname failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}
	if nicknameExists > 0 {
		c.JSON(400, gin.H{"error": "Nickname already exists"})
		return
	}

	// 密码加密
	hash, err := hashutil.HashPassword(req.Password)
	if err != nil {
		core.Logger.Error("hash password failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}

	// 查找默认角色 "user"
	var defaultRole user.Role
	if err := core.DB.Where("name = ?", "user").First(&defaultRole).Error; err != nil {
		core.Logger.Error("default role not found", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal configuration error: default role not found"})
		return
	}

	newUser := user.User{
		Username:     req.Username,
		PasswordHash: hash,
		RoleID:       defaultRole.ID,
		Status:       "active",
		Profile: &user.UserProfile{
			Nickname: &nickname,
		},
	}
	if req.Email != "" {
		newUser.Email = &req.Email
	}

	if err := core.DB.Create(&newUser).Error; err != nil {
		core.Logger.Error("create user failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(200, gin.H{"message": "Register success", "user_id": newUser.ID})
}

// 用户登录
func (u *UserApi) Login(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var targetUser user.User
	if err := core.DB.Where("username = ?", req.Username).First(&targetUser).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(401, gin.H{"error": "Invalid username or password"})
		} else {
			core.Logger.Error("query user failed", zap.Error(err))
			c.JSON(500, gin.H{"error": "Internal Server Error"})
		}
		return
	}

	if !hashutil.CheckPasswordHash(req.Password, targetUser.PasswordHash) {
		c.JSON(401, gin.H{"error": "Invalid username or password"})
		return
	}

	token, err := global.TokenManager.GenerateToken(targetUser.ID)
	if err != nil {
		core.Logger.Error("generate token failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}

	if err := core.DB.Preload("Role").Preload("Profile").Preload("Profile.Avatar").First(&targetUser, targetUser.ID).Error; err != nil {
		core.Logger.Error("load user with profile failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}

	respUser := buildUserInfo(&targetUser)

	c.JSON(200, LoginResponse{
		Token: token,
		User:  respUser,
	})
}

// 获取用户信息
func (u *UserApi) GetUserInfo(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var currentUser user.User
	if err := core.DB.Preload("Role").Preload("Profile").Preload("Profile.Avatar").First(&currentUser, userID).Error; err != nil {
		core.Logger.Error("get user info failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to get user info"})
		return
	}

	respUser := buildUserInfo(&currentUser)

	c.JSON(200, gin.H{"user": respUser})
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

type UpdateProfileDirectRequest struct {
	Nickname string `form:"nickname" binding:"omitempty,min=2,max=20"`
}

// UpdateProfileDirect 合并了头像上传和资料更新的接口
func (u *UserApi) UpdateProfileDirect(c *gin.Context) {
	// 1. 绑定表单数据 (Nickname)
	var req UpdateProfileDirectRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// 2. 尝试获取上传的头像文件
	fileHeader, err := c.FormFile("avatar")
	hasFile := err == nil // 如果 err 为 nil，说明有文件上传

	// 如果有文件，先进行预检查（大小限制）
	if hasFile {
		if fileHeader.Size > 5*1024*1024 { // 5MB 限制
			c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 5MB)"})
			return
		}
	}

	// 3. 开启事务
	tx := core.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 4. 处理头像上传逻辑（如果有）
	var newFileID int64
	var objectName string
	var fullURL string

	if hasFile {
		// 上传到 MinIO
		objectName, fullURL, err = storage.UploadFile(c.Request.Context(), fileHeader, "avatars")
		if err != nil {
			tx.Rollback()
			core.Logger.Error("upload avatar failed", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Upload failed"})
			return
		}

		// 创建文件记录 (直接标记为 avatar)
		newFile := file.File{
			Bucket:    global.AppConfig.MinIO.Bucket,
			ObjectKey: objectName,
			Status:    "active",
			RefType:   "avatar", // 直接关联
			RefID:     userID,   // 直接关联当前用户
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

	// 5. 准备更新 UserProfile
	// 先查询当前 Profile 以便后续处理旧头像
	var currentProfile user.UserProfile
	if err := tx.Where("user_id = ?", userID).First(&currentProfile).Error; err != nil {
		// 如果不存在 profile，可能需要创建（视业务逻辑而定，这里假设已存在或自动创建）
	}

	// 检查 Nickname 唯一性 (如果修改了)
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

	// 构造更新 Map
	updates := map[string]interface{}{}
	if req.Nickname != "" {
		updates["nickname"] = req.Nickname
	}
	if hasFile {
		updates["avatar_file_id"] = newFileID
	}

	// 执行更新
	if len(updates) > 0 {
		result := tx.Model(&user.UserProfile{}).
			Where("user_id = ?", userID).
			Updates(updates)

		if result.Error != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
			return
		}

		// 如果没有记录被更新（可能是第一次设置），则创建
		if result.RowsAffected == 0 {
			profile := user.UserProfile{
				UserID: userID,
			}
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

	// 6. 处理旧头像状态 (标记为 replaced)
	if hasFile && currentProfile.AvatarFileID != nil {
		if err := tx.Model(&file.File{}).
			Where("id = ?", *currentProfile.AvatarFileID).
			Update("status", "replaced").Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update old avatar status"})
			return
		}
	}

	// 7. 提交事务
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	// 8. 异步清理旧头像文件 (MinIO)
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

	// 9. 返回成功响应
	resp := gin.H{"message": "Profile updated successfully"}
	if hasFile {
		resp["avatar_url"] = fullURL
	}
	c.JSON(http.StatusOK, resp)
}

// 更新用户 密码
func (u *UserApi) UpdateUserPassword(c *gin.Context) {
	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	var currentUser user.User
	if err := core.DB.Select("id, password_hash").First(&currentUser, userID).Error; err != nil {
		c.JSON(404, gin.H{"error": "User not found"})
		return
	}

	if !hashutil.CheckPasswordHash(req.OldPassword, currentUser.PasswordHash) {
		c.JSON(400, gin.H{"error": "Incorrect old password"})
		return
	}

	newHash, err := hashutil.HashPassword(req.NewPassword)
	if err != nil {
		core.Logger.Error("hash password failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Internal Server Error"})
		return
	}

	if err := core.DB.Model(&user.User{}).Where("id = ?", userID).Update("password_hash", newHash).Error; err != nil {
		core.Logger.Error("update password failed", zap.Error(err))
		c.JSON(500, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(200, gin.H{"message": "Password updated successfully"})
}
