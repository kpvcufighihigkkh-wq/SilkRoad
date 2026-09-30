# Phase 6 实施计划：Edge↔Center 同步鉴权与接线

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Edge→Center 的数据同步真正可用且不可伪造：以「IP 注册 + 签名 token」双重鉴权，把已经实现好但未接线的上传/下发处理器接入 HTTP 端点。

**Architecture:** Center 维护 Edge 注册表（`edge_code` + `ip_address`）。管理员为已注册 Edge 签发携带 `DeviceID` 的签名 JWT，配置到 Edge 端。每次上传时 Center 做四重校验（签名有效 / 已注册 / URL 与 token 一致 / 来源 IP 与注册值一致），校验通过后由 Center 自行推导权威 `edge_id`，不采信请求体。Gin 只信任反向代理那一层，从而让 `ClientIP()` 在代理后仍能取到真实来源 IP。

**Tech Stack:** Go 1.26.4 · Ent v0.14.6 · Gin v1.12.0 · PostgreSQL（Center）· SQLite（Edge，`modernc.org/sqlite`）

**Spec:** `docs/superpowers/specs/2026-09-29-backend-service-api-design.md`（§6.2 同步流程、§6.4 下发流程）

## Design Authority（本次新增，补充 spec §6）

spec 没有定义鉴权方式。以下设计经与主人逐项确认，是本次的权威依据：

| 决策 | 内容 |
|------|------|
| Edge 身份来源 | **Center 从已认证 token 推导**，绝不用请求体的 `edge_id` |
| 注册方式 | 管理员登记 `edge_code` + `ip_address`，IP 作为人工核对凭据 |
| 鉴权凭据 | 签名 JWT（`JWTClaims.DeviceID`），复用已有的 `GenerateEdgeToken` |
| IP 的作用 | **第二重约束**，不是唯一凭据 |
| 部署形态 | Center 前有反向代理；Edge 与代理同网段，**无 NAT**（已确认） |
| 代理信任 | 只信任代理的 IP/CIDR，**不是** `0.0.0.0/0`，**也不是** `nil` |

**为什么 `nil` 是错的：** `prepareTrustedCIDRs` 在 `trustedProxies == nil` 时返回 `nil`，导致 `isTrustedProxy` 恒为 false，`ClientIP()` 跳过 XFF 分支直接返回 `RemoteIP` —— 即代理的 IP。所有 Edge 会显示为同一地址，IP 校验失效。

**为什么 `0.0.0.0/0` 是错的：** `validateHeader` 从右往左扫描 XFF，全信任时攻击者可用 `X-Forwarded-For: <他人IP>` 冒充。

**为什么只信任代理就是对的：** XFF 最右侧是代理追加的真实来源，攻击者伪造的值只能落在其左侧；扫描从右往左，遇到「不可信 IP」即停并返回，因此取到的必然是代理写入的值。

---

## Global Constraints

来自仓库 `CLAUDE.md`：

- **编码：** 所有文本文件 UTF-8（无 BOM）。写含中文的文件后必须 grep 中文字符确认未损坏。
- **行尾：** 统一 LF（`\n`）。
- **提交：** 每个逻辑单元 commit，信息含 `Refs: #17`。`pre-commit` hook 校验编码，禁止 `--no-verify`。
- **模块路径：** `github.com/yourusername/igh-silkroad`（故意不是真实仓库名，不要"修正"）。
- **禁止** force push，禁止在 `main` 上直接提交。
- **分支：** 本次新建 `feat/PLN-17-phase6-sync-auth`。
- **数据库 DSN：** Center = `postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable`（不是代码里可能残留的 `igh:igh`）。
- **psql / sqlite3 CLI 均未安装。** PostgreSQL 走 `docker exec igh-postgres psql -U igh -d igh -c "..."`；SQLite 走 `python -c "import sqlite3; ..."`。
- **不要重跑迁移** —— 两套库已迁移完成。

## Review Focus

spec 与本次设计隐含、但常规测试不会覆盖的失效模式。每条都在对应任务中配了验证步骤。

1. **伪造 `X-Forwarded-For`** —— 预期：Center 取到代理追加的真实 IP，伪造值被忽略，请求被 403 拒绝。
2. **`SetTrustedProxies` 配置错误**（`nil` 或 `0.0.0.0/0`）—— 预期：两种错误配置都会让 IP 校验失效，必须有测试锁住正确配置。
3. **A 设备持自己的 token 访问 B 的 URL** —— 预期：403，不得因「token 有效」就放行。
4. **未注册的 `edge_code`** —— 预期：403，不得自动注册。
5. **请求体携带伪造 `edge_id`** —— 预期：被忽略，写入的是 Center 推导出的值。
6. **`Entries` 乱序（bobbin 先于其 barrel 到达）** —— 预期：Center 侧按依赖顺序重排，不因外键失败。
7. **重传同一批数据** —— 预期：幂等，`applied` 计数不重复累加。

---

## File Structure

**Center 端：**

| 路径 | 职责 |
|------|------|
| `internal/service/edge.go` | 新增。Edge 注册与查询的领域逻辑 |
| `api/center/v1/edge.go` | 新增。Edge 管理 + token 签发的 HTTP 处理器 |
| `internal/server/center.go` | 修改。代理信任配置、Edge 路由接线、上传四重校验、下发接线 |
| `internal/sync/center/handler.go` | 修改。身份来源改为 `req.EdgeID`；`Entries` 依赖排序 |
| `internal/sync/center/sync_test.go` | 修改。补排序与身份测试 |
| `internal/service/edge_test.go` | 新增。EdgeService 单元测试 |

**Edge 端：**

| 路径 | 职责 |
|------|------|
| `internal/sync/edge/client.go` | 修改。真实的上传/下载实现 |
| `internal/sync/edge/uploader.go` | 新增。扫描待同步记录、组批、标记状态 |
| `internal/sync/edge/uploader_test.go` | 新增。上传器测试 |
| `internal/sync/edge/scheduler.go` | 新增。定时重试调度 |
| `internal/server/edge.go` | 修改。接线 sync handler |

---

## Task 1: Center 代理信任配置

一切 IP 校验的前提。配置错了，后续任务全部失效。

**Files:**
- Modify: `internal/server/center.go`
- Create: `internal/server/proxy_test.go`

**Interfaces:**
- Consumes: `gin.Engine`
- Produces: `applyTrustedProxies(router *gin.Engine) error` —— 读环境变量 `TRUSTED_PROXIES`（逗号分隔 CIDR），未设置时使用安全默认值 `127.0.0.1/32,::1/128`（仅本机）

- [ ] **Step 1: 记录回退锚点**

```bash
git rev-parse HEAD
```

- [ ] **Step 2: 写失败的测试**

创建 `internal/server/proxy_test.go`：

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// 未配置时不得全信任代理：伪造的 XFF 必须被忽略
func TestTrustedProxies_DefaultRejectsSpoofedXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	req.Header.Set("X-Forwarded-For", "192.168.2.84")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "203.0.113.7" {
		t.Errorf("ClientIP = %q, want %q (伪造的 XFF 必须被忽略)", seen, "203.0.113.7")
	}
}

// 配置了代理后，来自该代理的 XFF 必须被采信
func TestTrustedProxies_ConfiguredProxyAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "203.0.113.0/24")

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.9:1234" // 来自可信代理
	req.Header.Set("X-Forwarded-For", "192.168.2.84")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "192.168.2.84" {
		t.Errorf("ClientIP = %q, want %q", seen, "192.168.2.84")
	}
}

