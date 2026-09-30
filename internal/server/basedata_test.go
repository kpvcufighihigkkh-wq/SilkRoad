package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
	"github.com/yourusername/igh-silkroad/internal/sync/center"
)

// newBaseDataRouter 组装只挂 base-data 端点的最小路由。
//
// 路径必须是 /v1/edges/:code/base-data，与生产路由一致：
// authenticateEdge 的第三重校验拿 c.Param("code") 与 token 的 DeviceID
// 比对，路径里没有 :code 的写法永远无法通过鉴权。
func newBaseDataRouter(t *testing.T, svr *CenterServer, jwtAuth *middleware.JWTAuth) *gin.Engine {
	t.Helper()

	router := gin.New()
	router.GET("/v1/edges/:code/base-data", testEdgeAuth(jwtAuth), svr.handleBaseDataPull)
	return router
}

// base-data 端点必须真实返回已注册线体，而非空 map
func TestHandleBaseDataPull_ReturnsRealData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-bd", EdgeName: "下发测试", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	// 造一条线体数据
	if _, err := client.SpinningLine.Create().
		SetLineName("A线").
		SetLineNumber("LINE-A").
		SetCapacity(48).
		SetStatus("running").
		Save(ctx); err != nil {
		t.Fatalf("failed creating spinning line: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-bd")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	svr := &CenterServer{
		client:       client,
		edgeService:  edgeSvc,
		dataProvider: center.NewBaseDataProvider(client),
		jwtAuth:      jwtAuth,
	}
	router := newBaseDataRouter(t, svr, jwtAuth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/edge-bd/base-data?tables=spinning_lines", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "LINE-A") {
		t.Errorf("响应未包含真实线体数据: %s", w.Body.String())
	}
}

// 未注册设备必须被拒绝：base-data 不得比上传端点宽松。
//
// 单独一个测试是必要的：ReturnsRealData 即便把鉴权整段删掉也会通过
// （可信的 RemoteAddr + 正确的 :code 一样返回数据），所以只有这条能
// 证明"接在鉴权后面"这件事真的发生了。
func TestHandleBaseDataPull_RejectsUnregisteredEdge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	// 设备未注册，但持有一张签名有效的 token
	token, err := jwtAuth.GenerateEdgeToken("ghost-edge")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	svr := &CenterServer{
		client:       client,
		edgeService:  edgeSvc,
		dataProvider: center.NewBaseDataProvider(client),
		jwtAuth:      jwtAuth,
	}
	router := newBaseDataRouter(t, svr, jwtAuth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/ghost-edge/base-data?tables=spinning_lines", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (未注册设备); body=%s", w.Code, w.Body.String())
	}
	// 拒绝时不得泄漏任何数据
	if strings.Contains(w.Body.String(), "\"data\"") {
		t.Errorf("被拒响应不应包含 data 字段: %s", w.Body.String())
	}
}

// 来源 IP 与注册地址不符必须被拒绝（第二重约束）。
func TestHandleBaseDataPull_RejectsIPMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-ip", EdgeName: "IP校验", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-ip")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	svr := &CenterServer{
		client:       client,
		edgeService:  edgeSvc,
		dataProvider: center.NewBaseDataProvider(client),
		jwtAuth:      jwtAuth,
	}
	router := newBaseDataRouter(t, svr, jwtAuth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/edge-ip/base-data?tables=spinning_lines", nil)
	req.RemoteAddr = "10.0.0.9:1234" // 与注册的 192.168.2.84 不符
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (来源IP不符); body=%s", w.Code, w.Body.String())
	}
}

// edges 表必须可下发（Step 3 新增的分支）。
func TestHandleBaseDataPull_EdgesTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-src", EdgeName: "下发源", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-src")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	svr := &CenterServer{
		client:       client,
		edgeService:  edgeSvc,
		dataProvider: center.NewBaseDataProvider(client),
		jwtAuth:      jwtAuth,
	}
	router := newBaseDataRouter(t, svr, jwtAuth)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/edge-src/base-data?tables=edges", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "edge-src") {
		t.Errorf("响应未包含 edges 表数据: %s", body)
	}
	// 必须落在 edges 这个 key 下，而不是被静默丢进空 map
	if !strings.Contains(body, "\"edges\"") {
		t.Errorf("响应缺少 edges 键: %s", body)
	}
}
