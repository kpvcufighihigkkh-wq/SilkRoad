# Center 管理后台后端支撑 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Center 管理后台补齐后端支撑——批次列表筛选、死代码清除、以及一套真正生效的角色权限模型。

**Architecture:** 全部改动集中在 Center 侧（`internal/server`、`internal/service`、`api/center/v1`）。批次只读边界不变：不新增任何批次/丝饼写路由。权限采用「路由级粗粒度守卫 + 服务层细粒度越权校验」两层，角色优先级用一张显式权重表。

**Tech Stack:** Go 1.26.4、Ent v0.14.6、Gin、SQLite（测试内存库）/ PostgreSQL（生产）

**Spec:** `docs/superpowers/specs/2026-10-12-frontend-admin-design.md`

## Global Constraints

- **数据归属单向**：Center 不得新增批次/丝饼写路由。`TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 与 `TestRealRouter_CenterKeepsReadRoutes` 必须保持绿，断言强度不得放宽。
- **编码**：所有文本文件 UTF-8 无 BOM、行尾 LF。`.editorconfig` 定义 2 空格缩进。
- **角色枚举**：`admin`、`operator`、`inspector`、`viewer`（`internal/database/ent/schema/user.go`）。优先级 `admin` > `inspector` > `operator` > `viewer`。
- **错误码**：用 `api.CodeForbidden`（20004）表示权限不足，`api.CodeInvalidParams`（10001）表示参数错误。两者已存在于 `api/response.go`。
- **不提交**：本计划的每个任务以「验证通过」结尾，**不包含 git commit 步骤**——提交时机由项目负责人决定。

## Review Focus

以下是 spec 隐含但没有任何任务的测试直接覆盖、最容易在真实使用中伤到人的五类输入。每条都在所属任务里配了对应测试。

1. **未知角色字符串**（`role: "superadmin"`）：调用方传来的角色不在四值枚举内时，权重查表返回 0。若用 `>=` 比较，权重 0 会被判为「不高于任何角色」从而放行 —— 必须 fail closed。
2. **空角色列表**（JWT 的 `roles` 为 `[]` 或 claims 缺失）：`requireAdmin` 类守卫不得因「没有任何角色需要检查」而放行。
3. **`role` 字段缺省与显式空串的区别**（`PUT /v1/users/:id`）：`UpdateUserRequest.Role` 为 `nil` 表示「不改角色」，为 `""` 表示「改成空角色」。两者语义不同，前者必须跳过校验，后者必须被拒绝。
4. **`keyword` 含 SQL LIKE 元字符**（`%`、`_`、`\`）：模糊匹配不得被这些字符改变语义，也不得拼接出非法 SQL。
5. **自改密码 vs 管理员改密码**：`PUT /v1/users/:id/password` 收紧为仅本人后，管理员无法再改他人密码。这不是回归（现状本就要求提供旧密码），但必须显式记录，否则会被误认为功能丢失。
---

### Task 1: 角色优先级与越权校验基础

先建立角色权重的唯一真源，后续任务都依赖它。

**Files:**
- Create: `internal/service/role.go`
- Test: `internal/service/role_test.go`

**Interfaces:**
- Consumes: 无
- Produces:
  - `service.RoleRank(role string) int` —— 返回角色权重，未知角色返回 `0`
  - `service.IsRoleHigherOrEqual(callerRole, targetRole string) bool` —— `targetRole` 不高于 `callerRole` 时返回 `true`；任一角色未知或为空时返回 `false`（fail closed）
  - `var service.ErrRoleTooHigh` —— 越权时返回的错误

- [ ] **Step 1: 写失败的测试**

创建 `internal/service/role_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run 'TestRoleRank|TestIsRoleHigherOrEqual' -v`
Expected: FAIL，`undefined: service.RoleRank`

- [ ] **Step 3: 写最小实现**

创建 `internal/service/role.go`：

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run 'TestRoleRank|TestIsRoleHigherOrEqual' -v`
Expected: PASS（4 个测试函数，含子测试）

- [ ] **Step 5: 全量回归**

