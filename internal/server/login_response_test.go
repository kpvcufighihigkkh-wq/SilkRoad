package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// 登录路由必须注册，且请求能真正抵达 handler。
//
// 断言对象取自**活路由表**（router.Routes()），不是硬编码清单：删掉
// registerRoutes 里的 v1.POST("/login", ...) 时用例必须变红。
//
// 不用「POST 一个必然失败的登录、断言响应码 != 404」这种写法。传入 nil client
// 时，handleLogin → ValidateLogin → GetUserByUsername 会在 s.client.User.Query()
// 处解引用 nil（internal/service/user.go:137）并 panic，gin 的 Recovery() 把它
// 转成 500，于是「500 != 404」意外成立。那样一来，登录路径上任何真实的 panic
// 都会被静默吞成通过 —— 测试失去区分能力。
func TestLogin_RouteExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	srv := NewCenterServer(newServerTestClient(t), "test-secret", 0)

	registered := make(map[string]bool)
	var v1Routes []string
	for _, r := range srv.router.Routes() {
		registered[r.Method+" "+r.Path] = true
		if strings.HasPrefix(r.Path, "/v1") {
			v1Routes = append(v1Routes, r.Method+" "+r.Path)
		}
	}
	sort.Strings(v1Routes)

	if !registered["POST /v1/login"] {
		t.Fatalf("POST /v1/login 未注册；已注册的 /v1 路由：\n  %s",
			strings.Join(v1Routes, "\n  "))
	}

	// 路由注册不等于请求可达：请求体非法时 ShouldBindJSON 在 handler 首行即返回
	// 400，据此证明请求确实进入了 handleLogin，而非停在 404 或 500。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/login",
		strings.NewReader(`{"username":""}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("非法请求体应返回 400，got %d; body=%s", w.Code, w.Body.String())
	}
}

// 用种子用户走真实登录成功路径，断言 role 出现在响应与 JWT 中。
//
// 前端靠 data.user.role 渲染菜单、决定是否显示系统管理入口；
// JWT 里的 roles 供刷新页面后 GET /v1/users/me 与后端守卫使用。
func TestLogin_SuccessReturnsRoleInBodyAndToken(t *testing.T) {
	client := newServerTestClient(t)
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	srv := NewCenterServer(client, "test-secret", 0)
	srv.userService = service.NewUserService(client)

	// 直接经 service 种子管理员（此时 callerRole=admin 满足越权校验）
	if _, err := srv.userService.CreateUser(context.Background(), "admin", &service.CreateUserRequest{
		Username: "admin-user",
		Password: "secret123",
		RealName: "管理员",
		Role:     "admin",
	}); err != nil {
		t.Fatalf("failed seeding admin: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/login",
		strings.NewReader(`{"username":"admin-user","password":"secret123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("登录失败: status=%d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Token string `json:"token"`
			User  struct {
				Username string `json:"username"`
				Role     string `json:"role"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("响应不是预期 JSON: %v; body=%s", err, w.Body.String())
	}

	if resp.Data.User.Role == "" {
		t.Error("登录响应缺少 data.user.role —— 前端无法判断角色")
	}
	if resp.Data.User.Role != "admin" {
		t.Errorf("data.user.role = %q, want %q", resp.Data.User.Role, "admin")
	}
	if resp.Data.Token == "" {
		t.Fatal("登录响应缺少 token")
	}

	// JWT 内也必须带角色
	claims, err := srv.jwtAuth.ParseToken(resp.Data.Token)
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	if len(claims.Roles) == 0 || claims.Roles[0] != "admin" {
		t.Errorf("JWT roles = %v, want [admin]", claims.Roles)
	}
}
