package service_test

import (
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func TestRoleRank_KnownRoles(t *testing.T) {
	cases := []struct {
		role string
		want int
	}{
		{"viewer", 1},
		{"operator", 2},
		{"inspector", 3},
		{"admin", 4},
	}

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			if got := service.RoleRank(tc.role); got != tc.want {
				t.Errorf("RoleRank(%q) = %d, want %d", tc.role, got, tc.want)
			}
		})
	}
}

// 未知角色必须权重为 0。若有实现给未知角色一个正权重，
// 下面的 IsRoleHigherOrEqual 用例会把后果暴露出来。
func TestRoleRank_UnknownRoleIsZero(t *testing.T) {
	for _, role := range []string{"", "superadmin", "ADMIN", "root"} {
		t.Run(role, func(t *testing.T) {
			if got := service.RoleRank(role); got != 0 {
				t.Errorf("RoleRank(%q) = %d, want 0（未知角色必须 fail closed）", role, got)
			}
		})
	}
}

// Review Focus #1：未知目标角色不得被判为「不高于调用者」。
//
// 若实现写成 `RoleRank(target) >= RoleRank(caller)`，未知角色权重 0 会让
// 每个角色都能创建它 —— 越权校验形同虚设。
func TestIsRoleHigherOrEqual_UnknownTargetIsDenied(t *testing.T) {
	if service.IsRoleHigherOrEqual("admin", "superadmin") {
		t.Error("admin 不应能创建未知角色 superadmin（权重 0 必须 fail closed）")
	}
	if service.IsRoleHigherOrEqual("admin", "") {
		t.Error("admin 不应能创建空角色")
	}
}

// Review Focus #2：未知/空调用者角色不得放行任何操作。
func TestIsRoleHigherOrEqual_UnknownCallerIsDenied(t *testing.T) {
	if service.IsRoleHigherOrEqual("", "viewer") {
		t.Error("未知调用者角色不应能创建 viewer")
	}
	if service.IsRoleHigherOrEqual("superadmin", "viewer") {
		t.Error("未知调用者角色不应能创建 viewer")
	}
}

func TestIsRoleHigherOrEqual_Matrix(t *testing.T) {
	cases := []struct {
		caller string
		target string
		want   bool
	}{
		{"admin", "admin", true},
		{"admin", "inspector", true},
		{"admin", "operator", true},
		{"admin", "viewer", true},
		{"inspector", "admin", false},
		{"inspector", "inspector", true},
		{"inspector", "operator", true},
		{"inspector", "viewer", true},
		{"operator", "admin", false},
		{"operator", "inspector", false},
		{"operator", "operator", true},
		{"operator", "viewer", true},
		{"viewer", "admin", false},
		{"viewer", "inspector", false},
		{"viewer", "operator", false},
		{"viewer", "viewer", true},
	}

	for _, tc := range cases {
		t.Run(tc.caller+"->"+tc.target, func(t *testing.T) {
			if got := service.IsRoleHigherOrEqual(tc.caller, tc.target); got != tc.want {
				t.Errorf("IsRoleHigherOrEqual(%q, %q) = %v, want %v",
					tc.caller, tc.target, got, tc.want)
			}
		})
	}
}
