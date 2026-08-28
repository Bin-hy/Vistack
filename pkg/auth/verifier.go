package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenValidator 定义验签接口；TokenManager（私钥验签）与 TokenVerifier（JWKS 验签）均实现。
type TokenValidator interface {
	ValidateToken(token string) (Claims, error)
}

// TokenVerifier 通过 JWKS 公钥本地验签 RS256 JWT（不持有私钥、不回调签发服务）。
type TokenVerifier struct {
	jwksURL  string
	keyCache map[string]*rsa.PublicKey // kid -> 公钥
	mu       sync.RWMutex
}

// jwksResponse / jwk 描述 JWKS 端点返回结构
type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// NewTokenVerifier 构造验签器；首次验签时会拉取 JWKS。
func NewTokenVerifier(jwksURL string) *TokenVerifier {
	return &TokenVerifier{jwksURL: jwksURL, keyCache: make(map[string]*rsa.PublicKey)}
}

// StartAutoRefresh 后台周期刷新 JWKS 公钥（支持密钥轮换：新 kid 在下一轮生效）。
func (tv *TokenVerifier) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = tv.refreshKeys(ctx)
			}
		}
	}()
}

// ValidateToken 校验 JWT 签名与过期时间，返回 Claims。
// 采用 RS256；按 header 的 kid 选择公钥，kid 未命中时触发一次 JWKS 刷新。
func (tv *TokenVerifier) ValidateToken(tokenStr string) (Claims, error) {
	var claims Claims
	parsed, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		key := tv.getKey(kid)
		if key == nil {
			// 未命中：尝试刷新一次后再取（例如密钥轮换后的首个请求）
			_ = tv.refreshKeys(context.Background())
			key = tv.getKey(kid)
		}
		if key == nil {
			return nil, fmt.Errorf("unknown kid: %s", kid)
		}
		return key, nil
	}, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return Claims{}, err
	}
	if !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}
	return claims, nil
}

// getKey 从缓存读取公钥
func (tv *TokenVerifier) getKey(kid string) *rsa.PublicKey {
	tv.mu.RLock()
	defer tv.mu.RUnlock()
	return tv.keyCache[kid]
}

// refreshKeys 拉取 JWKS 并重建 kid->公钥 缓存
func (tv *TokenVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tv.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var jwks jwksResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return err
	}

	m := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		if e == 0 || len(nBytes) == 0 {
			continue
		}
		m[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
	}

	tv.mu.Lock()
	tv.keyCache = m
	tv.mu.Unlock()
	return nil
}
