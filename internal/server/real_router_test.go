package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newRealRouter 用生产构造函数装配路由，返回 router 与认证器。
//
// 必须走 NewCenterServer 而不是自建 gin.New()：只有这样才能验证
// 「中间件真的挂在了它该在的位置」，以及路由路径与生产一致。
// TRUSTED_PROXIES 设为只信回环，与 edge_auth_test.go 的既有做法一致。
func newRealRouter(t *testing.T) (*gin.Engine, string, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	// nil client 可接受：构造过程只存储它，不访问数据库
	srv := NewCenterServer(nil, "test-secret", 0)

	deviceToken, err := srv.jwtAuth.GenerateEdgeToken("edge-dev")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}
	userToken, err := srv.jwtAuth.GenerateToken("u1", "alice", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	return srv.router, deviceToken, userToken
}

// Critical 2：持有合法设备凭证不得访问任何用户管理接口。
//
// requireUserPrincipal 必须挂在 authorized 组本身。挂在子组上时，新增一个
// 管理组只要忘记加那一行，设备凭证就能调用它 —— 具体到 POST /v1/users，
// 可以创建 role=admin 的账号（UserService.CreateUser 不校验 role），
// 从而把设备凭证洗成真正的用户 JWT，拿到 Center 管理员权限。
//
// 本测试走**生产路由**，因此删掉组级的 requireUserPrincipal 会让它变红；
// 既有的 TestRequireUserPrincipal_* 只自建路由，钉不住「挂在哪里」。
func TestRealRouter_DeviceTokenCannotReachUserRoutes(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/users", `{"username":"x","password":"y","role":"admin"}`},
		{http.MethodGet, "/v1/users", ""},
		{http.MethodPut, "/v1/users/00000000-0000-0000-0000-000000000001", `{"role":"admin"}`},
		{http.MethodDelete, "/v1/users/00000000-0000-0000-0000-000000000001", ""},
		// 同一把钥匙也保护 /v1/edges 的管理路由与其它管理组
		{http.MethodPost, "/v1/edges/edge-dev/token", ""},
		{http.MethodPost, "/v1/lots", `{}`},
		{http.MethodPost, "/v1/bobbins", `{}`},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body *strings.Reader
			if tc.body == "" {
				body = strings.NewReader("")
			} else {
				body = strings.NewReader(tc.body)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Header.Set("Authorization", "Bearer "+deviceToken)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("设备凭证访问 %s %s → %d, want 403; body=%s",
					tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}
}

// 用户凭证必须仍能通过这些路由抵达 handler（否则「全拒绝」会伪装成正确实现）。
//
// 不断言 200 —— handler 会访问 nil client 而失败，这里只要求「不是 403」，
// 即确实是 requireUserPrincipal 放行、由后续处理失败。
func TestRealRouter_UserTokenPassesPrincipalGuard(t *testing.T) {
	router, _, userToken := newRealRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Errorf("用户凭证被 requireUserPrincipal 拦下（status=%d）; body=%s", w.Code, w.Body.String())
	}
	if w.Code == http.StatusUnauthorized {
		t.Errorf("用户凭证未通过 jwtMiddleware（status=%d）; body=%s", w.Code, w.Body.String())
	}
}

