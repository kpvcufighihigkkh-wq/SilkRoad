package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/internal/service"
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

// 数据归属严格单向：Center 不得保留批次/丝饼的写路由。
// 这七条是设计定稿前的过渡产物，保留会与「Edge 是数据所有者」相抵。
func TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes(t *testing.T) {
	router, _, _ := newRealRouter(t)

	forbidden := []struct{ method, path string }{
		{http.MethodPost, "/v1/lots"},
		{http.MethodPut, "/v1/lots/:id/status"},
		{http.MethodDelete, "/v1/lots/:id"},
		{http.MethodPost, "/v1/bobbins"},
		{http.MethodPut, "/v1/bobbins/:id/status"},
		{http.MethodPost, "/v1/bobbins/:id/print"},
		{http.MethodDelete, "/v1/bobbins/:id"},
	}
	registered := make(map[string]struct{})
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	for _, f := range forbidden {
		if _, ok := registered[f.method+" "+f.path]; ok {
			t.Errorf("Center 不应注册 %s %s（数据归属单向：Center 只读）", f.method, f.path)
		}
	}
}

// 但只读端点必须保留
func TestRealRouter_CenterKeepsReadRoutes(t *testing.T) {
	router, _, _ := newRealRouter(t)
	registered := make(map[string]struct{})
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	for _, want := range []string{
		"GET /v1/lots",
		"GET /v1/lots/:id",
		"GET /v1/bobbins",
		"GET /v1/bobbins/:id",
	} {
		if _, ok := registered[want]; !ok {
			t.Errorf("缺少只读路由 %s", want)
		}
	}
}

// Edge 必须注册丝饼的两条写路由。
//
// 断言对象取自**活路由表**（router.Routes()），不是硬编码清单：把 edge.go 里
// 那两行注册删掉，本测试即变红。没有它的话删掉后整套测试仍全绿 ——
// bobbin.status 与 label_printed 会静默退回「无写入方」的状态
// （Center 侧同类路由已在 Task 9 删除），与
// TestRealRouter_HasBaseDataRouteWithCodeParam 是同一失效形态。
//
// nil client 可接受：NewEdgeServer 只存储它，构造路由不访问数据库。
func TestRealRouter_EdgeHasBobbinWriteRoutes(t *testing.T) {
	srv := NewEdgeServer(nil, "edge-001", "http://center", "test-secret", "", 0)

	registered := make(map[string]struct{})
	for _, r := range srv.router.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}

	for _, want := range []string{
		"PUT /v1/bobbins/:id/status",
		"POST /v1/bobbins/:id/print",
	} {
		if _, ok := registered[want]; !ok {
			t.Errorf("Edge 缺少写路由 %s（删掉它 bobbin.status/label_printed 将无写入方，且无其它测试会红）", want)
		}
	}
}

// newRealRouterWithRole 用生产构造函数装配路由，签发指定角色的用户 token。
func newRealRouterWithRole(t *testing.T, role string) (*gin.Engine, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	srv := NewCenterServer(nil, "test-secret", 0)

	token, err := srv.jwtAuth.GenerateToken("u-"+role, role, []string{role})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	return srv.router, token
}

// 非管理员角色不得抵达任何 /v1/users 管理端点。
//
// 此前这些路由只挂了 requireUserPrincipal（挡设备凭证），任何登录用户
// 都能创建 admin 账号。本测试钉住「角色守卫真的挂上了」。
func TestRealRouter_NonAdminCannotReachUserManagement(t *testing.T) {
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/users", `{"username":"x","password":"y","real_name":"z","role":"viewer"}`},
		{http.MethodGet, "/v1/users", ""},
		{http.MethodGet, "/v1/users/00000000-0000-0000-0000-000000000001", ""},
		{http.MethodPut, "/v1/users/00000000-0000-0000-0000-000000000001", `{"real_name":"n","is_active":true}`},
		// 该 UUID 与下面签发的 token 的 UserID（"u-"+role）不同，即为「改他人密码」。
		// 请求体必须通过 binding 校验（new_password 有 min=6），否则会先拿到 400
		// 而根本走不到授权判定 —— 那样这条断言就不再证明角色守卫拦住了越权。
		{http.MethodPut, "/v1/users/00000000-0000-0000-0000-000000000001/password", `{"old_password":"old-secret","new_password":"new-secret"}`},
		{http.MethodDelete, "/v1/users/00000000-0000-0000-0000-000000000001", ""},
	}

	for _, role := range []string{"viewer", "operator", "inspector"} {
		t.Run(role, func(t *testing.T) {
			router, token := newRealRouterWithRole(t, role)

			for _, tc := range cases {
				t.Run(tc.method+" "+tc.path, func(t *testing.T) {
					w := httptest.NewRecorder()
					req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
					req.Header.Set("Authorization", "Bearer "+token)
					if tc.body != "" {
						req.Header.Set("Content-Type", "application/json")
					}
					router.ServeHTTP(w, req)

					if w.Code != http.StatusForbidden {
						t.Errorf("%s 访问 %s %s → %d, want 403; body=%s",
							role, tc.method, tc.path, w.Code, w.Body.String())
					}
				})
			}
		})
	}
}

