package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// 未带 token 必须 401
func TestAuthenticateEdge_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return // authenticateEdge 已写响应
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-001/upload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (未提供凭证)", w.Code)
	}
}

// 未注册的 edge_code 必须 403（不得自动注册）
func TestAuthenticateEdge_UnregisteredCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	token, err := jwtAuth.GenerateEdgeToken("ghost-edge")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/ghost-edge/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (未注册设备)", w.Code)
	}
}

// token 身份与 URL 不一致必须 403（防 A 冒充 B）
func TestAuthenticateEdge_CodeMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-a", EdgeName: "A", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge A failed: %v", err)
	}
	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-b", EdgeName: "B", IPAddress: "192.168.2.85",
	}); err != nil {
		t.Fatalf("CreateEdge B failed: %v", err)
	}

	// 持 A 的 token 访问 B 的 URL
	token, err := jwtAuth.GenerateEdgeToken("edge-a")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-b/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (token 与 URL 不符)", w.Code)
	}
}

// IP 与注册值不符必须 403
func TestAuthenticateEdge_IPMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-ip", EdgeName: "IP测试", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-ip")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-ip/upload", nil)
	req.RemoteAddr = "10.9.9.9:1234" // 与注册的 192.168.2.84 不符
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (IP 不符)", w.Code)
	}
}

// 四重校验全通过必须放行
func TestAuthenticateEdge_AllChecksPass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	created, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-ok", EdgeName: "通过", IPAddress: "192.168.2.84",
	})
	if err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-ok")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	var gotID string
	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		edgeRow, err := authenticateEdge(c, edgeSvc)
		if err != nil {
			return
		}
		gotID = edgeRow.ID
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-ok/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotID != created.ID {
		t.Errorf("推导出的 ID = %q, want %q", gotID, created.ID)
	}
}

// newTestCenterServer 构造可直连内存库的 CenterServer。
//
// 刻意走生产路径 NewCenterServer，而不是手搓 struct —— 缺陷正藏在
// uploadHandler 与 edgeService 的接线缝里，手搓会把这个缝绕过去。
func newTestCenterServer(t *testing.T, client *ent.Client) *CenterServer {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	return NewCenterServer(client, "test-secret", 0)
}

// lotEntry 构造一条 lots 表的 create 记录。
func lotEntry(lotID uuid.UUID, lotNumber string) map[string]interface{} {
	return map[string]interface{}{
		"table":     "lots",
		"operation": "create",
		"id":        lotID.String(),
		"data": map[string]interface{}{
			"id":               lotID.String(),
			"lot_number":       lotNumber,
			"product_type":     "FDY",
			"planned_quantity": 100,
			"actual_quantity":  0,
			"status":           "in_progress",
		},
	}
}

