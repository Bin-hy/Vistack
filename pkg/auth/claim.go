package auth

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 为自定义 JWT 负载，内嵌标准注册声明（exp/iss 等由 jwt v5 校验）
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// NewClaims 便捷构造：基于用户 ID 和有效期秒数生成 Claims
func NewClaims(userID int64, ttlSeconds uint64) Claims {
	c := Claims{UserID: userID}
	if ttlSeconds > 0 {
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Second * time.Duration(ttlSeconds)))
	}
	return c
}

// GetClaim 从 gin 上下文中提取已解析的 Claims
func GetClaim(ctx *gin.Context) (Claims, error) {
	if claims, exist := ctx.Get("claims"); exist {
		if c, ok := claims.(Claims); ok {
			return c, nil
		}
	}
	return Claims{}, errors.New("claims not found")
}

// GetUserID 从 gin 上下文中提取自定义 Claims 的 UserID
func GetUserID(ctx *gin.Context) int64 {
	if c, err := GetClaim(ctx); err == nil {
		return c.UserID
	}
	return 0
}