// 经可信代理转发时，攻击者塞在左侧的伪造值必须被忽略（validateHeader 右往左扫描）
func TestTrustedProxies_SpoofedLeftEntryIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TRUSTED_PROXIES", "203.0.113.0/24")

	router := gin.New()
	if err := applyTrustedProxies(router); err != nil {
		t.Fatalf("applyTrustedProxies failed: %v", err)
	}

	var seen string
	router.GET("/probe", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	// 攻击者在最左侧伪造他人 IP，代理把真实 IP 追加到最右
	req.Header.Set("X-Forwarded-For", "192.168.2.84, 192.168.5.99")

	router.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "192.168.5.99" {
		t.Errorf("ClientIP = %q, want %q (必须取代理追加的最右值)", seen, "192.168.5.99")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/server/ -run TestTrustedProxies -v`

预期：编译失败，`undefined: applyTrustedProxies`。

- [ ] **Step 4: 实现**

在 `internal/server/center.go` 中添加（放在 `NewCenterServer` 之前）：

```go
// applyTrustedProxies 配置 Gin 的可信代理。
//
// 必须只信任反向代理那一层：全信任（默认的 0.0.0.0/0）会让攻击者用
// X-Forwarded-For 冒充任意 IP；设为 nil 则会让 ClientIP() 恒返回代理自身的
// IP，使基于 IP 的设备识别失效。两种情况都会破坏上传接口的鉴权。
func applyTrustedProxies(router *gin.Engine) error {
	raw := os.Getenv("TRUSTED_PROXIES")
	if raw == "" {
		// 未配置时不信任任何代理，只信本机回环
		raw = "127.0.0.1/32,::1/128"
	}

	proxies := make([]string, 0, 4)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}

	return router.SetTrustedProxies(proxies)
}
```

在 `NewCenterServer` 中，`router := gin.New()` 之后紧接着调用：

```go
	router := gin.New()

	if err := applyTrustedProxies(router); err != nil {
		log.Fatalf("设置可信代理失败: %v", err)
	}
```

import 需补 `"os"`、`"strings"`（若尚未存在）。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/server/ -run TestTrustedProxies -v`

预期：3 个测试全部 PASS。

- [ ] **Step 6: 验证中文未损坏并提交**

```bash
grep -c "[一-龥]" internal/server/center.go
git add internal/server/
git commit -m "$(cat <<'EOF'
feat: Center配置可信代理，修复ClientIP可被伪造

默认的0.0.0.0/0信任所有代理，攻击者可用X-Forwarded-For冒充任意IP；
设为nil则ClientIP恒返回代理IP。改为只信任TRUSTED_PROXIES指定的CIDR。

Refs: #17
EOF
)"
```

---

## Task 2: EdgeService 注册与查询

**Files:**
- Create: `internal/service/edge.go`
- Create: `internal/service/edge_test.go`

**Interfaces:**
- Consumes: `*ent.Client`
- Produces:
  - `NewEdgeService(client *ent.Client) *EdgeService`
  - `CreateEdge(ctx, *CreateEdgeRequest) (*EdgeResponse, error)`
  - `GetEdgeByCode(ctx, code string) (*EdgeResponse, error)` —— 未知 code 返回 `ErrEdgeNotRegistered`
  - `ListEdges(ctx, page, pageSize int, status string) ([]*EdgeResponse, int, error)`
  - `UpdateHeartbeat(ctx, code string) error`
  - 哨兵错误 `ErrEdgeNotRegistered = errors.New("edge not registered")`
  - `EdgeResponse{ID, EdgeCode, EdgeName, IPAddress, Status, LastSeen, Version, CreatedAt, UpdatedAt}`

- [ ] **Step 1: 写失败的测试**

创建 `internal/service/edge_test.go`：

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func TestEdgeService_CreateEdge(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	resp, err := svc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-001",
		EdgeName:  "一号边端",
		IPAddress: "192.168.2.84",
	})
	if err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	if resp.EdgeCode != "edge-001" {
		t.Errorf("EdgeCode = %q, want %q", resp.EdgeCode, "edge-001")
	}
	if resp.IPAddress != "192.168.2.84" {
		t.Errorf("IPAddress = %q, want %q", resp.IPAddress, "192.168.2.84")
	}
	if resp.ID == "" {
		t.Error("ID 不应为空")
	}
}

func TestEdgeService_GetEdgeByCode_NotRegistered(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	_, err := svc.GetEdgeByCode(ctx, "never-registered")
	if !errors.Is(err, service.ErrEdgeNotRegistered) {
		t.Errorf("err = %v, want ErrEdgeNotRegistered", err)
	}
}

func TestEdgeService_GetEdgeByCode_Found(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	if _, err := svc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-002",
		EdgeName:  "二号边端",
		IPAddress: "192.168.2.85",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	found, err := svc.GetEdgeByCode(ctx, "edge-002")
	if err != nil {
		t.Fatalf("GetEdgeByCode failed: %v", err)
	}
	if found.IPAddress != "192.168.2.85" {
		t.Errorf("IPAddress = %q, want %q", found.IPAddress, "192.168.2.85")
	}
}

func TestEdgeService_CreateEdge_DuplicateCodeRejected(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	req := &service.CreateEdgeRequest{
		EdgeCode:  "edge-dup",
		EdgeName:  "重复",
		IPAddress: "192.168.2.86",
	}
	if _, err := svc.CreateEdge(ctx, req); err != nil {
		t.Fatalf("首次 CreateEdge 失败: %v", err)
	}

	if _, err := svc.CreateEdge(ctx, req); err == nil {
		t.Error("重复 edge_code 应当被拒绝（edge_code 有 Unique 约束）")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestEdgeService -v`

预期：编译失败，`undefined: service.NewEdgeService`。

- [ ] **Step 3: 实现**

创建 `internal/service/edge.go`：

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/edge"
)

// ErrEdgeNotRegistered 表示该 edge_code 未在 Center 注册。
// 上传接口据此拒绝请求 —— 未注册的设备不得被自动接纳。
var ErrEdgeNotRegistered = errors.New("edge not registered")

// EdgeService 边端设备服务
type EdgeService struct {
	client *ent.Client
}

// NewEdgeService 创建边端设备服务
func NewEdgeService(client *ent.Client) *EdgeService {
	return &EdgeService{client: client}
}

// CreateEdgeRequest 注册边端设备请求
type CreateEdgeRequest struct {
	EdgeCode  string `json:"edge_code"  binding:"required,max=50"`
	EdgeName  string `json:"edge_name"  binding:"required,max=100"`
	IPAddress string `json:"ip_address" binding:"required,max=50"`
	Version   string `json:"version"    binding:"omitempty,max=50"`
	Notes     string `json:"notes"      binding:"omitempty"`
}

// EdgeResponse 边端设备响应
type EdgeResponse struct {
	ID        string `json:"id"`
	EdgeCode  string `json:"edge_code"`
	EdgeName  string `json:"edge_name"`
	IPAddress string `json:"ip_address,omitempty"`
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateEdge 注册边端设备
func (s *EdgeService) CreateEdge(ctx context.Context, req *CreateEdgeRequest) (*EdgeResponse, error) {
	builder := s.client.Edge.Create().
		SetEdgeCode(req.EdgeCode).
		SetEdgeName(req.EdgeName).
		SetIPAddress(req.IPAddress).
		SetStatus(edge.StatusOffline)

	if req.Version != "" {
		builder.SetVersion(req.Version)
	}
	if req.Notes != "" {
		builder.SetNotes(req.Notes)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}

	return s.toEdgeResponse(created), nil
}

// GetEdgeByCode 按 edge_code 查询设备。
// 未注册时返回 ErrEdgeNotRegistered —— 上传鉴权依赖这个区分。
func (s *EdgeService) GetEdgeByCode(ctx context.Context, code string) (*EdgeResponse, error) {
	found, err := s.client.Edge.Query().
		Where(edge.EdgeCodeEQ(code)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrEdgeNotRegistered
		}
		return nil, fmt.Errorf("query edge: %w", err)
	}

	return s.toEdgeResponse(found), nil
}

// ListEdges 查询设备列表
func (s *EdgeService) ListEdges(ctx context.Context, page, pageSize int, status string) ([]*EdgeResponse, int, error) {
	query := s.client.Edge.Query()

	if status != "" {
		query = query.Where(edge.StatusEQ(edge.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc(edge.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*EdgeResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, s.toEdgeResponse(r))
	}

	return result, total, nil
}

// UpdateHeartbeat 刷新心跳时间并标记在线
func (s *EdgeService) UpdateHeartbeat(ctx context.Context, code string) error {
	affected, err := s.client.Edge.Update().
		Where(edge.EdgeCodeEQ(code)).
		SetLastSeen(time.Now()).
		SetStatus(edge.StatusOnline).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	if affected == 0 {
		return ErrEdgeNotRegistered
	}
	return nil
}

// toEdgeResponse 转换为响应格式
func (s *EdgeService) toEdgeResponse(e *ent.Edge) *EdgeResponse {
	resp := &EdgeResponse{
		ID:        e.ID.String(),
		EdgeCode:  e.EdgeCode,
		EdgeName:  e.EdgeName,
		IPAddress: e.IPAddress,
		Status:    string(e.Status),
		Version:   e.Version,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}

	if !e.LastSeen.IsZero() {
		resp.LastSeen = e.LastSeen.Format(time.RFC3339)
	}

	return resp
}

```
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestEdgeService -v`

预期：4 个测试全部 PASS。

- [ ] **Step 5: 验证中文并提交**

```bash
grep -c "边端\|注册" internal/service/edge.go
git add internal/service/
git commit -m "$(cat <<'EOF'
feat: 新增EdgeService支持边端设备注册与查询

未注册的edge_code返回ErrEdgeNotRegistered，上传接口据此拒绝请求。

Refs: #17
EOF
)"
```

---

## Task 3: Edge 管理 API 与 token 签发

**Files:**
- Create: `api/center/v1/edge.go`
- Modify: `internal/server/center.go`
- Create: `api/center/v1/edge_test.go`

**Interfaces:**
- Consumes: `service.EdgeService`、`middleware.JWTAuth`
- Produces:
  - `NewEdgeHandler(edgeService *service.EdgeService, jwtAuth *middleware.JWTAuth) *EdgeHandler`
  - `CreateEdge`、`ListEdges`、`GetEdge`、`IssueToken` 四个 handler
  - 路由：`POST /v1/edges`、`GET /v1/edges`、`GET /v1/edges/:code`、`POST /v1/edges/:code/token`

**注意：** `IssueToken` 必须挂在 **用户认证** 组（管理员操作），而上传/下发端点挂在 Edge token 之下。两者都在 `/v1/edges` 前缀内，路由冲突风险需在 Step 4 处理。

- [ ] **Step 1: 写失败的测试**

创建 `api/center/v1/edge_test.go`：

```go
package v1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api/middleware"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

func setupEdgeTest(t *testing.T) (*gin.Engine, *ent.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := setupTestClientShared(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})

	h := centerv1.NewEdgeHandler(service.NewEdgeService(client), jwtAuth)

	router := gin.New()
	router.POST("/v1/edges", h.CreateEdge)
	router.POST("/v1/edges/:code/token", h.IssueToken)

	return router, client
}

