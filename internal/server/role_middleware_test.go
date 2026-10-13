package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
)

// newRoleTestRouter 搭一个只挂 requireRole 的最小路由。
// 直接注入 claims，避免依赖真实 JWT 签发。
func newRoleTestRouter(claims *middleware.JWTClaims, allowed ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		if claims != nil {
			c.Set(string(middleware.ClaimsKey), claims)
		}
		c.Next()
	})

	router.GET("/probe", requireRole(allowed...), func(c *gin.Context) {
		c.JSON(http.StatusOK, api.Success(nil))
	})

	return router
}

func doProbe(t *testing.T, router *gin.Engine) int {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	router.ServeHTTP(w, req)
	return w.Code
}

func TestRequireRole_AllowsListedRole(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "u1", Roles: []string{"admin"}},
		"admin",
	)
	if got := doProbe(t, router); got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
}

func TestRequireRole_DeniesUnlistedRole(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "u1", Roles: []string{"viewer"}},
		"admin",
	)
	if got := doProbe(t, router); got != http.StatusForbidden {
		t.Errorf("status = %d, want 403", got)
	}
}

// Review Focus #2：claims 缺失或 roles 为空时不得放行。
func TestRequireRole_MissingClaimsDenied(t *testing.T) {
	router := newRoleTestRouter(nil, "admin")
	if got := doProbe(t, router); got != http.StatusForbidden {
		t.Errorf("无 claims 时 status = %d, want 403", got)
	}
}

func TestRequireRole_EmptyRolesDenied(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "u1", Roles: []string{}},
		"admin",
	)
	if got := doProbe(t, router); got != http.StatusForbidden {
		t.Errorf("空 roles 时 status = %d, want 403", got)
	}
}

// 允许列表为空是调用方的编程错误，必须拒绝一切而非放行一切。
func TestRequireRole_EmptyAllowListDeniesEverything(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "u1", Roles: []string{"admin"}},
	)
	if got := doProbe(t, router); got != http.StatusForbidden {
		t.Errorf("空允许列表 status = %d, want 403（必须 fail closed）", got)
	}
}

// 多角色时应命中任意一个即可。
func TestRequireRole_AnyListedRoleSuffices(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "u1", Roles: []string{"inspector"}},
		"admin", "inspector",
	)
	if got := doProbe(t, router); got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
}

// 设备凭证（roles: ["edge"]）不得进入任何管理路由。
func TestRequireRole_EdgeRoleDenied(t *testing.T) {
	router := newRoleTestRouter(
		&middleware.JWTClaims{UserID: "dev1", Roles: []string{"edge"}, DeviceID: "dev1"},
		"admin",
	)
	if got := doProbe(t, router); got != http.StatusForbidden {
		t.Errorf("设备凭证 status = %d, want 403", got)
	}
}
