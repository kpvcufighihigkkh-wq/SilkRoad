package server

import (
	"context"
	"encoding/json"
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

// edges 表必须可下发，且**只**下发调用方自己那一行。
//
// 两条断言都是必需的：正向断言（含自己的 code）证明分支接通，负向断言
// （不含同租户设备的 code）才证明过滤真的发生。只有正向断言时，全表下发
// 也会通过 —— 那正是本测试此前漏掉的问题。
//
// 用结构化解析而非子串匹配：子串匹配无法区分「自己的行被下发」与
// 「别人的行里恰好包含这个字符串」，且无法检查字段名。
func TestHandleBaseDataPull_EdgesTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	// 两台设备，各自 IP 不同。调用方是 self，other 是邻居。
	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-self", EdgeName: "自身设备", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge self failed: %v", err)
	}
	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-neighbor", EdgeName: "邻居设备", IPAddress: "192.168.2.85",
	}); err != nil {
		t.Fatalf("CreateEdge neighbor failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-self")
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
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/edge-self/base-data?tables=edges", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	// 信封为 api.Success 包一层 data，内层再是 BaseDataPullResponse.Data
	var envelope struct {
		Data struct {
			Data map[string][]map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("响应不是合法 JSON: %v; body=%s", err, w.Body.String())
	}

	rows, ok := envelope.Data.Data["edges"]
	if !ok {
		t.Fatalf("响应缺少 edges 键: %s", w.Body.String())
	}
	// 这里刻意用 Errorf 而非 Fatalf：若用 Fatalf，行数断言会先行中断测试，
	// 使下方承重的负向断言永远得不到执行，从而看不出究竟是哪一处失效。
	if len(rows) != 1 {
		t.Errorf("edges 行数 = %d, want 1（只下发自身）; body=%s", len(rows), w.Body.String())
	}

	// 字段名是跨边界契约（Task 7 的消费者依赖），重命名必须让测试失败。
	for _, field := range []string{"id", "edge_code", "edge_name", "ip_address", "status", "version"} {
		if _, ok := rows[0][field]; !ok {
			t.Errorf("edges 行缺少字段 %q; row=%v", field, rows[0])
		}
	}

	if got := rows[0]["edge_code"]; got != "edge-self" {
		t.Errorf("edge_code = %v, want edge-self", got)
	}

	// 负向断言：不得出现邻居设备。这是本测试的承重部分。
	if strings.Contains(w.Body.String(), "edge-neighbor") {
		t.Errorf("响应泄漏了其它设备: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "192.168.2.85") {
		t.Errorf("响应泄漏了其它设备的内网 IP: %s", w.Body.String())
	}
}
