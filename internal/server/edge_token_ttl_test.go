package server

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/yourusername/igh-silkroad/api/middleware"
)

// 设备凭证的有效期必须与用户会话分开，且默认足够长。
//
// 修复前 GenerateEdgeToken 用的是 ExpireDuration（Center 配成 2 小时），
// 而设备凭证没有任何续期路径：过期后 Edge 只能人工取新 token 并重启，
// 期间同步静默停止、/health 仍返回 ok。短有效期在这里不是安全措施，而是定时故障。
func TestGenerateEdgeToken_UsesOwnTTLNotUserTTL(t *testing.T) {
	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:          "test-secret",
		ExpireDuration:     2 * time.Hour,
		EdgeExpireDuration: 30 * 24 * time.Hour,
	})

	token, err := jwtAuth.GenerateEdgeToken("edge-001")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	claims, err := jwtAuth.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	// 必须接近 EdgeExpireDuration，而不是 ExpireDuration
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < 29*24*time.Hour {
		t.Errorf("设备凭证有效期 = %s, want ≈30 天（仍在沿用用户会话的 2 小时？）", ttl)
	}
	if ttl > 31*24*time.Hour {
		t.Errorf("设备凭证有效期 = %s，超出预期上限", ttl)
	}
}

// 未显式配置时必须落到 DefaultEdgeTokenTTL，而不是沿用用户会话的默认值。
func TestGenerateEdgeToken_DefaultTTL(t *testing.T) {
	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey: "test-secret",
		// 刻意不设置 EdgeExpireDuration
	})

	token, err := jwtAuth.GenerateEdgeToken("edge-001")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	claims, err := jwtAuth.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl < middleware.DefaultEdgeTokenTTL-time.Minute {
		t.Errorf("默认设备凭证有效期 = %s, want ≈%s", ttl, middleware.DefaultEdgeTokenTTL)
	}
}

// 已过期的设备凭证必须被拒；同一凭证在长 TTL 下仍可用。
//
// 这里手工构造一个 ExpiresAt 已过去的凭证，而不是用很短的 TTL 后 sleep ——
// jwt.NewNumericDate 会把时间截断到整秒，亚秒级 TTL 可能被截断到「早于现在」
// 而立即过期，那样的测试不可靠。手工构造是确定性的，且同样证明「过期即拒」。
//
// 对应的生产故障：Center 把设备凭证配成 2 小时，过期后 Edge 无处续期，
// 只能人工换 token 重启。延长 TTL 后同样的凭证仍在有效期内 —— 这就是本轮的修复。
func TestEdgeToken_ExpiryEnforced(t *testing.T) {
	const secret = "test-secret"

	// 用与生产相同的密钥手工签一个已过期的设备凭证
	expiredClaims := &middleware.JWTClaims{
		UserID:   "edge-001",
		Username: "edge",
		Roles:    []string{"edge"},
		DeviceID: "edge-001",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-3 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-3 * time.Hour)),
		},
	}
	expiredToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed signing expired token: %v", err)
	}

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:          secret,
		EdgeExpireDuration: middleware.DefaultEdgeTokenTTL,
	})

	if _, err := jwtAuth.ParseToken(expiredToken); err == nil {
		t.Error("已过期的设备凭证仍被接受")
	}

	// 未过期的凭证在同一认证器下必须可用
	fresh, err := jwtAuth.GenerateEdgeToken("edge-001")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}
	if _, err := jwtAuth.ParseToken(fresh); err != nil {
		t.Errorf("新建的设备凭证被拒: %v", err)
	}
}

// EDGE_TOKEN_TTL 的非法取值必须回退到默认，不能让设备永久 401。
func TestEdgeTokenTTLFromEnv(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{"未配置", "", middleware.DefaultEdgeTokenTTL},
		{"合法值-天", "168h", 168 * time.Hour},
		{"合法值-小时", "24h", 24 * time.Hour},
		{"零", "0", middleware.DefaultEdgeTokenTTL},
		{"零秒", "0s", middleware.DefaultEdgeTokenTTL},
		{"负数", "-1h", middleware.DefaultEdgeTokenTTL},
		{"无法解析", "abc", middleware.DefaultEdgeTokenTTL},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("EDGE_TOKEN_TTL", tc.raw)

			got := edgeTokenTTLFromEnv()
			if got != tc.want {
				t.Errorf("edgeTokenTTLFromEnv(%q) = %s, want %s", tc.raw, got, tc.want)
			}
			if got <= 0 {
				t.Fatalf("返回非正的有效期 %s：会让签发的凭证立即过期", got)
			}
		})
	}
}