// 设备数据路由必须仍可被设备凭证访问。
//
// requireUserPrincipal 挂到 authorized 组后，这两个端点必须留在独立的 edge 组里，
// 否则设备会被自己的凭证锁在门外 —— 上传与基础数据下发全部 403。
func TestRealRouter_DeviceRoutesRemainReachableByDeviceToken(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	cases := []struct {
		method string
		path   string
		// wantRoute 该请求必须匹配到的生产路由模板
		wantRoute string
	}{
		{http.MethodPost, "/v1/edges/edge-dev/upload", "/v1/edges/:code/upload"},
		{http.MethodGet, "/v1/edges/edge-dev/base-data", "/v1/edges/:code/base-data"},
	}

	// 先确认路由确实注册了。只断言「不是 403」会被 404 混过去 ——
	// 删掉整个 edge 组时两个用例都会静默通过。
	registered := make(map[string]bool)
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = true
	}
	for _, tc := range cases {
		if !registered[tc.method+" "+tc.wantRoute] {
			t.Errorf("生产路由缺少 %s %s（设备凭证无法访问）", tc.method, tc.wantRoute)
		}
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader("{}"))
			req.Header.Set("Authorization", "Bearer "+deviceToken)
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// 404 = 路由不存在，403 = 被身份守卫拦下。两者都说明设备进不来，
			// 且都不是本用例想看到的「凭证有效、路由可达」。
			if w.Code == http.StatusNotFound {
				t.Errorf("路由不存在（status=404）：%s %s", tc.method, tc.path)
			}
			if w.Code == http.StatusForbidden {
				t.Errorf("设备凭证被拦在上传/下发路由之外（status=%d）; body=%s",
					w.Code, w.Body.String())
			}
			if w.Code == http.StatusUnauthorized {
				t.Errorf("设备凭证未通过 jwtMiddleware（status=%d）; body=%s",
					w.Code, w.Body.String())
			}

			// 必须真的抵达 handler。这些 handler 持有 nil client，抵达后会 panic
			// 并被 gin 的 Recovery 兜成 500 —— 那正是「已到达」的证据。
			// 断言 500 而非「不是 403」：后者会被 404（路由被删）混过去。
			if w.Code != http.StatusInternalServerError {
				t.Errorf("%s %s → %d, want 500（nil client panic = 已抵达 handler）; body=%s",
					tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}
}

// Item 4：生产路由必须包含 GET /v1/edges/:code/base-data。
//
// basedata_test.go 的四个测试各自搭路由，因此把 center.go 的路径改回
// "/base-data" 会让整个测试套件仍然全绿 —— 而少了 :code，authenticateEdge
// 的第三重校验拿到的 c.Param("code") 恒为空串，每台合法设备都会 403。
func TestRealRouter_HasBaseDataRouteWithCodeParam(t *testing.T) {
	router, _, _ := newRealRouter(t)

	want := "/v1/edges/:code/base-data"
	for _, r := range router.Routes() {
		if r.Method == http.MethodGet && r.Path == want {
			return
		}
	}

	var got []string
	for _, r := range router.Routes() {
		if strings.HasPrefix(r.Path, "/v1/edges") {
			got = append(got, r.Method+" "+r.Path)
		}
	}
	t.Errorf("生产路由缺少 GET %s（:code 缺失会让鉴权第三重对每台设备都失败）; 已注册: %v",
		want, got)
}

// Item 4：NewCenterServer 必须调用 applyTrustedProxies。
//
// 删掉 center.go 的 applyTrustedProxies 后 gin 会退回**信任所有代理**的默认值，
// 于是 X-Forwarded-For 可被任意伪造，而现有测试全部自建 router（不经过
// NewCenterServer），删掉那行不会有任何测试变红。这里走真实 router：
// 从一个未被信任的源地址发起、带伪造 XFF 的请求，ClientIP() 必须仍是源地址。
func TestRealRouter_IgnoresForgedXFFFromUntrustedSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	srv := NewCenterServer(nil, "test-secret", 0)

	var seenClientIP, seenRawXFF string
	srv.router.GET("/__probe/client-ip", func(c *gin.Context) {
		seenClientIP = c.ClientIP()
		seenRawXFF = c.GetHeader("X-Forwarded-For")
		c.Status(http.StatusOK)
	})

	const forged = "203.0.113.77"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/__probe/client-ip", nil)
	req.RemoteAddr = "198.51.100.9:5555" // 未被信任的源地址
	req.Header.Set("X-Forwarded-For", forged)
	srv.router.ServeHTTP(w, req)

	if seenRawXFF != forged {
		t.Fatalf("探针未收到伪造头（got %q），测试本身无效", seenRawXFF)
	}
	if seenClientIP == forged {
		t.Errorf("ClientIP() = %q，接受了未被信任来源的 X-Forwarded-For —— "+
			"applyTrustedProxies 未生效（gin 默认信任所有代理）", seenClientIP)
	}
	if seenClientIP != "198.51.100.9" {
		t.Errorf("ClientIP() = %q, want %q（真实源地址）", seenClientIP, "198.51.100.9")
	}
}