// 非管理员不得注册设备或签发凭证。
func TestRealRouter_NonAdminCannotReachEdgeManagement(t *testing.T) {
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/edges", `{"edge_code":"e1","edge_name":"n","ip_address":"1.2.3.4"}`},
		{http.MethodGet, "/v1/edges", ""},
		{http.MethodPost, "/v1/edges/e1/token", ""},
	}

	router, token := newRealRouterWithRole(t, "inspector")

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+token)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			router.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("inspector 访问 %s %s → %d, want 403; body=%s",
					tc.method, tc.path, w.Code, w.Body.String())
			}
		})
	}
}

// 管理员必须能真正抵达 handler（否则「全拒绝」会伪装成正确实现）。
//
// 不断言 200 —— handler 持有 nil client 会失败，这里只要求「不是 403」。
// 但「不是 403」会被 404（路由被删）混过去，因此同时断言活路由表里确实注册了
// 这些管理路由 —— 与本文件 TestRealRouter_HasOwnProfileRoute 同一范式。
func TestRealRouter_AdminPassesRoleGuard(t *testing.T) {
	router, token := newRealRouterWithRole(t, "admin")

	for _, want := range []string{
		"GET /v1/users",
		"POST /v1/users",
		"GET /v1/users/:id",
		"PUT /v1/users/:id",
		"DELETE /v1/users/:id",
		// 该路由对所有人开放（admin 或本人），见 center.go 中的说明
		"PUT /v1/users/:id/password",
		"GET /v1/edges",
		"POST /v1/edges",
		"POST /v1/edges/:code/token",
	} {
		if !routeRegistered(router, want) {
			t.Errorf("生产路由缺少 %s（管理员无法抵达；断言「非 403」会被此处的 404 混过去）", want)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Errorf("admin 被角色守卫拦下（status=%d）; body=%s", w.Code, w.Body.String())
	}
}

// routeRegistered 报告活路由表中是否存在 method+path。
//
// 用 router.Routes() 而非硬编码清单：断言的是「真的注册了」，
// 把它删掉测试即变红。
func routeRegistered(router *gin.Engine, methodAndPath string) bool {
	for _, r := range router.Routes() {
		if r.Method+" "+r.Path == methodAndPath {
			return true
		}
	}
	return false
}

// 只读的批次/丝饼查询对所有已登录角色开放 —— 角色守卫不得误伤它们。
func TestRealRouter_AllRolesCanReadLots(t *testing.T) {
	for _, role := range []string{"viewer", "operator", "inspector", "admin"} {
		t.Run(role, func(t *testing.T) {
			router, token := newRealRouterWithRole(t, role)

			// 删除整组只读路由时，「非 403」会被 404 混过。
			for _, want := range []string{"GET /v1/lots", "GET /v1/bobbins"} {
				if !routeRegistered(router, want) {
					t.Errorf("生产路由缺少 %s（断言「非 403」会被此处的 404 混过去）", want)
				}
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/lots", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusForbidden {
				t.Errorf("%s 被拦在 /v1/lots 之外（status=403）; body=%s",
					role, w.Body.String())
			}
		})
	}
}

// /v1/users/me 必须对所有已登录角色开放：前端靠它拿自己的角色渲染菜单。
func TestRealRouter_AllRolesCanReadOwnProfile(t *testing.T) {
	for _, role := range []string{"viewer", "operator", "inspector", "admin"} {
		t.Run(role, func(t *testing.T) {
			router, token := newRealRouterWithRole(t, role)

			// 删掉该路由时 viewer 的请求会落到 admin 组内的 GET /v1/users/:id 得到 403，
			// 尚能被下面的断言捕获；但「非 403」本身仍不足以证明路由存在。
			if !routeRegistered(router, "GET /v1/users/me") {
				t.Errorf("生产路由缺少 GET /v1/users/me（断言「非 403」会被此处的 404 混过去）")
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w, req)

			if w.Code == http.StatusForbidden {
				t.Errorf("%s 被拦在 /v1/users/me 之外（status=403）; body=%s",
					role, w.Body.String())
			}
		})
	}
}

// PUT /v1/users/:id/password 允许的调用者是「admin 或本人」（设计文档 §权限矩阵）。
//
// 它此前落在 requireRole("admin") 组内，除 admin 外没有任何角色能改自己的密码 ——
// 而 admin 也改不了别人（ChangePassword 需要旧密码）。净效果是该功能对所有人不可用。
// 本测试用**生产路由** + 真实 client，因此把该路由挪回 admin 组、或在 handler 里
// 去掉「本人」分支，都会变红。
func TestRealRouter_NonAdminCanChangeOwnPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	client := newServerTestClient(t)
	svc := service.NewUserService(client)

	// 用生产构造函数装配路由，并把 client 交给它 —— 直接调 NewCenterServer 会拿到 nil client。
	srv := NewCenterServer(client, "test-secret", 0)
	// NewCenterServer 内部的 userService 与本处 svc 是同一 client 上的两个实例，
	// 因此下面用 svc 建档、用 srv 处理请求是等价的。

	const oldPwd, newPwd = "old-secret", "new-secret"

	mkUser := func(username, role string) string {
		t.Helper()
		u, err := svc.CreateUser(context.Background(), "admin", &service.CreateUserRequest{
			Username: username,
			Password: oldPwd,
			RealName: "测试用户",
			Role:     role,
		})
		if err != nil {
			t.Fatalf("CreateUser(%s) failed: %v", username, err)
		}
		return u.ID
	}

	call := func(token, targetID, oldPassword string) *httptest.ResponseRecorder {
		t.Helper()
		body := `{"old_password":"` + oldPassword + `","new_password":"` + newPwd + `"}`
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/v1/users/"+targetID+"/password",
			strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		srv.router.ServeHTTP(w, req)
		return w
	}

	// 1) 非管理员改自己的密码 → 200
	for _, role := range []string{"viewer", "operator", "inspector"} {
		t.Run("本人-"+role, func(t *testing.T) {
			ownID := mkUser("u-pwd-"+role, role)
			token, err := srv.jwtAuth.GenerateToken(ownID, "u-pwd-"+role, []string{role})
			if err != nil {
				t.Fatalf("GenerateToken failed: %v", err)
			}

			w := call(token, ownID, oldPwd)
			if w.Code != http.StatusOK {
				t.Fatalf("%s 改自己密码 → %d, want 200; body=%s", role, w.Code, w.Body.String())
			}

			// 新密码必须真的生效（否则「返回 200」可能只是空实现）
			if _, err := svc.ValidateLogin(context.Background(), "u-pwd-"+role, newPwd); err != nil {
				t.Errorf("%s 用新密码登录失败: %v", role, err)
			}
		})
	}

	// 2) 非管理员改他人密码 → 403（且不得改动目标账号的密码）
	t.Run("改他人", func(t *testing.T) {
		viewerID := mkUser("u-pwd-attacker", "viewer")
		victimID := mkUser("u-pwd-victim", "operator")

		token, err := srv.jwtAuth.GenerateToken(viewerID, "u-pwd-attacker", []string{"viewer"})
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		w := call(token, victimID, oldPwd)
		if w.Code != http.StatusForbidden {
			t.Fatalf("viewer 改他人密码 → %d, want 403; body=%s", w.Code, w.Body.String())
		}

		// 受害者原密码必须仍然有效 —— 拦下请求但不能是「先改了再报 403」
		if _, err := svc.ValidateLogin(context.Background(), "u-pwd-victim", oldPwd); err != nil {
			t.Errorf("他人密码被改动: %v", err)
		}
	})

	// 3) admin 改他人密码：路径放行到 handler（无旧密码则业务失败，但不该是 403）
	t.Run("admin不被身份拦截", func(t *testing.T) {
		adminID := mkUser("u-pwd-admin", "admin")
		otherID := mkUser("u-pwd-other", "viewer")

		token, err := srv.jwtAuth.GenerateToken(adminID, "u-pwd-admin", []string{"admin"})
		if err != nil {
			t.Fatalf("GenerateToken failed: %v", err)
		}

		w := call(token, otherID, "错误的旧密码")
		if w.Code == http.StatusForbidden {
			t.Errorf("admin 被误判为无权访问（status=403）; body=%s", w.Body.String())
		}
	})
}