func TestEdgeHandler_IssueToken_ReturnsDeviceToken(t *testing.T) {
	router, client := setupEdgeTest(t)
	ctx := context.Background()

	if _, err := service.NewEdgeService(client).CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-tok",
		EdgeName:  "签发测试",
		IPAddress: "192.168.2.90",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-tok/token", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "token") {
		t.Errorf("响应应包含 token，实际: %s", w.Body.String())
	}
}

func TestEdgeHandler_IssueToken_UnregisteredRejected(t *testing.T) {
	router, _ := setupEdgeTest(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/no-such-edge/token", nil)
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("未注册设备不应签发 token，实际 status = %d", w.Code)
	}
}
```

**注意：** 上面的测试引用了 `setupTestClientShared(t)` 与 `time`。在 Step 2 首先生成共享测试脚手架（见 Step 2），若 `api/center/v1` 下已有测试文件可复用其脚手架，则直接复用并删除本测试中的 `setupTestClientShared` 引用改为已有函数名。

- [ ] **Step 2: 建测试脚手架**

`api/center/v1` 目前没有测试文件。创建 `api/center/v1/setup_test.go`：

```go
package v1_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

// setupTestClientShared 创建带完整 Schema 的内存 SQLite 测试客户端。
func setupTestClientShared(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:apitest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&testDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}
```

同时确保 `api/center/v1/edge_test.go` 的 import 含 `"time"`（`middleware.JWTConfig` 用到）。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./api/center/v1/ -run TestEdgeHandler -v`

预期：编译失败，`undefined: centerv1.NewEdgeHandler`。

- [ ] **Step 4: 实现 handler**

创建 `api/center/v1/edge.go`：

```go
package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// EdgeHandler 边端设备处理器
type EdgeHandler struct {
	edgeService *service.EdgeService
	jwtAuth     *middleware.JWTAuth
}

// NewEdgeHandler 创建边端设备处理器
func NewEdgeHandler(edgeService *service.EdgeService, jwtAuth *middleware.JWTAuth) *EdgeHandler {
	return &EdgeHandler{
		edgeService: edgeService,
		jwtAuth:     jwtAuth,
	}
}

// CreateEdge godoc
// @Summary 注册边端设备
// @Tags edges
// @Router /v1/edges [post]
func (h *EdgeHandler) CreateEdge(c *gin.Context) {
	var req service.CreateEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	resp, err := h.edgeService.CreateEdge(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// ListEdges godoc
// @Summary 查询边端设备列表
// @Tags edges
// @Router /v1/edges [get]
func (h *EdgeHandler) ListEdges(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)
	status := c.Query("status")

	list, total, err := h.edgeService.ListEdges(c.Request.Context(), page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(list, page, pageSize, total))
}

// GetEdge godoc
// @Summary 获取边端设备详情
// @Tags edges
// @Router /v1/edges/{code} [get]
func (h *EdgeHandler) GetEdge(c *gin.Context) {
	code := c.Param("code")

	resp, err := h.edgeService.GetEdgeByCode(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, "设备未注册"))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// IssueToken godoc
// @Summary 为已注册设备签发同步凭证
// @Tags edges
// @Router /v1/edges/{code}/token [post]
func (h *EdgeHandler) IssueToken(c *gin.Context) {
	code := c.Param("code")

	// 必须先确认已注册，否则任何字符串都能换到 token
	if _, err := h.edgeService.GetEdgeByCode(c.Request.Context(), code); err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, "设备未注册"))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	token, err := h.jwtAuth.GenerateEdgeToken(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, api.Success(gin.H{
		"token":     token,
		"edge_code": code,
	}))
}

```

- [ ] **Step 5: 注册路由**

在 `internal/server/center.go` 的 `registerRoutes` 中，**用户认证组内**添加：

```go
		// 边端设备管理（管理员操作，需用户 JWT）
		edgeService := service.NewEdgeService(s.client)
		edgeAdminHandler := centerv1.NewEdgeHandler(edgeService, s.jwtAuth)

		edgesAdmin := authorized.Group("/edges")
		{
			edgesAdmin.POST("", edgeAdminHandler.CreateEdge)
			edgesAdmin.GET("", edgeAdminHandler.ListEdges)
			edgesAdmin.GET("/:code", edgeAdminHandler.GetEdge)
			edgesAdmin.POST("/:code/token", edgeAdminHandler.IssueToken)
		}
```

**注意路由冲突：** 既有的 Edge 数据组 `/v1/edges/:id/upload` 与这里的 `/v1/edges/:code/token` 在 Gin 的路由树中会冲突（同一位置用了不同参数名 `:id` / `:code`）。**必须统一参数名**：把既有的 `:id` 改为 `:code`，并同步更新 `handleEdgeUpload` 中 `c.Param("id")` → `c.Param("code")`。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./api/center/v1/ -run TestEdgeHandler -v`

预期：2 个测试 PASS。

再跑全量确认路由冲突已解决：

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`

预期：无输出。

- [ ] **Step 7: 验证中文并提交**

```bash
grep -c "边端\|设备\|签发" api/center/v1/edge.go
git add api/ internal/server/
git commit -m "$(cat <<'EOF'
feat: 新增Edge管理API与同步凭证签发

POST /v1/edges/:code/token 为已注册设备签发携带DeviceID的签名JWT。
未注册的edge_code拒绝签发。统一路由参数名为:code以避免与上传端点冲突。

Refs: #17
EOF
)"
```

---

## Task 4: 上传端点四重校验与接线

本计划的核心。修复「空壳返回假成功」与「身份取自不可信载荷」。

**Files:**
- Modify: `internal/server/center.go`
- Create: `internal/server/upload_auth_test.go`

**Interfaces:**
- Consumes: `service.EdgeService`、`middleware.GetClaimsFromContext`、`center.UploadHandler`
- Produces: `authenticateEdge(c *gin.Context, svc *service.EdgeService) (*service.EdgeResponse, error)` —— 四重校验，成功返回权威设备记录，失败已写入响应并返回 error

- [ ] **Step 1: 写失败的测试**

