package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenManager 负责签发 RS256 JWT 并生成 JWKS 公钥（仅 auth 服务持有私钥）。
type TokenManager struct {
	privateKey *rsa.PrivateKey
	kid        string
	issuer     string
	expire     time.Duration
}

// NewTokenManager 解析 PEM 私钥并构造签发器。
// privateKeyPEM 支持 PKCS1 与 PKCS8 格式；kid/issuer 为空时使用默认值；expire<=0 默认 1 小时。
func NewTokenManager(privateKeyPEM []byte, kid, issuer string, expire time.Duration) (*TokenManager, error) {
	key, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	if kid == "" {
		kid = "vistack-rs256"
	}
	if issuer == "" {
		issuer = "vistack"
	}
	if expire <= 0 {
		expire = time.Hour
	}
	return &TokenManager{privateKey: key, kid: kid, issuer: issuer, expire: expire}, nil
}

// GenerateToken 签发 RS256 签名的 JWT，claims 含 user_id / exp / iss，header 含 kid。
func (tm *TokenManager) GenerateToken(userID int64) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tm.expire)),
			Issuer:    tm.issuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = tm.kid
	return token.SignedString(tm.privateKey)
}

// ValidateToken 用私钥对应的公钥直接验签（auth 服务内部受保护路由使用，不依赖 JWKS 网络自引用）。
func (tm *TokenManager) ValidateToken(tokenStr string) (Claims, error) {
	var claims Claims
	parsed, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return &tm.privateKey.PublicKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	return claims, nil
}

// PublicJWKS 返回公钥的 JWKS JSON（含 kid / kty / n / e）。
func (tm *TokenManager) PublicJWKS() ([]byte, error) {
	pub := &tm.privateKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": tm.kid,
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			},
		},
	}
	return json.Marshal(jwks)
}

// parseRSAPrivateKey 解析 PEM 编码的 RSA 私钥（PKCS1 / PKCS8）。
func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("key is not RSA")
	}
	return nil, fmt.Errorf("unsupported private key format (need PKCS1 or PKCS8)")
}