Run: `go build ./... && go test ./internal/service/`
Expected: 编译通过，既有 service 测试全绿

---

### Task 2: RequireRole 中间件

**Files:**
- Create: `internal/server/role_middleware.go`
- Test: `internal/server/role_middleware_test.go`

**Interfaces:**
- Consumes: `middleware.JWTClaims`（`Roles []string`）、`middleware.ClaimsKey`
- Produces: `requireRole(roles ...string) gin.HandlerFunc` —— server 包内函数，允许列表为空时拒绝一切请求（fail closed）

- [ ] **Step 1: 写失败的测试**

创建 `internal/server/role_middleware_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/server/ -run TestRequireRole -v`
Expected: FAIL，`undefined: requireRole`

- [ ] **Step 3: 写最小实现**

创建 `internal/server/role_middleware.go`：

```go
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
)

// requireRole 限制路由只能由指定角色访问。
//
// 与 requireUserPrincipal 的分工：后者只回答「是不是人类用户」（挡设备凭证），
// 本中间件回答「这个人类用户能不能做这件事」。两者需要同时挂载 ——
// requireRole 依赖 claims 存在，若单独使用而 claims 缺失，它会因读不到角色
// 而拒绝（fail closed），但错误信息不如 requireUserPrincipal 明确。
//
// 允许列表为空时拒绝一切：空列表是调用方的编程错误，
// 「没有角色需要检查」不能推导出「所有人都可以进」。
func requireRole(roles ...string) gin.HandlerFunc {
	// 预先转成集合，避免每请求线性扫描
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		raw, exists := c.Get(string(middleware.ClaimsKey))
		if !exists {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
			c.Abort()
			return
		}

		claims, ok := raw.(*middleware.JWTClaims)
		if !ok {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
			c.Abort()
			return
		}

		for _, r := range claims.Roles {
			if _, hit := allowed[r]; hit {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
		c.Abort()
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/server/ -run TestRequireRole -v`
Expected: PASS（7 个测试函数）

- [ ] **Step 5: 全量回归**

Run: `go build ./... && go test ./internal/server/`
Expected: 全绿 —— 此时 `requireRole` 尚未挂载到任何路由，既有测试不受影响

---

### Task 3: 批次列表筛选

**Files:**
- Modify: `internal/service/lot.go`（`ListLots` 签名与查询构建）
- Modify: `api/center/v1/lot.go`（`ListLots` handler 读取新参数）
- Test: `internal/service/lot_test.go`（追加用例）

**Interfaces:**
- Consumes: 无
- Produces: `func (s *LotService) ListLots(ctx context.Context, page, pageSize int, edgeID, status, orderCode, productType, keyword string) ([]*LotResponse, int, error)`

> **注意签名变化：** `ListLots` 从 6 个参数变为 9 个。唯一调用方是 `api/center/v1/lot.go` 的 handler，本任务一并改。

- [ ] **Step 1: 写失败的测试**

在 `internal/service/lot_test.go` **末尾追加**（保留既有两个测试不动）：

