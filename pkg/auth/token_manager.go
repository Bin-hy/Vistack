package auth

import (
	"errors"
	"fmt"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
)

type TokenManager struct {
	secret         string
	expire_seconds uint64
}

func NewTokenManager(secret string, expire_seconds uint64) *TokenManager {
	return &TokenManager{
		secret:         secret,
		expire_seconds: expire_seconds,
	}
}

func (tm *TokenManager) GenerateToken(userID int64) (string, error) {
	// 使用自定义 Claims 生成 token
	claims := NewClaims(userID, tm.expire_seconds)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tm.secret))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (tm *TokenManager) ValidateToken(token string) (Claims, error) {
	// 使用自定义 Claims 进行解析与校验
	var claims Claims
	parsed, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) {
		// 确认签名算法为 HMAC（例如 HS256）
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(tm.secret), nil
	})
	if err != nil {
		return Claims{}, err
	}

	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}

	// claims.Valid 已在解析流程中调用
	return claims, nil
}

func GetClaim(ctx *gin.Context) (Claims, error) {
	if claims, exsist := ctx.Get("claims"); exsist {
		return claims.(Claims), nil
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