// 生产路由必须包含 GET /v1/users/me，且它不得落在 admin 组内。
//
// TestRealRouter_AllRolesCanReadOwnProfile 只断言「非 403」，而该文件在
// TestRealRouter_DeviceRoutesRemainReachableByDeviceToken 中已把标准写明：
// 「不是 403」会被 404（路由被删）混过去。没有本断言时，/v1/users/me 的存在性
// 仅靠间接的参数路由遮蔽行为支撑 —— 若该行被删，viewer 请求会落到 admin 组内的
// GET /v1/users/:id 上得到 403，尚能被上面那条用例捕获；但若删的是整条路由
// 之外的其它形态，就没有任何断言会红。这里直接钉住活路由表。
func TestRealRouter_HasOwnProfileRoute(t *testing.T) {
	router, _, _ := newRealRouter(t)

	want := "GET /v1/users/me"
	for _, r := range router.Routes() {
		if r.Method+" "+r.Path == want {
			return
		}
	}

	var got []string
	for _, r := range router.Routes() {
		if strings.HasPrefix(r.Path, "/v1/users") {
			got = append(got, r.Method+" "+r.Path)
		}
	}
	t.Errorf("生产路由缺少 %s（前端靠它获取自身角色渲染菜单）；已注册: %v", want, got)
}