```go
// seedLotsForFilter 造三条可区分的批次，供筛选测试使用。
func seedLotsForFilter(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := context.Background()

	seeds := []struct {
		number  string
		order   string
		product string
		spec    string
	}{
		{"FDY-2026-001", "PO-1001", "FDY", "150D/48F"},
		{"POY-2026-002", "PO-1002", "POY", "100D/36F"},
		{"DTY-2026-003", "", "DTY", "150D/48F"},
	}

	for _, s := range seeds {
		b := client.Lot.Create().
			SetLotNumber(s.number).
			SetProductType(lot.ProductType(s.product)).
			SetProductSpec(s.spec).
			SetPlannedQuantity(1000).
			SetActualQuantity(0).
			SetStatus(lot.Status("in_progress"))
		if s.order != "" {
			b.SetOrderCode(s.order)
		}
		if _, err := b.Save(ctx); err != nil {
			t.Fatalf("failed seeding lot %s: %v", s.number, err)
		}
	}
}

func TestLotService_ListLots_FilterByOrderCode(t *testing.T) {
	client := newTestClient(t)
	seedLotsForFilter(t, client)
	svc := service.NewLotService(client)

	got, total, err := svc.ListLots(context.Background(), 1, 20, "", "", "PO-1001", "", "")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(got) != 1 || got[0].OrderCode != "PO-1001" {
		t.Errorf("got %d 条, 期望恰好 1 条 PO-1001", len(got))
	}
}

func TestLotService_ListLots_FilterByProductType(t *testing.T) {
	client := newTestClient(t)
	seedLotsForFilter(t, client)
	svc := service.NewLotService(client)

	_, total, err := svc.ListLots(context.Background(), 1, 20, "", "", "", "POY", "")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

func TestLotService_ListLots_KeywordMatchesLotNumber(t *testing.T) {
	client := newTestClient(t)
	seedLotsForFilter(t, client)
	svc := service.NewLotService(client)

	// "2026-001" 只应命中 FDY-2026-001，不应命中 POY-2026-002
	got, total, err := svc.ListLots(context.Background(), 1, 20, "", "", "", "", "2026-001")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(got) != 1 || got[0].LotNumber != "FDY-2026-001" {
		t.Errorf("keyword 命中了错误的批次: %+v", got)
	}
}

// Review Focus #4：LIKE 元字符不得改变语义。
//
// 若实现直接把 keyword 拼进 LIKE 模式（"%"+keyword+"%"）而不转义，
// "%" 会匹配所有批次、"_" 会匹配任意单字符 —— 搜索框里输入 % 返回全表。
func TestLotService_ListLots_KeywordEscapesLikeMetachars(t *testing.T) {
	client := newTestClient(t)
	seedLotsForFilter(t, client)
	svc := service.NewLotService(client)

	// 没有任何批次号含字面量 "%"，期望 0 条；未转义时会返回全部 3 条
	_, total, err := svc.ListLots(context.Background(), 1, 20, "", "", "", "", "%")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 0 {
		t.Errorf("keyword=%% 返回 %d 条, want 0（LIKE 元字符未转义）", total)
	}

	// 下划线同理
	_, total, err = svc.ListLots(context.Background(), 1, 20, "", "", "", "", "_")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 0 {
		t.Errorf("keyword=_ 返回 %d 条, want 0（LIKE 元字符未转义）", total)
	}
}

// 空参数必须等价于「不过滤」，否则前端首次进入页面会看到空列表。
func TestLotService_ListLots_EmptyFiltersReturnAll(t *testing.T) {
	client := newTestClient(t)
	seedLotsForFilter(t, client)
	svc := service.NewLotService(client)

	_, total, err := svc.ListLots(context.Background(), 1, 20, "", "", "", "", "")
	if err != nil {
		t.Fatalf("ListLots failed: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}
```

同时在文件顶部补 import：

```go
import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestLotService_ListLots -v`
Expected: FAIL，`too many arguments in call to svc.ListLots`

- [ ] **Step 3: 改 service 层**

修改 `internal/service/lot.go` 的 `ListLots`。原实现整体替换为：

```go
// ListLots 查询批次列表。
//
// 筛选参数全部可选，空串表示不过滤。keyword 模糊匹配 lot_number。
func (s *LotService) ListLots(
	ctx context.Context,
	page, pageSize int,
	edgeID, status, orderCode, productType, keyword string,
) ([]*LotResponse, int, error) {
	query := s.client.Lot.Query()

	if edgeID != "" {
		id, err := uuid.Parse(edgeID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid edge_id: %w", err)
		}
		query = query.Where(lot.EdgeIDEQ(id))
	}

	if status != "" {
		query = query.Where(lot.StatusEQ(lot.Status(status)))
	}

	if orderCode != "" {
		query = query.Where(lot.OrderCodeEQ(orderCode))
	}

	if productType != "" {
		query = query.Where(lot.ProductTypeEQ(lot.ProductType(productType)))
	}

	if keyword != "" {
		// 转义 LIKE 元字符后再拼接模式。
		//
		// 不转义时，"%" 会匹配全表、"_" 会匹配任意单字符 ——
		// 用户在搜索框里输入一个 % 就能拉出所有批次。反斜杠必须最先替换，
		// 否则会把后续插入的转义反斜杠再次转义。
		escaped := strings.NewReplacer(
			`\`, `\\`,
			`%`, `\%`,
			`_`, `\_`,
		).Replace(keyword)
		query = query.Where(lot.LotNumberContainsFold(escaped))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	lots, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc(lot.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*LotResponse, 0, len(lots))
	for _, l := range lots {
		result = append(result, s.toLotResponse(l))
	}

	return result, total, nil
}
```

