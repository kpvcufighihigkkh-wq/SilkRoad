package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// TestPrimaryRole_HighestRankWins 钉住 primaryRole 的选择规则。
//
// 它是两条提权路径上唯一的调用者身份来源，但没有任何 handler 会走到它
// （Task 6 的 requireRole 在进入 handler 之前就拦下了非 admin，而
// GET /v1/users → ListUsers 根本不计算调用者角色），因此必须在此单独覆盖。
func TestPrimaryRole_HighestRankWins(t *testing.T) {
	cases := []struct {
		name  string
		roles []string
		want  string
	}{
		// 两种顺序都要测：否则「取最高」与「取最后一个」无法区分。
		{"多个角色时取最高（低权在前）", []string{"viewer", "inspector"}, "inspector"},
		{"多个角色时取最高（低权在后）", []string{"inspector", "viewer"}, "inspector"},
		{"admin 与低权角色并存时取 admin", []string{"operator", "admin"}, "admin"},
		{"单角色返回自身", []string{"operator"}, "operator"},
		// 设备凭证的角色 "edge" 不在权重表内，RoleRank 返回 0，
		// 因而永远选不出，结果为 ""（越权校验随后 fail closed）。
		{"设备凭证角色不在权重表内", []string{"edge"}, ""},
		{"空角色切片", []string{}, ""},
		{"nil 角色切片", nil, ""},
		{"全部角色非法", []string{"superadmin", "root"}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := centerv1.PrimaryRole(&middleware.JWTClaims{Roles: tc.roles})
			if got != tc.want {
				t.Errorf("primaryRole(%v) = %q, want %q", tc.roles, got, tc.want)
			}
		})
	}
}

// setupUserUpdateTest 构造一个带 claims 注入的 UpdateUser 路由。
// 复用 internal/server/setup_test.go 的做法：c.Set + middleware.ClaimsKey。
func setupUserUpdateTest(t *testing.T, claims *middleware.JWTClaims) (*gin.Engine, *service.UserService) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := setupTestClientShared(t)
	svc := service.NewUserService(client)
	h := centerv1.NewUserHandler(svc)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ClaimsKey), claims)
		c.Next()
	})
	router.PUT("/v1/users/:id", h.UpdateUser)

	return router, svc
}

// TestUserHandler_UpdateUser_EscalationReturns403 钉住更新路径的提权错误映射。
//
// 与 CreateUser 同一处拒绝（ErrRoleTooHigh）在两个路径都必须返回
// 403 + 20004：
//   - 用 400 是错的：越权不是参数错误，前端按状态码区分「没权限」时会漏掉它；
//   - 用 500 更是错的：那会告诉运维「服务器坏了」，而事实是「你没有权限」；
//   - 用 30001（业务规则错误）同样不合口径：它和 requireRole 中间件返回的
//     20004 分属两个码，前端要同时处理两种才认得出「没权限」。
func TestUserHandler_UpdateUser_EscalationReturns403(t *testing.T) {
	// 调用者是 inspector，目标是把它提权为 admin。
	router, svc := setupUserUpdateTest(t, &middleware.JWTClaims{
		UserID: "u-inspector",
		Roles:  []string{"inspector"},
	})

	target, err := svc.CreateUser(context.Background(), "admin", &service.CreateUserRequest{
		Username: "u-victim",
		Password: "secret123",
		RealName: "目标用户",
		Role:     "viewer",
	})
	if err != nil {
		t.Fatalf("seed CreateUser failed: %v", err)
	}

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"role":"admin","is_active":true}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/users/"+target.ID, body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.Code != 20004 {
		t.Errorf("code = %d, want 20004（与 requireRole 中间件一致）", resp.Code)
	}
}

// TestUserHandler_UpdateUser_NonServiceErrorStays500 确认真正的内部错误仍是 500。
// 传入非法 UUID，service 返回的不是 *ServiceError，必须落到 500 分支。
func TestUserHandler_UpdateUser_NonServiceErrorStays500(t *testing.T) {
	router, _ := setupUserUpdateTest(t, &middleware.JWTClaims{
		UserID: "u-admin",
		Roles:  []string{"admin"},
	})

	w := httptest.NewRecorder()
	body := strings.NewReader(`{"is_active":true}`)
	req := httptest.NewRequest(http.MethodPut, "/v1/users/not-a-uuid", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}
