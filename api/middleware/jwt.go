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

	// EdgeExpireDuration 设备凭证（Edge token）的有效期，与用户会话分开配置。
	//
	// 设备凭证签发给无人值守的产线设备，仓库里没有任何续期路径：一旦过期，
	// Edge 只能靠人工取新 token 并重启，期间同步静默停止且 /health 仍返回 ok。
	// 而它绑定的是已注册设备 + 已注册源 IP，二者在每次请求上都会被重新校验
	// （internal/server/center.go 的 authenticateEdge），影响面远小于用户会话 ——
	// 因此短有效期在这里不是安全措施，而是定时故障。
	EdgeExpireDuration time.Duration
}

// DefaultEdgeTokenTTL 设备凭证的默认有效期。
//
// 取值理由见 JWTConfig.EdgeExpireDuration：设备凭证由设备身份与源 IP 双重约束，
// 且没有续期机制，所以默认取一个能与无人值守部署兼容的长有效期。
const DefaultEdgeTokenTTL = 30 * 24 * time.Hour

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
	if config.EdgeExpireDuration == 0 {
		config.EdgeExpireDuration = DefaultEdgeTokenTTL
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

// GenerateEdgeToken 生成边端JWT token（带DeviceID）。
//
// 有效期取自 EdgeExpireDuration，与用户 token 的 ExpireDuration 相互独立 ——
// 设备无人值守且没有续期路径，不能沿用用户会话的 2 小时。
func (j *JWTAuth) GenerateEdgeToken(deviceID string) (string, error) {
	now := time.Now()
	claims := &JWTClaims{
		UserID:   deviceID,
		Username: "edge",
		Roles:    []string{"edge"},
		DeviceID: deviceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(j.config.EdgeExpireDuration)),
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