// uploadRequest 构造一个通过四重校验的上传请求。
func uploadRequest(t *testing.T, srv *CenterServer, code string, entries ...map[string]interface{}) *http.Request {
	t.Helper()

	token, err := srv.jwtAuth.GenerateEdgeToken(code)
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	body, err := json.Marshal(map[string]interface{}{
		"edge_id": "payload-must-be-ignored",
		"cursor":  42,
		"entries": entries,
	})
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/edges/"+code+"/upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// TestHandleEdgeUpload_ActuallyApplies 上传端点必须真的落库。
//
// 原实现返回 {"applied": len(entries), "rejected": 0} 却从未调用 uploadHandler，
// Edge 依据这个响应标记已同步，数据被静默丢弃。这里断言响应与数据库的真实状态一致。
func TestHandleEdgeUpload_ActuallyApplies(t *testing.T) {
	client := newServerTestClient(t)
	ctx := context.Background()

	srv := newTestCenterServer(t, client)

	code := "edge-apply"
	if _, err := srv.edgeService.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: code, EdgeName: "落库测试", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	lotID := uuid.New()
	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, uploadRequest(t, srv, code, lotEntry(lotID, "LOT-APPLY-001")))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Applied int `json:"applied"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是合法 JSON: %v; body=%s", err, w.Body.String())
	}
	if resp.Data.Applied != 1 {
		t.Errorf("applied = %d, want 1", resp.Data.Applied)
	}

	// 决定性断言：数据必须真的在库里，而不只是响应里说成功
	lot, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("批次未落库（假成功响应）: %v", err)
	}
	if lot.LotNumber != "LOT-APPLY-001" {
		t.Errorf("lot_number = %q, want %q", lot.LotNumber, "LOT-APPLY-001")
	}
}

// TestHandleEdgeUpload_RejectsMalformedEntryID 非法 entry id 必须 400，不得静默跳过。
//
// 放过的话客户端只会看到一个 rejected 计数，拿不到「请求本身是坏的」这一信号。
func TestHandleEdgeUpload_RejectsMalformedEntryID(t *testing.T) {
	client := newServerTestClient(t)
	ctx := context.Background()

	srv := newTestCenterServer(t, client)

	code := "edge-bad"
	if _, err := srv.edgeService.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: code, EdgeName: "坏请求", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	entry := lotEntry(uuid.New(), "LOT-BAD-001")
	entry["id"] = "not-a-uuid"

	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, uploadRequest(t, srv, code, entry))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (畸形 entry id); body=%s", w.Code, w.Body.String())
	}
}

// TestHandleEdgeUpload_UpdatesHeartbeat 鉴权成功的上传必须刷新设备心跳。
func TestHandleEdgeUpload_UpdatesHeartbeat(t *testing.T) {
	client := newServerTestClient(t)
	ctx := context.Background()

	srv := newTestCenterServer(t, client)

	code := "edge-hb"
	if _, err := srv.edgeService.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: code, EdgeName: "心跳", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	before, err := srv.edgeService.GetEdgeByCode(ctx, code)
	if err != nil {
		t.Fatalf("GetEdgeByCode failed: %v", err)
	}
	if before.Status != "offline" {
		t.Fatalf("前置状态 = %q, want %q", before.Status, "offline")
	}

	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, uploadRequest(t, srv, code, lotEntry(uuid.New(), "LOT-HB-001")))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	after, err := srv.edgeService.GetEdgeByCode(ctx, code)
	if err != nil {
		t.Fatalf("GetEdgeByCode after failed: %v", err)
	}
	if after.Status != "online" {
		t.Errorf("status = %q, want %q (心跳未刷新)", after.Status, "online")
	}
	if after.LastSeen == before.LastSeen {
		t.Errorf("last_seen 未更新: %q", after.LastSeen)
	}
}

// TestHandleEdgeUpload_RejectsUnregisteredDevice 未注册设备的凭证不得上传。
func TestHandleEdgeUpload_RejectsUnregisteredDevice(t *testing.T) {
	client := newServerTestClient(t)
	ctx := context.Background()

	srv := newTestCenterServer(t, client)

	if _, err := srv.edgeService.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-reg", EdgeName: "已注册", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, uploadRequest(t, srv, "ghost"))

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (未注册设备不得上传); body=%s", w.Code, w.Body.String())
	}
}

// TestAuthenticateEdge_RejectsUserToken 用户 token 必须被拒绝。
//
// 用户 token 由同一个 SecretKey 签名，因此能通过 jwtMiddleware ——
// 中间件只能证明「签名有效」，无法区分身份类型。它带着 DeviceID == ""
// 抵达 authenticateEdge，必须被第一重校验拦下：否则任何登录用户都能
// 冒充设备上传数据。
//
// 断言 401 而非 403 是有意的：401 表示「没有设备凭证」，
// 403 表示「设备凭证不对」。用户 token 属于前者 —— 越权的用户身份
// 压根不该出现在设备通道上，而非「设备身份不匹配」。
func TestAuthenticateEdge_RejectsUserToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	jwtAuth := newTestJWTAuth()
	edgeSvc := service.NewEdgeService(client)

	// 签名有效、但 DeviceID 为空的用户 token
	userToken, err := jwtAuth.GenerateToken("u1", "alice", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	var ran bool
	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		ran = true
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-any/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(w, req)

	if !ran {
		t.Fatal("authenticateEdge 未被调用 —— 中间件不应拦截签名有效的用户 token")
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401（用户 token 不含设备凭证）; body=%s", w.Code, w.Body.String())
	}
}

// TestAuthenticateEdge_RejectsMissingClaims 未经认证中间件时不得放行。
//
// 覆盖 claims 根本不在上下文里的分支：与「claims 在但 DeviceID 为空」
// 是两条不同的路径，都必须 fail closed。
func TestAuthenticateEdge_RejectsMissingClaims(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	edgeSvc := service.NewEdgeService(client)

	router := gin.New()
	// 故意不挂 testEdgeAuth：上下文里没有 claims
	router.POST("/v1/edges/:code/upload", func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-any/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401（上下文中无 claims）; body=%s", w.Code, w.Body.String())
	}
}

// TestHandleEdgeUpload_PassesUUIDNotEdgeCode 传递给 uploadHandler 的权威身份
// 必须能按 UUID 解析。
//
// lot.edge_id 声明为 field.UUID("edge_id", uuid.UUID{})
// （internal/database/ent/schema/lot.go:35），指向 edges.id 的外键；
// 消费方 handler.go 用 uuid.Parse 解析 UploadRequest.EdgeID。
// 若传 EdgeCode（如 "edge-001"），uuid.Parse 报 "invalid UUID length: 8"，
// 消费方退化为 uuid.Nil，SetEdgeID 的条件分支永不触发，外键恒为 NULL ——
// 与「身份被伪造」在观测上无法区分。
//
// 本测试通过捕获 handler.go 的上传日志来观测实际传入的值：
// handleEdgeUpload 把身份写进 UploadRequest，而 HandleUpload 开头就
// 打印 edge_id=%s，这是 Task 5 落地前唯一可观测该值的接缝。
func TestHandleEdgeUpload_PassesUUIDNotEdgeCode(t *testing.T) {
	client := newServerTestClient(t)
	ctx := context.Background()

	srv := newTestCenterServer(t, client)

	// 刻意使用无法被 uuid.Parse 接受的设备编码
	const code = "edge-001"
	created, err := srv.edgeService.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: code, EdgeName: "UUID断言", IPAddress: "192.168.2.84",
	})
	if err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	// 前置校验：本测试依赖 EdgeCode 不是合法 UUID
	if _, err := uuid.Parse(code); err == nil {
		t.Fatalf("EdgeCode %q 居然是合法 UUID，本测试失去意义", code)
	}
	if _, err := uuid.Parse(created.ID); err != nil {
		t.Fatalf("EdgeResponse.ID = %q 不是合法 UUID: %v", created.ID, err)
	}

	// 捕获标准日志（gin 的请求日志走 gin.DefaultWriter，不会被截获）
	var buf safeBuffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	w := httptest.NewRecorder()
	srv.router.ServeHTTP(w, uploadRequest(t, srv, code, lotEntry(uuid.New(), "LOT-UUID-001")))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	got := extractLoggedEdgeID(buf.String())
	if got == "" {
		t.Fatalf("未能从日志中提取 edge_id（handler.go 的日志格式变了？）\n日志: %s", buf.String())
	}
	if got != created.ID {
		t.Errorf("传入 uploadHandler 的 EdgeID = %q, want %q（设备 UUID，而非 EdgeCode）", got, created.ID)
	}
	if _, err := uuid.Parse(got); err != nil {
		t.Errorf("传入 uploadHandler 的 EdgeID = %q 无法按 UUID 解析: %v", got, err)
	}
}

// safeBuffer 是并发安全的日志缓冲。
// 标准 log 包自带互斥锁，但测试读取时仍可能与写入竞争，
// gin 的中间件亦可能在别的 goroutine 写入。
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// extractLoggedEdgeID 从上传日志中取出 handler.go 打印的 edge_id 值。
func extractLoggedEdgeID(logs string) string {
	const marker = "edge_id="
	idx := strings.Index(logs, marker)
	if idx < 0 {
		return ""
	}
	rest := logs[idx+len(marker):]
	if end := strings.IndexAny(rest, ", \n"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}
