package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent/user"
)

// mustUUID 解析测试中已知合法的 UUID 字符串。
func mustUUID(t *testing.T, raw string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("invalid uuid %q: %v", raw, err)
	}
	return id
}

// 停用账号后，已签发的 token 必须立即失效 —— 不能等它自然过期（默认 2 小时）。
//
// 这是管理后台区别于「角色只是装饰」的关键：管理员在系统管理页点了停用，
// 期望的是访问权立刻消失。若中间件只信 token 内的快照，被停用的人在这段
// 时间窗口里仍可签发设备凭证。
func TestRequireActiveUser_DeactivatedAccountIsRejectedImmediately(t *testing.T) {
	client := newServerTestClient(t)
	srv := newCenterServerForTest(t, client)

	userID := seedTestUser(t, client, "soon-disabled", "admin", true)
	token, err := srv.jwtAuth.GenerateToken(userID, "soon-disabled", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	// 先确认 token 本身可用（否则下面的 403 不能归因于停用）
	if code := doGet(srv, token, "/v1/users"); code == http.StatusForbidden {
		t.Fatalf("停用前既已被拒（status=%d），测试前提不成立", code)
	}

	// 停用账号 —— token 不变
	if err := client.User.UpdateOneID(mustUUID(t, userID)).
		SetIsActive(false).
		Exec(t.Context()); err != nil {
		t.Fatalf("disable user: %v", err)
	}

	if code := doGet(srv, token, "/v1/users"); code != http.StatusForbidden {
		t.Errorf("停用后访问 → %d, want 403（旧 token 仍在有效期内）", code)
	}
}

// 删除账号后，旧 token 必须被拒为 401（身份已不存在），而不是 500 或放行。
func TestRequireActiveUser_DeletedAccountIsRejected(t *testing.T) {
	client := newServerTestClient(t)
	srv := newCenterServerForTest(t, client)

	userID := seedTestUser(t, client, "soon-deleted", "admin", true)
	token, err := srv.jwtAuth.GenerateToken(userID, "soon-deleted", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if err := client.User.DeleteOneID(mustUUID(t, userID)).Exec(t.Context()); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if code := doGet(srv, token, "/v1/users"); code != http.StatusUnauthorized {
		t.Errorf("删除后访问 → %d, want 401", code)
	}
}

// 角色降级必须立即生效：token 里仍写着 admin，但库里已是 viewer。
//
// 断言方式：用同一个人对管理接口发请求，降级前放行、降级后被 requireRole 拒绝。
// 若中间件不覆盖 claims.Roles，这里会一直放行。
func TestRequireActiveUser_RoleChangeTakesEffectWithoutReLogin(t *testing.T) {
	client := newServerTestClient(t)
	srv := newCenterServerForTest(t, client)

	userID := seedTestUser(t, client, "soon-demoted", "admin", true)
	token, err := srv.jwtAuth.GenerateToken(userID, "soon-demoted", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	if code := doGet(srv, token, "/v1/users"); code == http.StatusForbidden {
		t.Fatalf("降级前已被拒（status=%d），测试前提不成立", code)
	}

	if err := client.User.UpdateOneID(mustUUID(t, userID)).
		SetRole(user.RoleViewer).
		Exec(t.Context()); err != nil {
		t.Fatalf("demote user: %v", err)
	}

	if code := doGet(srv, token, "/v1/users"); code != http.StatusForbidden {
		t.Errorf("降级后访问管理接口 → %d, want 403（token 内仍写着 admin）", code)
	}
}

// 设备凭证不经过本中间件：它没有 users 行，若被回查会永远 403。
func TestRequireActiveUser_DeviceTokenIsNotLookedUp(t *testing.T) {
	client := newServerTestClient(t)
	srv := newCenterServerForTest(t, client)

	deviceToken, err := srv.jwtAuth.GenerateEdgeToken("edge-no-user-row")
	if err != nil {
		t.Fatalf("GenerateEdgeToken: %v", err)
	}

	code := doGet(srv, deviceToken, "/v1/lots")
	if code == http.StatusUnauthorized || code == http.StatusInternalServerError {
		t.Errorf("设备凭证在 requireActiveUser 处被误伤（status=%d）", code)
	}
}

// doGet 发一个带 Bearer token 的 GET 请求并返回状态码。
func doGet(srv *CenterServer, token, path string) int {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	srv.router.ServeHTTP(w, req)
	return w.Code
}