创建 `internal/server/upload_auth_test.go`：

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
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// 未带 token 必须 401
func TestAuthenticateEdge_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return // authenticateEdge 已写响应
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-001/upload", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (未提供凭证)", w.Code)
	}
}

// 未注册的 edge_code 必须 403（不得自动注册）
func TestAuthenticateEdge_UnregisteredCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	token, err := jwtAuth.GenerateEdgeToken("ghost-edge")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/ghost-edge/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (未注册设备)", w.Code)
	}
}

// token 身份与 URL 不一致必须 403（防 A 冒充 B）
func TestAuthenticateEdge_CodeMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-a", EdgeName: "A", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge A failed: %v", err)
	}
	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-b", EdgeName: "B", IPAddress: "192.168.2.85",
	}); err != nil {
		t.Fatalf("CreateEdge B failed: %v", err)
	}

	// 持 A 的 token 访问 B 的 URL
	token, err := jwtAuth.GenerateEdgeToken("edge-a")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-b/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (token 与 URL 不符)", w.Code)
	}
}

// IP 与注册值不符必须 403
func TestAuthenticateEdge_IPMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-ip", EdgeName: "IP测试", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-ip")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		if _, err := authenticateEdge(c, edgeSvc); err != nil {
			return
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-ip/upload", nil)
	req.RemoteAddr = "10.9.9.9:1234" // 与注册的 192.168.2.84 不符
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (IP 不符)", w.Code)
	}
}

// 四重校验全通过必须放行
func TestAuthenticateEdge_AllChecksPass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	created, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-ok", EdgeName: "通过", IPAddress: "192.168.2.84",
	})
	if err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-ok")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	var gotID string
	router := gin.New()
	router.POST("/v1/edges/:code/upload", testEdgeAuth(jwtAuth), func(c *gin.Context) {
		edgeRow, err := authenticateEdge(c, edgeSvc)
		if err != nil {
			return
		}
		gotID = edgeRow.ID
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/edges/edge-ok/upload", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotID != created.ID {
		t.Errorf("推导出的 ID = %q, want %q", gotID, created.ID)
	}
}
```

- [ ] **Step 2: 建测试脚手架**

创建 `internal/server/setup_test.go`：

```go
package server

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)

var serverTestDBCounter int64

func newServerTestClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:servertest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&serverTestDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

// testEdgeAuth 测试专用的 Edge 认证中间件。
//
// 生产中由 CenterServer.jwtMiddleware() 承担，但那是 server 包内的方法，
// 测试无法直接复用，因此这里复刻其行为：解析 Bearer token，再用 c.Set
// 把 claims 存入 gin 上下文。必须用 c.Set 而非 context.WithValue ——
// authenticateEdge 用 c.Get 读取（与 api/center/v1/user.go:130 一致）。
func testEdgeAuth(jwtAuth *middleware.JWTAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未提供认证token"))
			c.Abort()
			return
		}

		claims, err := jwtAuth.ParseToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeTokenInvalid, "token无效"))
			c.Abort()
			return
		}

		c.Set(string(middleware.ClaimsKey), claims)
		c.Next()
	}
}
```

`internal/server/setup_test.go` 的 import 需含：

```go
import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)
```

同时确保 `upload_auth_test.go` 的 import 含 `"time"`。

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/server/ -run TestAuthenticateEdge -v`

预期：编译失败，`undefined: authenticateEdge`。

- [ ] **Step 4: 实现四重校验**

在 `internal/server/center.go` 中添加：

```go
// authenticateEdge 对上传请求做四重校验，返回权威的设备记录。
//
// 校验顺序与理由：
//  1. token 有效且带 DeviceID —— HMAC 签名，不可伪造
//  2. DeviceID 对应的设备已注册 —— 未注册设备不得被自动接纳
//  3. URL 的 :code 与 token 的 DeviceID 一致 —— 防 A 持己方 token 冒充 B
//  4. ClientIP 与注册的 ip_address 一致 —— 第二重约束
//
// 校验通过后，调用方必须使用返回记录中的 ID 作为数据归属，
// 不得采信请求体里的 edge_id。
//
// 校验失败时已写入响应，返回的 error 仅供调用方提前 return 使用。
func authenticateEdge(c *gin.Context, svc *service.EdgeService) (*service.EdgeResponse, error) {
	// 注意：Center 的 jwtMiddleware 用 c.Set 写入 gin 自己的 Keys map，
	// 而 middleware.GetClaimsFromContext 读的是 c.Request.Context()。
	// 二者不互通（gin 的 Value() 只对 string 类型的 key 回退查 Keys，
	// 而 ClaimsKey 是自定义类型），因此这里必须用 c.Get 读取 —— 与
	// api/center/v1/user.go:130 的既有写法保持一致。
	rawClaims, exists := c.Get(string(middleware.ClaimsKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "缺少设备凭证"))
		return nil, errors.New("missing device claims")
	}

	claims, ok := rawClaims.(*middleware.JWTClaims)
	if !ok || claims.DeviceID == "" {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "设备凭证无效"))
		return nil, errors.New("invalid device claims")
	}

	edgeRow, err := svc.GetEdgeByCode(c.Request.Context(), claims.DeviceID)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "设备未注册"))
			return nil, err
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return nil, err
	}

	if c.Param("code") != claims.DeviceID {
		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "凭证与请求设备不符"))
		return nil, errors.New("edge code mismatch")
	}

	if clientIP := c.ClientIP(); clientIP != edgeRow.IPAddress {
		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "来源IP与注册地址不符"))
		return nil, errors.New("ip mismatch")
	}

	return edgeRow, nil
}
```

- [ ] **Step 5: 接线 handleEdgeUpload**

把 `internal/server/center.go` 中 `handleEdgeUpload` 的 `TODO` 段替换为：

```go
	edgeRow, err := authenticateEdge(c, s.edgeService)
	if err != nil {
		return // authenticateEdge 已写响应
	}

	// 权威身份由 Center 推导，忽略请求体中的 edge_id
	uploadReq := &models.UploadRequest{
		EdgeID: claims.DeviceID,
		Cursor: req.Cursor,
	}
	for _, e := range req.Entries {
		entryID, idErr := uuid.Parse(e.ID)
		if idErr != nil {
			c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError,
				fmt.Sprintf("entry id 非法: %q", e.ID)))
			return
		}
		uploadReq.Entries = append(uploadReq.Entries, models.UploadEntry{
			Table:     e.Table,
			Operation: e.Operation,
			ID:        entryID,
			Data:      e.Data,
		})
	}

	resp, err := s.uploadHandler.HandleUpload(c.Request.Context(), uploadReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	// 心跳：鉴权成功即视为设备在线
	if hbErr := s.edgeService.UpdateHeartbeat(c.Request.Context(), edgeRow.EdgeCode); hbErr != nil {
		log.Printf("⚠️  刷新设备心跳失败 %s: %v", edgeRow.EdgeCode, hbErr)
	}

	c.JSON(http.StatusOK, api.Success(resp))
```

**同时：**
- `CenterServer` 需新增字段 `edgeService *service.EdgeService`，在 `NewCenterServer` 中初始化
- 请求体的内联 struct 需增加 `Cursor int64 \`json:"cursor"\`` 字段
- import 补 `"errors"`、`"fmt"`、`"github.com/yourusername/igh-silkroad/internal/sync/models"`

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/server/ -run TestAuthenticateEdge -v`

预期：5 个测试全部 PASS。

全量：

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`

预期：无输出。

- [ ] **Step 7: 验证中文并提交**

```bash
grep -c "校验\|设备\|凭证" internal/server/center.go
git add internal/server/
git commit -m "$(cat <<'EOF'
fix: 上传端点四重鉴权并接线，消除假成功响应

原实现返回 applied=len(entries) 却未调用 uploadHandler，Edge 会据此
标记已同步而实际丢数据；且身份取自请求体 data["edge_id"]，可伪造。
改为：签名校验→注册校验→URL一致性→IP一致性，身份由Center推导。

Refs: #17
EOF
)"
```

---

## Task 5: handler 身份来源修复与 Entries 排序

**Files:**
- Modify: `internal/sync/center/handler.go`
- Modify: `internal/sync/center/sync_test.go`

