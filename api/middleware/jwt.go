package middleware

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims JWT载荷
type JWTClaims struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	DeviceID string   `json:"device_id,omitempty"` // 边端特有
	jwt.RegisteredClaims
}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey       string
	ExpireDuration  time.Duration
	RefreshDuration time.Duration
}

// JWTAuth JWT认证器
type JWTAuth struct {
	config *JWTConfig
}

// NewJWTAuth 创建JWT认证器
func NewJWTAuth(config *JWTConfig) *JWTAuth {
	if config.ExpireDuration == 0 {
		config.ExpireDuration = 2 * time.Hour // 默认2小时
	}
	if config.RefreshDuration == 0 {
		config.RefreshDuration = 24 * time.Hour // 默认24小时
	}
	return &JWTAuth{
		config: config,
	}
}

// GenerateToken 生成JWT token
func (j *JWTAuth) GenerateToken(userID, username string, roles []string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.ExpireDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// GenerateEdgeToken 生成边端JWT token（带DeviceID）
func (j *JWTAuth) GenerateEdgeToken(deviceID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   deviceID,
		Username: "edge",
		Roles:    []string{"edge"},
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.ExpireDuration)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.config.SecretKey))
}

// ParseToken 解析JWT token
func (j *JWTAuth) ParseToken(tokenString string) (*JWTClaims, error) {
	// 去除 "Bearer " 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(j.config.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateToken 验证token是否有效
func (j *JWTAuth) ValidateToken(tokenString string) error {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return err
	}

	// 检查是否过期
	if claims.ExpiresAt.Before(time.Now()) {
		return errors.New("token expired")
	}

	return nil
}

// RefreshToken 刷新token
func (j *JWTAuth) RefreshToken(tokenString string) (string, error) {
	claims, err := j.ParseToken(tokenString)
	if err != nil {
		return "", err
	}

	// 检查是否在刷新期内
	if time.Until(claims.ExpiresAt.Time) > j.config.RefreshDuration {
		return "", errors.New("token not expired enough to refresh")
	}

	// 生成新token
	return j.GenerateToken(claims.UserID, claims.Username, claims.Roles)
}

// ContextKey 用于context的key类型
type ContextKey string

const (
	// ClaimsKey JWT claims在context中的key
	ClaimsKey ContextKey = "jwt_claims"
)

// GetClaimsFromContext 从context中获取JWT claims
func GetClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	claims, ok := ctx.Value(ClaimsKey).(*JWTClaims)
	return claims, ok
}

// SetClaimsToContext 将JWT claims设置到context
func SetClaimsToContext(ctx context.Context, claims *JWTClaims) context.Context {
	return context.WithValue(ctx, ClaimsKey, claims)
}