在 `internal/service/lot.go` 的 import 中补 `"strings"`。

**关于转义：** ent 的 `LotNumberContainsFold` 生成参数化 LIKE，绑定值本身不构成 SQL 注入。转义解决的是 **LIKE 模式语义**问题（`%`/`_` 被当作通配符）。注意 ent 默认的 LIKE 不带 `ESCAPE` 子句，因此转义在不同数据库上对 `%` 与 `_` 的生效程度可能不同——测试断言的是「输入 `%` 不返回全表」这一可观察行为。**若该断言在实现后仍失败，请勿放宽测试，而是在查询中显式指定 ESCAPE 字符（ent 的 `sql.FieldContainsFold` 需要走 `modify` 才能加 `ESCAPE`），并在任务报告中说明所采用的方式。**

- [ ] **Step 4: 改 handler**

修改 `api/center/v1/lot.go` 的 `ListLots` 方法体：

```go
func (h *LotHandler) ListLots(c *gin.Context) {
	page, pageSize := api.ParsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	edgeID := c.Query("edge_id")
	status := c.Query("status")
	orderCode := c.Query("order_code")
	productType := c.Query("product_type")
	keyword := c.Query("keyword")

	lots, total, err := h.lotService.ListLots(
		c.Request.Context(), page, pageSize,
		edgeID, status, orderCode, productType, keyword,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(lots, page, pageSize, total))
}
```

同时更新该方法的 godoc 注释，补上新参数：

```go
// @Param order_code query string false "订单号（精确匹配）"
// @Param product_type query string false "产品类型（FDY/POY/DTY）"
// @Param keyword query string false "批次号模糊搜索"
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestLotService_ListLots -v`
Expected: PASS（5 个测试函数）

- [ ] **Step 6: 全量回归**

Run: `go build ./... && go test ./internal/service/ ./internal/server/ ./api/...`
Expected: 全绿

---

### Task 4: 删除 Center 批次写接口死代码

**Files:**
- Modify: `api/center/v1/lot.go`（删除三个 handler 方法）
- Modify: `internal/service/lot.go`（删除三个 service 方法及 `CreateLotRequest`）
- Test: `internal/server/real_router_test.go`（**不修改**——它是防线）
- Test: `internal/service/lot_test.go`（删除两个 `CreateLot` 测试）

**Interfaces:**
- Consumes: 无
- Produces: `api/center/v1/lot.go` 只剩 `GetLot`、`ListLots` 两个方法

**为什么删：** spec §2.4。这三个 handler 从未挂载，其存在会让人误以为 Center 具备批次写入能力，与 `8ae036f` 确立的只读边界相悖。

- [ ] **Step 1: 确认删除前的基线**

Run: `go test ./internal/server/ -run 'TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes|TestRealRouter_CenterKeepsReadRoutes' -v`
Expected: PASS —— 记录这个基线，删除后必须仍然 PASS

- [ ] **Step 2: 删除三个 handler 方法**

从 `api/center/v1/lot.go` 中删除以下方法的全部内容（含其 godoc 注释）：
- `func (h *LotHandler) CreateLot(c *gin.Context)` —— 约 32 行起
- `func (h *LotHandler) UpdateLotStatus(c *gin.Context)` —— 约 102 行起
- `func (h *LotHandler) DeleteLot(c *gin.Context)` —— 约 130 行起