**Interfaces:**
- Consumes: `models.UploadRequest`（`EdgeID` 字段现由 Center 权威填充）
- Produces:
  - `UploadHandler.HandleUpload` 在 `Entries` 处理前按依赖顺序排序
  - `createLot` 使用 `req.EdgeID`（需传入）而非 `data["edge_id"]`
  - `sortEntriesByDependency(entries []models.UploadEntry) []models.UploadEntry`

**背景：** spec §6.2 要求上传顺序为「批次 → 落纱 → 桶 → 丝饼」，避免外键引用尚未到达的记录。当前是按原序处理。

- [ ] **Step 1: 写失败的测试**

在 `internal/sync/center/sync_test.go` 末尾追加：

```go
func TestSortEntriesByDependency(t *testing.T) {
	lotID := uuid.New()
	barrelID := uuid.New()
	bobbinID := uuid.New()

	// 故意乱序：bobbin 先于 barrel，barrel 先于 lot
	entries := []models.UploadEntry{
		{Table: "bobbins", Operation: "create", ID: bobbinID},
		{Table: "barrels", Operation: "create", ID: barrelID},
		{Table: "lots", Operation: "create", ID: lotID},
	}

	sorted := center.SortEntriesByDependency(entries)

	want := []string{"lots", "barrels", "bobbins"}
	for i, e := range sorted {
		if e.Table != want[i] {
			t.Errorf("位置 %d = %q, want %q（必须按依赖顺序）", i, e.Table, want[i])
		}
	}
}

func TestSortEntriesByDependency_StableForUnknownTables(t *testing.T) {
	entries := []models.UploadEntry{
		{Table: "unknown_b", Operation: "create", ID: uuid.New()},
		{Table: "lots", Operation: "create", ID: uuid.New()},
		{Table: "unknown_a", Operation: "create", ID: uuid.New()},
	}

	sorted := center.SortEntriesByDependency(entries)

	if sorted[0].Table != "lots" {
		t.Errorf("首个应为 lots，实际 %q", sorted[0].Table)
	}
	// 未知表保持相对顺序
	if sorted[1].Table != "unknown_b" || sorted[2].Table != "unknown_a" {
		t.Errorf("未知表相对顺序应保持，实际 %q, %q", sorted[1].Table, sorted[2].Table)
	}
}

// 载荷中的 edge_id 必须被忽略，使用请求级 EdgeID
func TestCreateLot_IgnoresPayloadEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	authoritativeEdge, err := client.Edge.Create().
		SetEdgeCode("edge-authoritative").
		SetEdgeName("权威设备").
		SetIPAddress("192.168.2.84").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	forgedEdge, err := client.Edge.Create().
		SetEdgeCode("edge-forged").
		SetEdgeName("伪造设备").
		SetIPAddress("10.0.0.1").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating forged edge: %v", err)
	}

	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: authoritativeEdge.ID.String(), // Center 推导出的权威身份
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-FORGERY-001",
					"edge_id":          forgedEdge.ID.String(), // 载荷试图伪造
					"product_type":     "FDY",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
			},
		},
	}

	resp, err := handler.HandleUpload(ctx, req)
	if err != nil {
		t.Fatalf("HandleUpload failed: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("failed retrieving lot: %v", err)
	}

	if got.EdgeID != authoritativeEdge.ID {
		t.Errorf("EdgeID = %v, want %v（必须用权威身份，忽略载荷）",
			got.EdgeID, authoritativeEdge.ID)
	}
	if got.EdgeID == forgedEdge.ID {
		t.Error("载荷中的 edge_id 被采信了 —— 这是安全缺陷")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/center/ -run "TestSortEntries|TestCreateLot_Ignores" -v`

预期：编译失败，`undefined: center.SortEntriesByDependency`。

- [ ] **Step 3: 实现排序**

在 `internal/sync/center/handler.go` 中添加：

```go
// entityDependencyOrder 定义上传实体的依赖顺序。
//
// 外键要求被引用者先到：barrel 引用 lot，bobbin 引用 lot 与 barrel。
// 乱序到达会使 Ent 的 FK 校验失败，导致整批记录被拒。
var entityDependencyOrder = map[string]int{
	"lots":     0,
	"doffings": 1,
	"barrels":  2,
	"bobbins":  3,
	"modules":  4,
	"pallets":  5,
	"cartons":  6,
}

// SortEntriesByDependency 按外键依赖顺序稳定排序上传条目。
//
// 未知表排在已知表之后，并保持其原有相对顺序（sort.SliceStable）。
func SortEntriesByDependency(entries []models.UploadEntry) []models.UploadEntry {
	if len(entries) < 2 {
		return entries
	}

	sorted := make([]models.UploadEntry, len(entries))
	copy(sorted, entries)

	rank := func(table string) int {
		if r, ok := entityDependencyOrder[table]; ok {
			return r
		}
		return len(entityDependencyOrder) // 未知表最后
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		return rank(sorted[i].Table) < rank(sorted[j].Table)
	})

	return sorted
}
```

import 补 `"sort"`。

- [ ] **Step 4: 在 HandleUpload 中应用排序**

在 `internal/sync/center/handler.go` 的 `HandleUpload` 中，把 `for _, entry := range req.Entries` 改为：

```go
	// 按外键依赖顺序处理，避免被引用记录尚未到达
	entries := SortEntriesByDependency(req.Entries)

	for _, entry := range entries {
```

- [ ] **Step 5: 修复 createLot 的身份来源**

把 `createLot` 的签名改为接收权威 edgeID：

```go
	tx, err := h.client.Tx(ctx)
```

**注意：** 简化起见，`createLot` 改为接收 `req` 上的权威 edgeID。把 `handleCreate` 与 `createLot`/`createBobbin` 的签名改为带 `edgeID uuid.UUID` 参数，并在 `HandleUpload` 中统一传入：

```go
func (h *UploadHandler) HandleUpload(ctx context.Context, req *models.UploadRequest) (*models.UploadResponse, error) {
```

在函数开头解析一次权威身份：

```go
	// 权威身份由 Center 的鉴权层推导并写入 req.EdgeID；载荷中的 edge_id 一律忽略
	authoritativeEdgeID, err := uuid.Parse(req.EdgeID)
	if err != nil {
		authoritativeEdgeID = uuid.Nil
	}
```

随后把 `applyEntry` / `handleCreate` / `createLot` / `createBobbin` 的签名逐层加上 `edgeID uuid.UUID`，并在 `createLot` 中把：

```go
	edgeID, err := uuid.Parse(getString(data, "edge_id"))
	if err != nil {
		edgeID = uuid.Nil
	}
```

整段**删除**，改为直接使用传入的 `edgeID` 参数。`if edgeID != uuid.Nil { builder.SetEdgeID(edgeID) }` 保持不变。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/sync/center/ -v`

预期：全部 PASS，含新增的 3 个测试。

- [ ] **Step 7: 验证中文并提交**

```bash
grep -c "[一-龥]" internal/sync/center/handler.go
git add internal/sync/center/
git commit -m "$(cat <<'EOF'
fix: 同步层改用权威身份并按外键依赖排序

createLot 原从载荷读 edge_id，可被伪造；改用鉴权层推导的 req.EdgeID。
新增 SortEntriesByDependency，避免被引用记录尚未到达导致外键失败。

