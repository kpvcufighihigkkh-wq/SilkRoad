package service_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func mustCreateUser(t *testing.T, svc *service.UserService, callerRole, username, role string) {
	t.Helper()
	_, err := svc.CreateUser(context.Background(), callerRole, &service.CreateUserRequest{
		Username: username,
		Password: "secret123",
		RealName: "测试用户",
		Role:     role,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s by %s) failed: %v", role, callerRole, err)
	}
}

// findUserID 按用户名取出用户 ID。
func findUserID(t *testing.T, svc *service.UserService, username string) string {
	t.Helper()
	list, _, err := svc.ListUsers(context.Background(), 1, 50, "")
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	for _, u := range list {
		if u.Username == username {
			return u.ID
		}
	}
	t.Fatalf("未找到用户 %s", username)
	return ""
}

func TestUserService_CreateUser_AdminCanCreateLowerRoles(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)

	for _, role := range []string{"admin", "inspector", "operator", "viewer"} {
		t.Run(role, func(t *testing.T) {
			mustCreateUser(t, svc, "admin", "u-"+role, role)
		})
	}
}

// Review Focus #1 的端到端形态：未知角色必须被拒绝。
func TestUserService_CreateUser_UnknownRoleRejected(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)

	_, err := svc.CreateUser(context.Background(), "admin", &service.CreateUserRequest{
		Username: "u-bad",
		Password: "secret123",
		RealName: "测试",
		Role:     "superadmin",
	})
	if err == nil {
		t.Fatal("未知角色 superadmin 被接受，越权校验失效")
	}
}

func TestUserService_CreateUser_CannotEscalate(t *testing.T) {
	cases := []struct {
		caller string
		target string
	}{
		{"viewer", "admin"},
		{"viewer", "operator"},
		{"operator", "inspector"},
		{"inspector", "admin"},
	}

	for _, tc := range cases {
		t.Run(tc.caller+"->"+tc.target, func(t *testing.T) {
			client := newTestClient(t)
			svc := service.NewUserService(client)

			_, err := svc.CreateUser(context.Background(), tc.caller, &service.CreateUserRequest{
				Username: "u-esc",
				Password: "secret123",
				RealName: "测试",
				Role:     tc.target,
			})
			if err == nil {
				t.Errorf("%s 创建了 %s，越权未拦截", tc.caller, tc.target)
			}
		})
	}
}

// 同级创建允许。
func TestUserService_CreateUser_SameRoleAllowed(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "inspector", "u-same", "inspector")
}

// Review Focus #3：Role 为 nil 时必须保持原角色不变。
func TestUserService_UpdateUser_NilRoleKeepsOriginal(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-keep", "inspector")
	targetID := findUserID(t, svc, "u-keep")

	// Role 缺省（nil）：只改姓名，角色应保持 inspector
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		RealName: "改名了",
		IsActive: true,
	}); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	got, err := svc.GetUser(context.Background(), targetID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if got.Role != "inspector" {
		t.Errorf("Role = %q, want %q（Role 为 nil 时不得改角色）", got.Role, "inspector")
	}
	if got.RealName != "改名了" {
		t.Errorf("RealName = %q, want %q", got.RealName, "改名了")
	}
}

// Review Focus #3：显式空串必须被拒绝，不能当作「保持原值」。
func TestUserService_UpdateUser_EmptyRoleRejected(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-empty", "inspector")
	targetID := findUserID(t, svc, "u-empty")

	empty := ""
	err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		Role:     &empty,
		IsActive: true,
	})
	if err == nil {
		t.Error("显式空角色被接受，应被拒绝")
	}
}

// 更新路径同样要拦越权提权。
func TestUserService_UpdateUser_CannotEscalateViaUpdate(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-target", "viewer")
	targetID := findUserID(t, svc, "u-target")

	adminRole := "admin"
	err := svc.UpdateUser(context.Background(), targetID, "inspector", &service.UpdateUserRequest{
		Role:     &adminRole,
		IsActive: true,
	})
	if err == nil {
		t.Error("inspector 通过更新接口把用户提升为 admin，越权未拦截")
	}
}

// 使用 UpdateUserRequest 的 Role 字段改角色应生效。
func TestUserService_UpdateUser_CanPromoteWithinRank(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-promote", "viewer")
	targetID := findUserID(t, svc, "u-promote")

	operatorRole := "operator"
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		Role:     &operatorRole,
		IsActive: true,
	}); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	got, _ := svc.GetUser(context.Background(), targetID)
	if got.Role != "operator" {
		t.Errorf("Role = %q, want %q", got.Role, "operator")
	}
}
