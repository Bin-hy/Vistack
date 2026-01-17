package auth

import (
	"errors"
	"time"
)

// Claims 为自定义的 JWT 负载
// 实现 jwt-go 的 Claims 接口 (Valid 方法)
type Claims struct {
	UserID int64 `json:"user_id"`
	Exp    int64 `json:"exp"`
}

// Valid 在解析阶段进行过期校验
func (c Claims) Valid() error {
	if c.Exp == 0 {
		// 未设置过期时间，视为不过期
		return nil
	}
	if time.Unix(c.Exp, 0).Before(time.Now()) {
		return errors.New("token expired")
	}
	return nil
}

// NewClaims 便捷构造：基于用户 ID 和有效期秒数生成 Claims
func NewClaims(userID int64, ttlSeconds uint64) Claims {
	var exp int64
	if ttlSeconds > 0 {
		exp = time.Now().Add(time.Second * time.Duration(ttlSeconds)).Unix()
	}
	return Claims{UserID: userID, Exp: exp}
}