Refs: #17
EOF
)"
```

---

## Task 6: base-data 下发接线

**Files:**
- Modify: `internal/server/center.go`
- Modify: `internal/sync/center/provider.go`（增加 edges 表支持）
- Create: `internal/server/basedata_test.go`

**Interfaces:**
- Consumes: `center.BaseDataProvider`
- Produces: `handleBaseDataPull` 真实调用 `HandlePullRequest`；`getTableData` 新增 `case "edges"`

- [ ] **Step 1: 写失败的测试**

创建 `internal/server/basedata_test.go`：

```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// base-data 端点必须真实返回已注册线体，而非空 map
func TestHandleBaseDataPull_ReturnsRealData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})
	edgeSvc := service.NewEdgeService(client)

	if _, err := edgeSvc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode: "edge-bd", EdgeName: "下发测试", IPAddress: "192.168.2.84",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	// 造一条线体数据
	if _, err := client.SpinningLine.Create().
		SetLineName("A线").
		SetLineNumber("LINE-A").
		SetCapacity(48).
		SetStatus("running").
		Save(ctx); err != nil {
		t.Fatalf("failed creating spinning line: %v", err)
	}

	token, err := jwtAuth.GenerateEdgeToken("edge-bd")
	if err != nil {
		t.Fatalf("GenerateEdgeToken failed: %v", err)
	}

	svr := &CenterServer{
		client:        client,
		edgeService:   edgeSvc,
		dataProvider:  center.NewBaseDataProvider(client),
		jwtAuth:       jwtAuth,
	}

	router := gin.New()
	router.GET("/v1/edges/base-data", testEdgeAuth(jwtAuth), svr.handleBaseDataPull)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/edges/base-data?tables=spinning_lines", nil)
	req.RemoteAddr = "192.168.2.84:1234"
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "LINE-A") {
		t.Errorf("响应未包含真实线体数据: %s", w.Body.String())
	}
}
```

import 需含 `"github.com/yourusername/igh-silkroad/internal/sync/center"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/server/ -run TestHandleBaseDataPull -v`

预期：编译失败（`CenterServer` 无 `edgeService` 字段，或断言失败——返回空 map）。

- [ ] **Step 3: provider 增加 edges 表**

在 `internal/sync/center/provider.go` 的 `getTableData` 中添加：

```go
	case "edges":
		return p.getEdges(ctx)