保留 `GetLot`、`ListLots`、`LotHandler` struct、`NewLotHandler` 构造函数与文件头 import。

- [ ] **Step 3: 删除 service 层对应方法**

从 `internal/service/lot.go` 中删除：
- `func (s *LotService) CreateLot(...)` —— 约 62 行起
- `func (s *LotService) UpdateLotStatus(...)` —— 约 150 行起
- `func (s *LotService) DeleteLot(...)` —— 约 162 行起
- `type CreateLotRequest struct {...}` —— 约 26 行起（只被已删的 `CreateLot` 使用）

`type UpdateLotRequest struct` 同样只被已删方法使用，一并删除。

保留 `lot` 包 import（`ListLots` 仍用）、`uuid` import（`ListLots` 解析 edgeID 仍用）、`fmt` import（`ListLots` 仍用）。

- [ ] **Step 4: 处理既有测试**

`internal/service/lot_test.go` 中的 `TestLotService_CreateLot_WithEdgeID` 与 `TestLotService_CreateLot_WithEdge` 会因 `CreateLot` 消失而编译失败。

**删除这两个测试函数。** 它们验证的是 `CreateLot` 的行为，而被验证的方法不再存在——这不是「为了通过而删测试」，是所测对象已从设计中移除。

**替代覆盖：** 「批次带 edge_id / plc_lot_number 落库」的验证责任转移给同步路径——`internal/sync/center/sync_test.go` 已覆盖 Edge 上传批次的落库行为，那才是生产上唯一真实的批次创建路径。

- [ ] **Step 5: 运行测试确认通过**

Run: `go build ./... && go test ./internal/service/ ./internal/server/ ./api/...`
Expected: 全绿

- [ ] **Step 6: 验证防线未被削弱**

Run: `go test ./internal/server/ -run 'TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes|TestRealRouter_CenterKeepsReadRoutes' -v`
Expected: PASS —— 与 Step 1 基线一致。**若变红，说明删除引入了路由变化，必须排查而非改测试。**

- [ ] **Step 7: 确认无残留引用**

Run: `grep -rn "CreateLot\|UpdateLotStatus\|DeleteLot" --include="*.go" internal/ api/ cmd/ | grep -v "^internal/database"`
Expected: 无输出

---

### Task 5: 用户创建与更新的越权校验

