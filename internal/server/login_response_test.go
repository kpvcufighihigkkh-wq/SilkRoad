package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// 路由必须存在，且失败路径不返回 404。
func TestLogin_RouteExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

	srv := NewCenterServer(nil, "test-secret", 0)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/login",
		strings.NewReader(`{"username":"nobody","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.router.ServeHTTP(w, req)

	if w.Code == http.StatusNotFound {
		t.Fatal("POST /v1/login 路由不存在")
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