```

并新增：

```go
// getEdges 获取边端设备基础数据
func (p *BaseDataProvider) getEdges(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := p.client.Edge.Query().
		Order(ent.Asc(edge.FieldEdgeCode)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(rows))
	for _, e := range rows {
		result = append(result, map[string]interface{}{
			"id":         e.ID.String(),
			"edge_code":  e.EdgeCode,
			"edge_name":  e.EdgeName,
			"ip_address": e.IPAddress,
			"status":     string(e.Status),
			"version":    e.Version,
		})
	}

	return result, nil
}
```

import 补 `"github.com/yourusername/igh-silkroad/internal/database/ent/edge"`。

- [ ] **Step 4: 接线 handleBaseDataPull**

把 `internal/server/center.go` 中 `handleBaseDataPull` 的 `TODO` 段替换为：

```go
	// 与上传端点相同的四重校验：base-data 同样不得对未注册设备开放
	edgeRow, err := authenticateEdge(c, s.edgeService)
	if err != nil {
		return
	}

	// 把请求体/查询参数中的 EdgeID 覆盖为权威值
	req.EdgeID = edgeRow.EdgeCode

	resp, err := s.dataProvider.HandlePullRequest(c.Request.Context(), &models.BaseDataPullRequest{
		EdgeID: req.EdgeID,
		Tables: req.Tables,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
```

**参数来源调整：** 该端点当前只接受 JSON body，但 `authenticateEdge` 依赖 `c.Param("code")`。**必须把路由改为经 Edge 身份寻址**：

```
edge.GET("/:code/base-data", s.handleBaseDataPull)
```

并把 `handleBaseDataPull` 的入参解析改为从**查询参数**读取 `tables`（用 `c.QueryArray("tables")`），避免 GET 带 body 的兼容性问题。同步更新 Edge 侧的调用。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/server/ -run TestHandleBaseDataPull -v`

预期：PASS。

全量：

Run: `go build ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`

预期：无输出。

- [ ] **Step 6: 验证中文并提交**

```bash
grep -c "下发\|设备\|线体" internal/server/center.go internal/sync/center/provider.go
git add internal/server/ internal/sync/center/
git commit -m "$(cat <<'EOF'
fix: base-data 下发端点接线并加四重校验

原实现返回空 map 却报成功。改为真实调用 dataProvider，并复用与上传
相同的设备鉴权；新增 edges 表下发支持。

Refs: #17
EOF
)"
```

---

## Task 7: Edge 侧上传实现与重试调度

**Files:**
- Modify: `internal/sync/edge/client.go`
- Create: `internal/sync/edge/uploader.go`
- Create: `internal/sync/edge/uploader_test.go`
- Create: `internal/sync/edge/scheduler.go`
- Modify: `internal/server/edge.go`
- Modify: `cmd/edge-server/main.go`

**Interfaces:**
- Consumes: `*ent.Client`、`models.UploadRequest/UploadResponse`
- Produces:
  - `NewUploader(client *ent.Client, edgeID, centerURL, token string) *Uploader`
  - `Uploader.CollectPending(ctx, limit int) ([]models.UploadEntry, error)`
  - `Uploader.Upload(ctx) (*models.UploadResponse, error)` —— 组批、上传、按结果标记
  - `NewScheduler(u *Uploader, interval time.Duration) *Scheduler`，`Start(ctx)` / `Stop()`

- [ ] **Step 1: 写失败的测试**

创建 `internal/sync/edge/uploader_test.go`：

```go
package edge_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

func setupEdgeClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:edgeuptest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&testDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

// 只收集 pending 状态的记录
func TestUploader_CollectPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	// pending 批次
	pending, err := client.Lot.Create().
		SetLotNumber("LOT-PENDING").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 已同步批次
	synced, err := client.Lot.Create().
		SetLotNumber("LOT-SYNCED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		SetSyncStatus("synced").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating synced lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("收集到 %d 条，want 1（只应收集 pending）", len(entries))
	}
	if entries[0].ID != pending.ID {
		t.Errorf("收集到 %v，want %v", entries[0].ID, pending.ID)
	}
	if entries[0].ID == synced.ID {
		t.Error("已同步记录被重复收集")
	}
	if entries[0].Table != "lots" {
		t.Errorf("Table = %q, want %q", entries[0].Table, "lots")
	}
}

// 超过重试上限的记录不再收集
func TestUploader_CollectPending_SkipsExhaustedRetries(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-EXHAUSTED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		SetSyncRetryCount(5).
		Save(ctx); err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("收集到 %d 条，want 0（retry_count 已达上限）", len(entries))
	}
}

// 收集结果必须按依赖顺序排列
func TestUploader_CollectPending_OrderedByDependency(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	lot, err := client.Lot.Create().
		SetLotNumber("LOT-ORDER").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	if _, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-ORDER").
		SetLotID(lot.ID).
		Save(ctx); err != nil {
		t.Fatalf("failed creating barrel: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("收集到 %d 条，want 2", len(entries))
	}
	if entries[0].Table != "lots" {
		t.Errorf("首个应为 lots，实际 %q（外键要求被引用者先到）", entries[0].Table)
	}
	if entries[1].Table != "barrels" {
		t.Errorf("第二个应为 barrels，实际 %q", entries[1].Table)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/edge/ -v`

预期：编译失败，`undefined: edge.NewUploader`。

- [ ] **Step 3: 实现 Uploader**

创建 `internal/sync/edge/uploader.go`：

```go
package edge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// maxRetryCount 超过此次数的记录不再重试，等待人工介入
const maxRetryCount = 5

// uploadableEntities 定义需要上传的实体及其依赖优先级。
// 顺序即外键依赖顺序：被引用者先上传。
var uploadableEntities = []string{"lots", "doffings", "barrels", "bobbins"}

// Uploader 负责把 Edge 本地的待同步记录推送到 Center
type Uploader struct {
	client    *ent.Client
	edgeID    string
	centerURL string
	token     string
	http      *http.Client
}

// NewUploader 创建上传器
func NewUploader(client *ent.Client, edgeID, centerURL, token string) *Uploader {
	return &Uploader{
		client:    client,
		edgeID:    edgeID,
		centerURL: centerURL,
		token:     token,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// CollectPending 收集待同步记录，按外键依赖顺序排列。
// 跳过已达重试上限的记录。
func (u *Uploader) CollectPending(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	entries := make([]models.UploadEntry, 0, limit)

	for _, table := range uploadableEntities {
		if len(entries) >= limit {
			break
		}
		remaining := limit - len(entries)

		switch table {
		case "lots":
			rows, err := u.client.Lot.Query().
				Where(
					lot.SyncStatusEQ(lot.SyncStatusPending),
					lot.SyncRetryCountLT(maxRetryCount),
				).
				Order(ent.Asc(lot.FieldCreatedAt)).
				Limit(remaining).
				All(ctx)
			if err != nil {
				return nil, fmt.Errorf("query lots: %w", err)
			}
			for _, r := range rows {
				entries = append(entries, lotToEntry(r))
			}

		case "doffings":
			rows, err := u.client.Doffing.Query().
				Where(
					doffing.SyncStatusEQ(doffing.SyncStatusPending),
					doffing.SyncRetryCountLT(maxRetryCount),
				).
				Order(ent.Asc(doffing.FieldCreatedAt)).
				Limit(remaining).
				All(ctx)
			if err != nil {
				return nil, fmt.Errorf("query doffings: %w", err)
			}
			for _, r := range rows {
				entries = append(entries, models.UploadEntry{
					Table:     "doffings",
					Operation: "create",
					ID:        r.ID,
					Data: map[string]interface{}{
						"id":                r.ID.String(),
						"lot_id":            r.LotID.String(),
						"spinning_line_id":  r.SpinningLineID.String(),
						"spinning_position": r.SpinningPosition,
						"status":            string(r.Status),
					},
					CreatedAt: r.CreatedAt,
				})
			}

		case "barrels":
			rows, err := u.client.Barrel.Query().
				Where(
					barrel.SyncStatusEQ(barrel.SyncStatusPending),
					barrel.SyncRetryCountLT(maxRetryCount),
				).
				Order(ent.Asc(barrel.FieldCreatedAt)).
				Limit(remaining).
				All(ctx)
			if err != nil {
				return nil, fmt.Errorf("query barrels: %w", err)
			}
			for _, r := range rows {
				entries = append(entries, models.UploadEntry{
					Table:     "barrels",
					Operation: "create",
					ID:        r.ID,
					Data: map[string]interface{}{
						"id":            r.ID.String(),
						"barrel_number": r.BarrelNumber,
						"lot_id":        r.LotID.String(),
						"capacity":      r.Capacity,
						"current_count": r.CurrentCount,
						"status":        string(r.Status),
					},
					CreatedAt: r.CreatedAt,
				})
			}

		case "bobbins":
			rows, err := u.BobbinQuery(ctx, remaining)
			if err != nil {
				return nil, err
			}
			entries = append(entries, rows...)
		}
	}

	return entries, nil
}

// BobbinQuery 查询待同步丝饼（独立方法便于测试与复用）
func (u *Uploader) BobbinQuery(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.Bobbin.Query().
		Where(
			bobbin.SyncStatusEQ(bobbin.SyncStatusPending),
			bobbin.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(bobbin.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query bobbins: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		data := map[string]interface{}{
			"id":                r.ID.String(),
			"bobbin_number":     r.BobbinNumber,
			"lot_id":            r.LotID.String(),
			"spinning_position": r.SpinningPosition,
			"gross_weight":      r.GrossWeight,
			"net_weight":        r.NetWeight,
			"status":            string(r.Status),
			"label_printed":     r.LabelPrinted,
		}
		if r.BarrelID != uuid.Nil {
			data["barrel_id"] = r.BarrelID.String()
		}
		if r.BarrelPosition != 0 {
			data["barrel_position"] = r.BarrelPosition
		}
		out = append(out, models.UploadEntry{
			Table:     "bobbins",
			Operation: "create",
			ID:        r.ID,
			Data:      data,
			CreatedAt: r.CreatedAt,
		})
	}

	return out, nil
}

// lotToEntry 把批次记录转为上传条目
func lotToEntry(r *ent.Lot) models.UploadEntry {
	data := map[string]interface{}{
		"id":               r.ID.String(),
		"lot_number":       r.LotNumber,
		"product_type":     string(r.ProductType),
		"planned_quantity": r.PlannedQuantity,
		"actual_quantity":  r.ActualQuantity,
		"status":           string(r.Status),
	}
	if r.PlcLotNumber != "" {
		data["plc_lot_number"] = r.PlcLotNumber
	}
	if r.OrderCode != "" {
		data["order_code"] = r.OrderCode
	}
	if r.ProductSpec != "" {
		data["product_spec"] = r.ProductSpec
	}

	return models.UploadEntry{
		Table:     "lots",
		Operation: "create",
		ID:        r.ID,
		Data:      data,
		CreatedAt: r.CreatedAt,
	}
}

// Upload 执行一次上传：收集、发送、按结果更新本地同步状态。
func (u *Uploader) Upload(ctx context.Context) (*models.UploadResponse, error) {
	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return &models.UploadResponse{}, nil
	}

	payload := &models.UploadRequest{
		EdgeID:  u.edgeID,
		Entries: entries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal upload request: %w", err)
	}

	// 必须用 :code 寻址 —— Center 会校验其与 token 身份一致
	url := fmt.Sprintf("%s/v1/edges/%s/upload", u.centerURL, u.edgeID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+u.token)

	resp, err := u.http.Do(httpReq)
	if err != nil {
		// 网络故障：计入重试，下轮再传
		if markErr := u.markFailed(ctx, entries); markErr != nil {
			log.Printf("⚠️  标记同步失败状态出错: %v", markErr)
		}
		return nil, fmt.Errorf("post upload: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if markErr := u.markFailed(ctx, entries); markErr != nil {
			log.Printf("⚠️  标记同步失败状态出错: %v", markErr)
		}
		return nil, fmt.Errorf("upload rejected: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Code int                   `json:"code"`
		Data models.UploadResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if err := u.markSynced(ctx, entries); err != nil {
		log.Printf("⚠️  标记已同步状态出错: %v", err)
	}

	return &parsed.Data, nil
}

// markSynced 把成功上传的记录标记为已同步。
// 注意：Center 的响应只给出计数，不逐条确认，因此这里按「整批成功」处理。
// 若 Center 将来改为部分成功语义，此处需同步改为按 ID 标记。
func (u *Uploader) markSynced(ctx context.Context, entries []models.UploadEntry) error {
	now := time.Now()

	byTable := groupByTable(entries)

	if len(byTable["lots"]) > 0 {
		if err := u.client.Lot.Update().
			Where(lot.IDIn(byTable["lots"]...)).
			SetSyncStatus(lot.SyncStatusSynced).
			SetSyncedAt(now).
			Exec(ctx); err != nil {
			return err
		}
	}
	if len(byTable["doffings"]) > 0 {
		if err := u.client.Doffing.Update().
			Where(doffing.IDIn(byTable["doffings"]...)).
			SetSyncStatus(doffing.SyncStatusSynced).
			SetSyncedAt(now).
			Exec(ctx); err != nil {
			return err
		}
	}
	if len(byTable["barrels"]) > 0 {
		if err := u.client.Barrel.Update().
			Where(barrel.IDIn(byTable["barrels"]...)).
			SetSyncStatus(barrel.SyncStatusSynced).
			SetSyncedAt(now).
			Exec(ctx); err != nil {
			return err
		}
	}
	if len(byTable["bobbins"]) > 0 {
		if err := u.client.Bobbin.Update().
			Where(bobbin.IDIn(byTable["bobbins"]...)).
			SetSyncStatus(bobbin.SyncStatusSynced).
			SetSyncedAt(now).
			Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}

// markFailed 累加重试计数，达上限时标记为 failed
func (u *Uploader) markFailed(ctx context.Context, entries []models.UploadEntry) error {
	byTable := groupByTable(entries)

	if ids := byTable["lots"]; len(ids) > 0 {
		if err := u.client.Lot.Update().
			Where(lot.IDIn(ids...)).
			AddSyncRetryCount(1).
			Exec(ctx); err != nil {
			return err
		}
	}

	if ids := byTable["doffings"]; len(ids) > 0 {
		if err := u.client.Doffing.Update().
			Where(doffing.IDIn(ids...)).
			AddSyncRetryCount(1).
			Exec(ctx); err != nil {
			return err
		}
	}

	if ids := byTable["barrels"]; len(ids) > 0 {
		if err := u.client.Barrel.Update().
			Where(barrel.IDIn(ids...)).
			AddSyncRetryCount(1).
			Exec(ctx); err != nil {
			return err
		}
	}

	if ids := byTable["bobbins"]; len(ids) > 0 {
		if err := u.client.Bobbin.Update().
			Where(bobbin.IDIn(ids...)).
			AddSyncRetryCount(1).
			Exec(ctx); err != nil {
			return err
		}
	}

	// 达上限的记录标记为 failed，避免无限重试
	_ = u.markExhausted(ctx)

	return nil
}

// markExhausted 把重试超限的 pending 记录标记为 failed，
// 避免它们被无限次重新收集。覆盖全部四个上传实体。
func (u *Uploader) markExhausted(ctx context.Context) error {
	if _, err := u.client.Lot.Update().
		Where(
			lot.SyncRetryCountGTE(maxRetryCount),
			lot.SyncStatusEQ(lot.SyncStatusPending),
		).
		SetSyncStatus(lot.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Doffing.Update().
		Where(
			doffing.SyncRetryCountGTE(maxRetryCount),
			doffing.SyncStatusEQ(doffing.SyncStatusPending),
		).
		SetSyncStatus(doffing.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Barrel.Update().
		Where(
			barrel.SyncRetryCountGTE(maxRetryCount),
			barrel.SyncStatusEQ(barrel.SyncStatusPending),
		).
		SetSyncStatus(barrel.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	if _, err := u.client.Bobbin.Update().
		Where(
			bobbin.SyncRetryCountGTE(maxRetryCount),
			bobbin.SyncStatusEQ(bobbin.SyncStatusPending),
		).
		SetSyncStatus(bobbin.SyncStatusFailed).
		Save(ctx); err != nil {
		return err
	}

	return nil
}

// groupByTable 按表名聚合记录 ID
func groupByTable(entries []models.UploadEntry) map[string][]uuid.UUID {
	out := make(map[string][]uuid.UUID)
	for _, e := range entries {
		out[e.Table] = append(out[e.Table], e.ID)
	}
	return out
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/sync/edge/ -v`

预期：3 个测试全部 PASS。

**注意：** 若 `lot.SyncRetryCountGTE` 或 `lot.IDIn` 不存在，用 `grep -n "func SyncRetryCount\|func IDIn" internal/database/ent/lot/where.go` 查实际名称并修正。

- [ ] **Step 5: 实现调度器**

创建 `internal/sync/edge/scheduler.go`：

```go
package edge

import (
	"context"
	"log"
	"time"
)

// Scheduler 定时触发上传重试。
// 实时上传由业务操作触发，本调度器是网络故障后的兜底。
type Scheduler struct {
	uploader *Uploader
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewScheduler 创建调度器
func NewScheduler(uploader *Uploader, interval time.Duration) *Scheduler {
	return &Scheduler{
		uploader: uploader,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start 启动调度（非阻塞）
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		defer close(s.done)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				resp, err := s.uploader.Upload(ctx)
				if err != nil {
					log.Printf("⚠️  定时同步失败: %v", err)
					continue
				}
				if resp.Applied > 0 || resp.Rejected > 0 {
					log.Printf("✅ 定时同步: applied=%d rejected=%d", resp.Applied, resp.Rejected)
				}
			}
		}
	}()
}

// Stop 停止调度并等待退出
func (s *Scheduler) Stop() {
	close(s.stop)
	<-s.done
}
```

- [ ] **Step 6: 接线 Edge 服务与启动参数**

修改 `internal/server/edge.go`：

```go
	// 同步客户端
	syncClient := edge.NewSyncClient(edgeID, centerURL, client)
	uploader := edge.NewUploader(client, edgeID, centerURL, os.Getenv("CENTER_TOKEN"))

	s := &EdgeServer{
		...
		syncClient: syncClient,
		uploader:   uploader,
	}
```

`EdgeServer` 新增字段 `uploader *edge.Uploader`。

把 `handleSyncUpload` 的 TODO 替换为：

```go
	resp, err := s.uploader.Upload(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
```

修改 `cmd/edge-server/main.go`：读取 `SYNC_INTERVAL`（默认 `5m`），启动调度器，并在优雅关闭时停止它：

```go
	// 同步调度器（网络故障后的兜底重试）
	interval := 5 * time.Minute
	if v := os.Getenv("SYNC_INTERVAL"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			interval = parsed
		}
	}
	scheduler := edge.NewScheduler(uploader, interval)
	scheduler.Start(context.Background())
```

关闭时：

```go
	scheduler.Stop()
```

- [ ] **Step 7: 全量验证**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`

预期：无输出。

- [ ] **Step 8: 验证中文并提交**

```bash
grep -c "[一-龥]" internal/sync/edge/uploader.go internal/sync/edge/scheduler.go
git add internal/ cmd/
git commit -m "$(cat <<'EOF'
feat: 实现Edge侧上传与定时重试调度

按外键依赖顺序收集pending记录，携带CENTER_TOKEN上传，
按结果标记synced或累加retry_count；超限标记failed避免无限重试。
新增调度器作为网络故障后的兜底。

Refs: #17
EOF
)"
```

---

## 完成标准

```bash
go build ./... && go vet ./...                      # 无输出
go test ./... 2>&1 | grep -c FAIL                   # 0
go test ./internal/server/ ./internal/service/ ./internal/sync/... -v   # 全部 PASS
```

且：

- 伪造 `X-Forwarded-For` 无法通过鉴权（Task 1 测试）
- 未注册 `edge_code` 被拒（Task 4 测试）
- 持 A 的 token 访问 B 的 URL 被拒（Task 4 测试）
- IP 不符被拒（Task 4 测试）
- 载荷中的伪造 `edge_id` 被忽略（Task 5 测试）
- 乱序 `Entries` 被正确重排（Task 5 测试）
- 上传端点返回真实的 `applied` 计数，非 `len(entries)`（Task 4）
- `base-data` 返回真实数据，非空 map（Task 6）

## 部署配置清单（交付给运维）

| 环境变量 | 位置 | 值 | 说明 |
|---------|------|-----|------|
| `TRUSTED_PROXIES` | Center | 代理的 IP/CIDR，逗号分隔 | **必配**。不配则只信任本机，代理后所有 Edge 会被视为同一 IP |
| `CENTER_TOKEN` | Edge | 由 `POST /v1/edges/:code/token` 获取 | **必配**。缺失则上传 401 |
| `SYNC_INTERVAL` | Edge | 如 `5m` | 可选，默认 5m |
| `EDGE_ID` | Edge | 与 Center 注册的 `edge_code` 一致 | **必配且必须一致**，否则鉴权第三步失败 |

nginx 侧：

```nginx
location /v1/ {
    proxy_pass http://center:8080;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}
```

**防火墙：Center 的 8080 端口只对代理开放**，否则可绕过代理直连。

## 后续计划

本计划覆盖 Phase 6 的同步鉴权与接线。**Phase 2–5（各实体的 Service/API 重建）仍是独立的计划**，其中 `EdgeService` 的注册/查询部分已在本计划 Task 2 落地，其余实体待各阶段展开。

## Self-Review 记录

**Spec 覆盖：**

| 来源 | 本计划中的任务 |
|------|--------------|
| spec §6.2 上传流程（幂等、排序） | Task 5（排序）、Task 4（幂等由已实现的 handler 提供） |
| spec §6.4 下发流程 | Task 6 |
| spec §10 Phase 6「扩展同步机制」 | Task 1–7 |
| **本次新增的鉴权设计** | Task 1（代理信任）、Task 2–3（注册与签发）、Task 4（四重校验） |

**审查发现的实现缺陷（已在本计划中修复）：**

1. **上传端点假成功** —— 返回 `applied: len(req.Entries)` 但不调用 `uploadHandler`，Edge 会据此标记已同步而丢数据。Task 4 修复。
2. **身份取自不可信载荷** —— `handler.go` 读 `data["edge_id"]`。Task 5 修复。
3. **`base-data` 返回空 map** —— 端点未接线。Task 6 修复。
4. **无凭证签发路径** —— `GenerateEdgeToken` 零调用方，Edge 拿不到 token。Task 3 修复。
5. **`Entries` 未排序** —— 外键要求被引用者先到。Task 5 修复。
6. **`ClientIP()` 可伪造** —— Gin 默认信任所有代理。Task 1 修复。
7. **Edge 侧上传完全未实现** —— `UploadData` 是 TODO，无调度器。Task 7 实现。

**已知未覆盖（本计划范围外）：**

- `Module`/`Pallet`/`Carton` 的上传（其 `edge_id` 无外键，接入前无风险；但 Phase 2 重建其 Service 后需补上传路径）
- `BobbinGrade` 是否需要同步（spec 未定义，待 Phase 2 决策）
- Center 侧 `Edge.status` 的离线判定（本计划只做心跳刷新，不做超时标记）
