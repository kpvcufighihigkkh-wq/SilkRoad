package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api/middleware"
)

// newTestJWTAuth 构造与 Center 同款配置的认证器。
// SecretKey 与 api/center/v1 的测试保持一致，便于对照。
func newTestJWTAuth() *middleware.JWTAuth {
	return middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
}

// TestRequireUserPrincipal_RejectsDeviceToken 设备凭证不得访问管理接口。
//
// 用户 token 与 Edge token 由同一个 SecretKey 签名，jwtMiddleware 只能证明
// "签名有效"，无法区分身份类型。缺了这道校验，设备就能用自身合法的 Edge token
// 调用 POST /v1/edges/:code/token，为任意其它已注册设备签发凭证。
func TestRequireUserPrincipal_RejectsDeviceToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtAuth := newTestJWTAuth()
	deviceToken, err := jwtAuth.GenerateEdgeToken("edge-dev")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	g := router.Group("")
	g.Use((&CenterServer{jwtAuth: jwtAuth}).jwtMiddleware())
	g.Use(requireUserPrincipal())
	g.POST("/v1/edges/:code/token", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-dev/token", nil)
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403（设备凭证不得访问管理接口）; body=%s", w.Code, w.Body.String())
	}
}

// TestRequireUserPrincipal_AllowsUserToken 用户 token 必须通过校验并到达 handler。
// 防止"一刀切全拒绝"的中间件伪装成正确实现。
func TestRequireUserPrincipal_AllowsUserToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	jwtAuth := newTestJWTAuth()
	userToken, err := jwtAuth.GenerateToken("u1", "alice", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	router := gin.New()
	g := router.Group("")
	g.Use((&CenterServer{jwtAuth: jwtAuth}).jwtMiddleware())
	g.Use(requireUserPrincipal())
	g.POST("/v1/edges/:code/token", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-dev/token", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200（用户凭证应放行）; body=%s", w.Code, w.Body.String())
	}
}

// TestRequireUserPrincipal_RejectsMissingClaims 未经过 jwtMiddleware 时不得放行。
func TestRequireUserPrincipal_RejectsMissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	g := router.Group("")
	g.Use(requireUserPrincipal())
	g.POST("/v1/edges/:code/token", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/edges/edge-dev/token", nil))

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401（无 claims 不得放行）; body=%s", w.Code, w.Body.String())
	}
}

// TestNewCenterServer_RoutesConstruct 路由树必须能构造。
//
// 同一路径位置使用不同参数名（:id vs :code）会让 gin 在注册时 panic，而
// go test ./... 不会构造 CenterServer —— 只有 cmd/center-server/main.go 会。
// 此测试补上这道防线。
func TestNewCenterServer_RoutesConstruct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewCenterServer 构造失败（路由参数名冲突？）: %v", r)
		}
	}()

	// nil client 可接受：构造过程只存储它，不访问数据库
	srv := NewCenterServer(nil, "test-secret", 0)

	var edgeRoutes []string
	for _, r := range srv.router.Routes() {
		if strings.HasPrefix(r.Path, "/v1/edges") {
			edgeRoutes = append(edgeRoutes, r.Method+" "+r.Path)
		}
	}

	if len(edgeRoutes) == 0 {
		t.Fatal("/v1/edges 下没有任何路由注册")
	}
	for _, r := range edgeRoutes {
		if strings.Contains(r, ":id") {
			t.Errorf("残留 :id 参数名: %s（应为 :code）", r)
		}
	}
	t.Logf("已注册的 /v1/edges 路由: %v", edgeRoutes)
}
