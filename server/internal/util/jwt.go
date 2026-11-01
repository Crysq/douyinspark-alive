package util

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// 令牌用途：完整令牌 / 待激活令牌（只能兑换卡密）。
const (
	ScopeFull     = "full"
	ScopeActivate = "activate"
)

// Claims 控制台登录态。
type Claims struct {
	UserID uint   `json:"uid"`
	Role   string `json:"role"`
	Scope  string `json:"scope"`
	JTI    string `json:"jti"`
	jwt.RegisteredClaims
}

// SignToken 签发访问令牌。
func SignToken(secret string, userID uint, role, scope string, ttl time.Duration) (string, time.Time, error) {
	expires := time.Now().Add(ttl)
	claims := Claims{
		UserID: userID,
		Role:   role,
		Scope:  scope,
		JTI:    uuid.NewString(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "doyinspark",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	return token, expires, err
}

// ParseToken 校验并解析令牌。
func ParseToken(secret, raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("令牌无效")
	}
	if claims.Scope == "" {
		claims.Scope = ScopeFull
	}
	return claims, nil
}
