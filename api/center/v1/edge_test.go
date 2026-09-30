package v1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

func setupEdgeTest(t *testing.T) (*gin.Engine, *ent.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := setupTestClientShared(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})

	h := centerv1.NewEdgeHandler(service.NewEdgeService(client), jwtAuth)

	router := gin.New()
	router.POST("/v1/edges", h.CreateEdge)
	router.POST("/v1/edges/:code/token", h.IssueToken)

	return router, client
}

func TestEdgeHandler_IssueToken_ReturnsDeviceToken(t *testing.T) {
	router, client := setupEdgeTest(t)
	ctx := context.Background()

	if _, err := service.NewEdgeService(client).CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-tok",
		EdgeName:  "签发测试",
		IPAddress: "192.168.2.90",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-tok/token", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "token") {
		t.Errorf("响应应包含 token，实际: %s", w.Body.String())
	}
}

func TestEdgeHandler_IssueToken_UnregisteredRejected(t *testing.T) {
	router, _ := setupEdgeTest(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/no-such-edge/token", nil)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("未注册设备不应签发 token，实际 status = %d", w.Code)
	}
}
