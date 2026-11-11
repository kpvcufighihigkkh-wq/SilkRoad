package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

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
		IsActive: boolPtr(true),
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
		IsActive: boolPtr(true),
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
		IsActive: boolPtr(true),
	})
	if err == nil {
		t.Error("inspector 通过更新接口把用户提升为 admin，越权未拦截")
	}
}

// boolPtr 取布尔字面量的地址，用于构造 UpdateUserRequest 的显式取值字段。
func boolPtr(v bool) *bool {
	return &v
}

// Review Focus #3：IsActive 为 nil 时必须保持原启用状态不变。
//
// 与 NilRoleKeepsOriginal 是同一个失效模式（同一 struct、同一函数）：用值类型
// 时缺省的 is_active 会被 encoding/json 解析成 false，一个只改姓名的
// PUT /v1/users/:id 会把目标账号静默停用 —— 此后该用户登录得到
// ErrUserInactive，而请求方看到的是 200 成功。
//
// 这条用例是该 bug 唯一的覆盖：此前所有用例都传了 IsActive: true，
// 因此把字段改回值类型不会有任何测试变红。
func TestUserService_UpdateUser_NilIsActiveKeepsOriginal(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-active", "inspector")
	targetID := findUserID(t, svc, "u-active")

	before, err := svc.GetUser(context.Background(), targetID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if !before.IsActive {
		t.Fatalf("前置条件不成立：新建用户 IsActive = false, want true")
	}

	// IsActive 缺省（nil）：只改姓名，启用状态应保持 true
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		RealName: "只改了姓名",
	}); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	got, err := svc.GetUser(context.Background(), targetID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if got.RealName != "只改了姓名" {
		t.Errorf("RealName = %q, want %q", got.RealName, "只改了姓名")
	}
	if !got.IsActive {
		t.Errorf("IsActive = false, want true（IsActive 为 nil 时不得改启用状态；该用户此后登录会得到 ErrUserInactive）")
	}

	// 显式 &false 必须真的停用 —— 否则「用 nil 判断」会退化成「永远不停用」。
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		IsActive: boolPtr(false),
	}); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	off, err := svc.GetUser(context.Background(), targetID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if off.IsActive {
		t.Errorf("IsActive = true, want false（显式 &false 必须停用）")
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
		IsActive: boolPtr(true),
	}); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	got, _ := svc.GetUser(context.Background(), targetID)
	if got.Role != "operator" {
		t.Errorf("Role = %q, want %q", got.Role, "operator")
	}
}

// 邮箱与电话必须能清空：只按非空字符串写入时，前端没有任何办法把
// 一个已经填过的邮箱改回空 —— 这与 Role / IsActive 的三态处理不一致。
func TestUserService_UpdateUser_CanClearEmailAndPhone(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-contact", "viewer")
	targetID := findUserID(t, svc, "u-contact")

	email := "a@example.com"
	phone := "13800000000"
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		Email: &email,
		Phone: &phone,
	}); err != nil {
		t.Fatalf("设置邮箱电话失败: %v", err)
	}

	got, _ := svc.GetUser(context.Background(), targetID)
	if got.Email != email || got.Phone != phone {
		t.Fatalf("设置后 email=%q phone=%q, want %q / %q", got.Email, got.Phone, email, phone)
	}

	// 显式传空串 → 清空
	empty := ""
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		Email: &empty,
		Phone: &empty,
	}); err != nil {
		t.Fatalf("清空邮箱电话失败: %v", err)
	}

	got, _ = svc.GetUser(context.Background(), targetID)
	if got.Email != "" {
		t.Errorf("Email = %q, want 空（显式空串必须清空）", got.Email)
	}
	if got.Phone != "" {
		t.Errorf("Phone = %q, want 空（显式空串必须清空）", got.Phone)
	}
}

// 不传 Email / Phone（nil）时必须保持原值 —— 与 Role / IsActive 同一纪律。
func TestUserService_UpdateUser_NilEmailAndPhoneKeepOriginal(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	mustCreateUser(t, svc, "admin", "u-contact-keep", "viewer")
	targetID := findUserID(t, svc, "u-contact-keep")

	email := "keep@example.com"
	phone := "13900000000"
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		Email: &email,
		Phone: &phone,
	}); err != nil {
		t.Fatalf("设置邮箱电话失败: %v", err)
	}

	name := "改名用户"
	if err := svc.UpdateUser(context.Background(), targetID, "admin", &service.UpdateUserRequest{
		RealName: name,
	}); err != nil {
		t.Fatalf("只改姓名失败: %v", err)
	}

	got, _ := svc.GetUser(context.Background(), targetID)
	if got.RealName != name {
		t.Errorf("RealName = %q, want %q", got.RealName, name)
	}
	if got.Email != email {
		t.Errorf("Email = %q, want %q（未提供时不得清空）", got.Email, email)
	}
	if got.Phone != phone {
		t.Errorf("Phone = %q, want %q（未提供时不得清空）", got.Phone, phone)
	}
}

// 管理员重置他人密码：不校验旧密码，凭 callerRole 授权。
func TestUserService_ResetPassword_AdminResetsWithoutOldPassword(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	ctx := context.Background()

	mustCreateUser(t, svc, "admin", "reset-target", "operator")
	id := findUserID(t, svc, "reset-target")

	if err := svc.ResetPassword(ctx, id, "admin", "brand-new-pw"); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	// 新密码可登录，旧密码失效
	if _, err := svc.ValidateLogin(ctx, "reset-target", "brand-new-pw"); err != nil {
		t.Errorf("新密码登录失败：%v", err)
	}
	if _, err := svc.ValidateLogin(ctx, "reset-target", "secret123"); err == nil {
		t.Error("旧密码仍然可登录：重置没有生效")
	}
}

// 非管理员不得重置他人密码 —— 这是本接口唯一的授权依据。
func TestUserService_ResetPassword_NonAdminRejected(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	ctx := context.Background()

	mustCreateUser(t, svc, "admin", "reset-victim", "admin")
	id := findUserID(t, svc, "reset-victim")

	for _, role := range []string{"operator", "inspector", "viewer", ""} {
		if err := svc.ResetPassword(ctx, id, role, "hijacked-pw"); err == nil {
			t.Errorf("callerRole=%q 竟然可以重置他人密码", role)
		}
	}

	// 确认原密码仍有效
	if _, err := svc.ValidateLogin(ctx, "reset-victim", "secret123"); err != nil {
		t.Errorf("被拒绝的请求仍然改动了密码：%v", err)
	}
}

func TestUserService_ResetPassword_RejectsShortPassword(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)
	ctx := context.Background()

	mustCreateUser(t, svc, "admin", "reset-short", "viewer")
	id := findUserID(t, svc, "reset-short")

	if err := svc.ResetPassword(ctx, id, "admin", "12345"); err == nil {
		t.Fatal("want error：新密码不足 6 位")
	}
}

func TestUserService_ResetPassword_RejectsUnknownUser(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewUserService(client)

	if err := svc.ResetPassword(context.Background(), uuid.NewString(), "admin", "whatever1"); err == nil {
		t.Fatal("want error：用户不存在")
	}
}
