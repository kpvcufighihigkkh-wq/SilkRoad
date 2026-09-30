package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 未配置时不得全信任代理：伪造的 XFF 必须被忽略
func TestTrustedProxies_DefaultRejectsSpoofedXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	req.Header.Set("X-Forwarded-For", "192.168.2.84")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "203.0.113.7" {
		t.Errorf("ClientIP = %q, want %q (伪造的 XFF 必须被忽略)", seen, "203.0.113.7")
	}
}

// 配置了代理后，来自该代理的 XFF 必须被采信
func TestTrustedProxies_ConfiguredProxyAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "203.0.113.0/24")

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.9:1234" // 来自可信代理
	req.Header.Set("X-Forwarded-For", "192.168.2.84")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "192.168.2.84" {
		t.Errorf("ClientIP = %q, want %q", seen, "192.168.2.84")
	}
}

// 经可信代理转发时，攻击者塞在左侧的伪造值必须被忽略（validateHeader 右往左扫描）
func TestTrustedProxies_SpoofedLeftEntryIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "203.0.113.0/24")

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	// 攻击者在最左侧伪造他人 IP，代理把真实 IP 追加到最右
	req.Header.Set("X-Forwarded-For", "192.168.2.84, 192.168.5.99")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "192.168.5.99" {
		t.Errorf("ClientIP = %q, want %q (必须取代理追加的最右值)", seen, "192.168.5.99")
	}
}