// 两条用例缺一不可，构成对「requireUserPrincipal 挂在 authorized 组级」的证明：
//
//  1. GET /v1/lots —— 该路由只有 requireUserPrincipal，**没有** requireRole。
//     它才是这层中间件的判别用例：删掉 center.go 的 authorized.Use(requireUserPrincipal())
//     会让它变红。只读端点如今是这层中间件唯一的专属守卫。
//  2. POST /v1/users —— 该路由同时有 requireUserPrincipal 与 requireRole("admin")。
//     设备 token 的 Roles 为 ["edge"]，会被角色允许列表拒掉，因此它**无法**单独
//     证明 requireUserPrincipal 仍挂着（双层守卫会掩盖单层的缺失）。
//     保留它是为了钉住「设备凭证进不了用户管理」这个终态。
func TestRealRouter_DeviceTokenStillBlockedFromUserRoutes(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		// 仅 requireUserPrincipal 守卫（无 requireRole）—— 判别用例
		{http.MethodGet, "/v1/lots", ""},
		// 双层守卫 —— 终态用例
		{http.MethodPost, "/v1/users", `{"username":"x","password":"y","real_name":"z","role":"admin"}`},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
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

// 报表端点必须存在，且对全部已登录角色开放、对设备凭证关闭。
//
// 三条用例各钉一件事：
//  1. 存在性 —— 路由真的注册了，而不是 handler 写了却忘了挂；
//  2. 设备凭证 —— 新路由组必须在 authorized 组内，继承组级 requireUserPrincipal；
//  3. viewer 放行 —— 报表不是管理功能，误挂 requireRole("admin") 时这里变红。
func TestRealRouter_ReportRoutesExist(t *testing.T) {
	router, _, _ := newRealRouter(t)

	paths := map[string]bool{}
	for _, r := range router.Routes() {
		paths[r.Path] = true
	}

	for _, want := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		if !paths[want] {
			t.Errorf("route %s 未注册", want)
		}
	}
}

func TestRealRouter_DeviceTokenBlockedFromReports(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	for _, path := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+deviceToken)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("设备凭证访问 %s → %d, want 403; body=%s", path, w.Code, w.Body.String())
		}
	}
}

// 断言「不是 403」而非 200：nil client 下 handler 会在 service 层失败，
// 这里要证明的只是角色守卫放行，查询成功与否由服务层测试覆盖。
func TestRealRouter_ViewerPassesReportGuard(t *testing.T) {
	router, viewerToken := newRealRouterWithRole(t, "viewer")

	for _, path := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		router.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Errorf("viewer 访问 %s 被 403 拒绝；报表应对全部已登录角色开放", path)
		}
	}
}
