package service

// 角色优先级 —— 越权校验的唯一真源。
//
// 权重表集中在这里，而不是散落到各处的字符串比较：角色一旦新增，
// 只改这一张表即可，不必去追查每个 if 分支。
//
// 未知角色不在表中，RoleRank 返回 0。所有比较都必须把 0 当作「非法」
// 而非「最低级」—— 否则 RoleRank(target) >= RoleRank(caller) 会让
// 任意调用者创建未知角色。
var roleRanks = map[string]int{
	"viewer":    1,
	"operator":  2,
	"inspector": 3,
	"admin":     4,
}

// RoleRank 返回角色的权重；未知角色（含空串）返回 0。
func RoleRank(role string) int {
	return roleRanks[role]
}

// IsRoleHigherOrEqual 判断 targetRole 是否不高于 callerRole。
//
// 任一角色未知或为空时返回 false —— 越权校验必须 fail closed，
// 不能因为「查不到权重」就默认放行。
func IsRoleHigherOrEqual(callerRole, targetRole string) bool {
	callerRank := RoleRank(callerRole)
	targetRank := RoleRank(targetRole)

	if callerRank == 0 || targetRank == 0 {
		return false
	}

	return targetRank <= callerRank
}