**Files:**
- Modify: `internal/service/user.go`（`CreateUser`、`UpdateUser` 签名与校验，`UpdateUserRequest` 加 `Role`）
- Modify: `api/center/v1/user.go`（两个 handler 传调用者角色）
- Test: `internal/service/user_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `service.IsRoleHigherOrEqual`、`service.RoleRank`
- Produces:
  - `func (s *UserService) CreateUser(ctx context.Context, callerRole string, req *CreateUserRequest) (*UserResponse, error)`
  - `func (s *UserService) UpdateUser(ctx context.Context, id, callerRole string, req *UpdateUserRequest) error`
  - `type UpdateUserRequest struct { RealName, Email, Phone string; Role *string; IsActive bool }`

> **签名变化：** 两个方法都新增 `callerRole string` 参数。调用方仅有 `api/center/v1/user.go` 中的两个 handler。

- [ ] **Step 1: 写失败的测试**

创建 `internal/service/user_test.go`：

```go
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
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestUserService -v`
Expected: FAIL，`too many arguments in call to svc.CreateUser`

- [ ] **Step 3: 改 `UpdateUserRequest` 与 service 方法**

修改 `internal/service/user.go`。先把 `UpdateUserRequest` 改为：

```go
// UpdateUserRequest 更新用户请求
//
// Role 用指针以区分「未提供」与「显式空串」：
//   nil      → 不改角色
//   &"admin" → 改角色为 admin
//   &""      → 非法，返回参数错误
//
// 用值类型时二者都会被 encoding/json 解析成空串，无法区分，
// 前端只改姓名的请求会意外清空角色。
type UpdateUserRequest struct {
	RealName string  `json:"real_name"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone"`
	Role     *string `json:"role"`
	IsActive bool    `json:"is_active"`
}
```

先在文件中（`ErrUserExists` 附近）定义错误。**读现有 `ErrUserExists` 的定义并照抄其形态**——若它用 `&ServiceError{Code: ..., Message: ...}`，则写：

```go
// ErrRoleTooHigh 目标角色高于调用者，或角色值非法。
var ErrRoleTooHigh = &ServiceError{
	Code:    30001, // api.CodeBusinessRuleError
	Message: "无权创建或授予高于自身级别的角色",
}
```

> **若 `ServiceError` 结构不同（字段名/类型不一致），以文件内既有错误的实际形态为准，不要自创结构。**

`CreateUser` 改为（在原有逻辑**之前**插入越权校验）：

```go
// CreateUser 创建用户。
//
// callerRole 是发起请求者的角色。校验「目标角色不得高于调用者」——
// 缺少这道校验时，任何已登录用户都能创建一个 admin 账号完成提权。
func (s *UserService) CreateUser(ctx context.Context, callerRole string, req *CreateUserRequest) (*UserResponse, error) {
	if !IsRoleHigherOrEqual(callerRole, req.Role) {
		return nil, ErrRoleTooHigh
	}

	// ...（以下原有实现保持不变：检查用户名存在、加密密码、builder、保存、返回）
}
```

`UpdateUser` 改为：

```go
func (s *UserService) UpdateUser(ctx context.Context, id, callerRole string, req *UpdateUserRequest) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	builder := s.client.User.UpdateOneID(userID)

	if req.RealName != "" {
		builder.SetFullName(req.RealName)
	}

	if req.Email != "" {
		builder.SetEmail(req.Email)
	}

	if req.Phone != "" {
		builder.SetPhone(req.Phone)
	}

	builder.SetIsActive(req.IsActive)

	// Role 为 nil 表示不改角色；非 nil 时必须校验。
	if req.Role != nil {
		if !IsRoleHigherOrEqual(callerRole, *req.Role) {
			return ErrRoleTooHigh
		}
		builder.SetRole(user.Role(*req.Role))
	}

	return builder.Exec(ctx)
}
```

- [ ] **Step 4: 改 handler 传调用者角色**

修改 `api/center/v1/user.go`。在 `UserHandler` 定义之前加两个辅助函数：

```go
// claimsFromContext 从 gin 上下文取出 JWT claims。
// 与 GetCurrentUser 中既有的读取方式一致（c.Get + middleware.ClaimsKey），
// 不改成 c.Request.Context() 的 Value —— 二者不互通（gin 的 Value()
// 只对 string 类型的 key 回退查 Keys，而 ClaimsKey 是自定义类型）。
func claimsFromContext(c *gin.Context) (*middleware.JWTClaims, bool) {
	raw, exists := c.Get(string(middleware.ClaimsKey))
	if !exists {
		return nil, false
	}
	claims, ok := raw.(*middleware.JWTClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// primaryRole 取调用者的最高角色，用于越权比较。
//
// JWT 的 Roles 是数组（设备凭证为 ["edge"]），管理接口的用户凭证实际只含
// 一个角色。取最高权重的那个：多角色时不能因为数组里有一个低权角色
// 就把调用者降级，也不能只取第一个。
func primaryRole(claims *middleware.JWTClaims) string {
	best := ""
	bestRank := 0
	for _, r := range claims.Roles {
		if rank := service.RoleRank(r); rank > bestRank {
			best, bestRank = r, rank
		}
	}
	return best
}
```

`CreateUser` 方法改为：

```go
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), primaryRole(claims), &req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(user))
}
```

`UpdateUser` 方法中，把调用改为：

```go
	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	if err := h.userService.UpdateUser(c.Request.Context(), id, primaryRole(claims), &req); err != nil {
```

在 `api/center/v1/user.go` 的 import 中补 `"github.com/yourusername/igh-silkroad/api/middleware"`。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestUserService -v`
Expected: PASS（8 个测试函数）

- [ ] **Step 6: 全量回归**

Run: `go build ./... && go test ./...`
Expected: 全绿

---

### Task 6: 挂载 RequireRole 到管理路由

**Files:**
- Modify: `internal/server/center.go`（在既有路由组上挂 `requireRole`）
- Test: `internal/server/real_router_test.go`（追加测试函数，既有函数一律不动）

**Interfaces:**
- Consumes: Task 2 的 `requireRole`、Task 5 的越权校验
- Produces: 生产路由上生效的角色守卫

- [ ] **Step 1: 写失败的测试**

在 `internal/server/real_router_test.go` **末尾追加**（既有函数一律不动）：

```go
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
		{http.MethodPut, "/v1/users/00000000-0000-0000-0000-000000000001", `{"real_name":"n","is_active":true}`},
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
func TestRealRouter_AdminPassesRoleGuard(t *testing.T) {
	router, token := newRealRouterWithRole(t, "admin")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Errorf("admin 被角色守卫拦下（status=%d）; body=%s", w.Code, w.Body.String())
	}
}

// 只读的批次/丝饼查询对所有已登录角色开放 —— 角色守卫不得误伤它们。
func TestRealRouter_AllRolesCanReadLots(t *testing.T) {
	for _, role := range []string{"viewer", "operator", "inspector", "admin"} {
		t.Run(role, func(t *testing.T) {
			router, token := newRealRouterWithRole(t, role)

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

// 设备凭证仍不得进入管理路由（回归：requireRole 不能取代 requireUserPrincipal）。
func TestRealRouter_DeviceTokenStillBlockedFromUserRoutes(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/users",
		strings.NewReader(`{"username":"x","password":"y","real_name":"z","role":"admin"}`))
	req.Header.Set("Authorization", "Bearer "+deviceToken)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("设备凭证 → %d, want 403; body=%s", w.Code, w.Body.String())
	}
}
```

> `strings`、`net/http`、`net/http/httptest`、`gin` 已在该文件的 import 中，无需新增。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/server/ -run 'TestRealRouter_NonAdmin|TestRealRouter_AdminPasses|TestRealRouter_AllRoles' -v`
Expected: FAIL —— 非管理员用例得到 200/500 而非 403（守卫尚未挂载）

- [ ] **Step 3: 挂载守卫**

修改 `internal/server/center.go` 的 `registerRoutes`。把用户与设备两个管理组改为：

```go
		// 用户管理（仅管理员）
		userService := service.NewUserService(s.client)
		userHandler := centerv1.NewUserHandler(userService)

		users := authorized.Group("/users", requireRole("admin"))
		{
			users.POST("", userHandler.CreateUser)
			users.GET("", userHandler.ListUsers)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.PUT("/:id/password", userHandler.ChangePassword)
			users.DELETE("/:id", userHandler.DeleteUser)
		}

		// GET /v1/users/me 必须对**所有已登录角色**开放：它返回调用者自己的
		// 信息，是前端判断角色、渲染菜单的依据。挂进 admin 组会让 viewer
		// 登录后拿不到自己的角色，前端无法决定显示什么。
		//
		// gin 的路由树允许 /users/me 与 /users/:id 共存（静态段优先于参数段），
		// 因此单独注册一条即可。
		authorized.GET("/users/me", userHandler.GetCurrentUser)

		// 边端设备管理（仅管理员）
		edgeAdminHandler := centerv1.NewEdgeHandler(s.edgeService, s.jwtAuth)

		edgesAdmin := authorized.Group("/edges", requireRole("admin"))
		{
			edgesAdmin.POST("", edgeAdminHandler.CreateEdge)
			edgesAdmin.GET("", edgeAdminHandler.ListEdges)
			edgesAdmin.GET("/:code", edgeAdminHandler.GetEdge)
			edgesAdmin.POST("/:code/token", edgeAdminHandler.IssueToken)
		}
```

**关于 `GET /v1/users/:id`：** 它保留在 admin 组内。理由：它返回任意用户的详情，属管理操作。

**关于 `PUT /v1/users/:id/password`：** 保留在 admin 组。`ChangePassword` 服务层要求提供 `old_password` 并校验，因此管理员实际无法改他人密码（不知道旧密码）——功能上等价于「仅本人可改」。这是在收紧权限而非放宽：原本所有登录用户都能调用它。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/server/ -run 'TestRealRouter' -v`
Expected: PASS（含既有全部 `TestRealRouter_*` 与新增 6 个）

- [ ] **Step 5: 全量回归**

Run: `go build ./... && go test ./...`
Expected: 全绿

- [ ] **Step 6: 验证只读边界仍完好**

Run: `go test ./internal/server/ -run 'TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes|TestRealRouter_CenterKeepsReadRoutes|TestRealRouter_DeviceTokenCannotReachUserRoutes' -v`
Expected: PASS —— 三条防线均未削弱

---

### Task 7: 登录响应补全角色信息

**Files:**
- Modify: `internal/server/center.go`（`handleLogin` 响应体，仅在测试失败时需要）
- Test: `internal/server/login_response_test.go`（新建）

**Interfaces:**
- Consumes: `service.NewUserService`
- Produces: 锁定 `POST /v1/login` 响应的 `data.user.role` 与 JWT `roles` claim

> **现状核查：** `handleLogin` 已返回 `data.user.role`，`GenerateToken` 已把角色写入 JWT 的 `roles` claim。**缺口 #6 实际上已实现，但没有任何测试断言它**——删掉响应里的 `role` 字段不会有测试变红。本任务的价值是补上这道防线。

- [ ] **Step 1: 写测试**

创建 `internal/server/login_response_test.go`：

```go
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
```

> **关于 `srv.userService`：** 若该字段在 `CenterServer` 上不存在或不可赋值，请读 `internal/server/center.go` 的 struct 定义与实际构造方式，按既有形态获取 `UserService`（可能需通过构造参数传入或调用已有的构造函数）。**不要为了测试便利而改变生产的可见性。**

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/server/ -run TestLogin -v`
Expected: 可能 PASS —— 因为字段已存在。**若 PASS，说明缺口 #6 早已实现，本任务的产出就是这道防线本身。** 若 FAIL，按报错进入 Step 3。

- [ ] **Step 3: 仅在失败时补响应字段**

确认 `handleLogin` 的成功响应包含：

```go
	c.JSON(http.StatusOK, api.Success(gin.H{
		"token": token,
		"user": gin.H{
			"id":        user.ID.String(),
			"username":  user.Username,
			"full_name": user.FullName,
			"role":      string(user.Role),
		},
	}))
```

- [ ] **Step 4: 确认测试通过**

Run: `go test ./internal/server/ -run TestLogin -v`
Expected: PASS（2 个测试函数）

- [ ] **Step 5: 全量回归**

Run: `go build ./... && go test ./...`
Expected: 全绿

---

## 完成标准

全部 7 个任务完成后，逐条核对 spec §10 的验收标准：

| spec 验收标准 | 由哪个任务保证 |
|---|---|
| §10.1 数据一致性（前端用后端 `progress`） | 前端任务，本计划不含 |
| §10.2 权限有效性（viewer 调 `POST /v1/users` 得 20004） | Task 6 的 `TestRealRouter_NonAdminCannotReachUserManagement` |
| §10.3 提权阻断（inspector 创建 admin 被拒） | Task 5 的 `TestUserService_CreateUser_CannotEscalate` |
| §10.4 只读边界（两条路由测试为绿） | Task 4 Step 6、Task 6 Step 6 |
| §10.5 死代码清除（三个方法不存在） | Task 4 Step 7 的 grep |
| §10.6 枚举完整 | 前端任务，本计划不含 |
| §10.7 缺口可验证 | spec §8 的 #1–#6 对应任务 |

**不在本计划范围：** spec §8 的 #7（包装单元端点）、#8（报表聚合端点），各需另立计划。

**未提交变更的提醒：** 本计划不包含 git commit 步骤。全部任务完成后，工作区会有 spec、本计划、以及 6 个代码文件的改动，交由项目负责人决定提交时机与分支策略。
