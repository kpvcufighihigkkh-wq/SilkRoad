# 后端 Service 层与 API 重构 · 第一阶段实施计划（Phase 0–1）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 清除已删除实体的残留代码恢复编译，并把 V3 数据模型缺失的同步元数据与设备归属字段补齐到 Schema 和两套数据库中。

**Architecture:** 本计划覆盖设计文档 §10 的 Phase 0 与 Phase 1 —— 先做纯删除与字段改写的「恢复编译」阶段，再做 Schema 增量调整 + Ent 代码重生成 + 双库迁移。Phase 2 及之后的 Service/API 重建不在本计划内：它们的代码依赖 Phase 1 生成的具体 Ent API 名称，必须先落地本阶段才能准确编写。

**Tech Stack:** Go 1.26.4 · Ent v0.14.6 · Gin v1.12.0 · PostgreSQL 16（Docker `igh-postgres`）· SQLite（`modernc.org/sqlite`，纯 Go）

**Spec:** `docs/superpowers/specs/2026-09-29-backend-service-api-design.md`

---

## Global Constraints

以下约束适用于本计划的每一个任务，均来自仓库的 `CLAUDE.md`：

- **编码：** 所有文本文件必须 UTF-8（无 BOM）。写入含中文的文件后必须 grep 关键中文字段确认未损坏。
- **换行：** 统一 LF（`\n`），不使用 CRLF。
- **提交：** 每完成一个逻辑单元即 commit，不积累大量未提交变更。提交信息包含 `Refs: #17`。`pre-commit` hook 会校验编码。
- **禁止：** 不对 `main` 分支 `git push -f`。不跳过 hook（不使用 `--no-verify`）。
- **回退锚点：** 执行任何删除或迁移前记录当前 HEAD hash。
- **模块路径：** `github.com/yourusername/igh-silkroad`（注意不是真实的仓库名，不要"修正"它）。
- **数据库 DSN：** Center = `postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable`（**不是**代码里现有的 `igh:igh` 默认值）。
- **任务跟踪：** 使用 `veans`，不使用 TodoWrite。任务标识 `#17`。

---

## Review Focus

以下是 spec 隐含、但本阶段的常规编译/测试不会覆盖的失效模式。每条都在下方拥有它的任务里配了对应的验证步骤。

1. **迁移在已有数据的库上重复执行** —— 预期：可安全重复运行，已有行不被修改或删除。当前 Center 有 `SpinningLines` 4 行、`Users` 4 行。
2. **向已有数据的表新增带默认值的列** —— 预期：迁移成功，既有行的新列取默认值（`sync_status='pending'`、`sync_retry_count=0`），不报错、不置空。
3. **在 SQLite 上删除列（`barrels.doffing_id`）** —— 预期：ent 的表重建过程保留表中既有行。当前该表为空，需在迁移前后对比行数确认未丢失。
4. **删除 `ent_edge` 包** —— 预期：构建失败会暴露任何残留引用；不能只靠 grep 判断"没人用"。
5. **Ent 代码重生成覆盖手工 Schema 编辑** —— 预期：`git diff` 必须只反映本次意图内的改动；若出现非预期的文件变动，说明生成命令的 target 不对，需停下来排查。

---

## File Structure

**Phase 0 删除：**

| 路径 | 原因 |
|------|------|
| `internal/service/order.go` | 引用已删除的 `ent/order` 包，是当前编译失败的根因 |
| `api/center/v1/order.go` | 同上，`Order` 实体已不存在 |
| `internal/database/ent_edge/`（整目录 29 文件） | 第二套 Ent schema，仍含 `order_id`，未被任何服务引用 |
| `internal/sync/edge/uploader.go` | 依赖 `ent_edge`，未接入服务 |
| `internal/sync/edge/downloader.go` | 同上 |
| `internal/sync/edge/sync_test.go` | 仅测试上面两个文件 |

**Phase 0 修改：**

| 路径 | 改动 |
|------|------|
| `internal/service/lot.go` | `OrderID` → `EdgeID` + `PLCLotNumber` + `OrderCode` |
| `api/center/v1/lot.go` | 查询参数 `order_id` → `edge_id` |
| `internal/server/center.go` | 移除 `/v1/orders` 路由组 |
| `internal/sync/center/handler.go` | `SetOrderID` → `SetEdgeID`/`SetPLCLotNumber` |
| `internal/sync/center/provider.go` | 删除 `getProjects`，改为基础数据（grades/spinning_lines） |
| `internal/sync/center/sync_test.go` | 修正 `Project` 用法与枚举值 |
| `cmd/seed/main.go` | 删除 Project/Order 种子数据 |
| `cmd/test-db/main.go` | 删除 Project CRUD 测试，改用新实体 |
| `cmd/migrate/main.go` | 修正默认 DSN 密码 |

**Phase 0 新建：**

| 路径 | 职责 |
|------|------|
| `internal/service/setup_test.go` | Service 层测试脚手架（内存 SQLite + Ent Client） |
| `internal/service/lot_test.go` | `LotService` 的第一批单元测试 |

**Phase 1 修改：**

| 路径 | 改动 |
|------|------|
| `internal/database/ent/schema/barrel.go` | 删除 `doffing_id` 字段、doffing 关联、对应索引 |
| `internal/database/ent/schema/doffing.go` | 删除 `barrels` 反向关联 |
| `internal/database/ent/schema/{lot,barrel,bobbin,doffing,module,pallet,carton}.go` | 各增 3 个同步字段 + 索引 |
| `internal/database/ent/schema/{module,pallet,carton}.go` | 各增 `edge_id` 字段 + 索引 |
| `internal/database/ent/**`（生成产物） | `ent generate` 重新生成 |

**Phase 1 新建：**

| 路径 | 职责 |
|------|------|
| `internal/database/ent/schema/schema_test.go` | Schema 层断言：新字段存在、默认值正确、Barrel 不再关联 Doffing |

---

## 前置事实（已实测确认，可直接依赖）

写计划前已在本机验证，以下事实无需重新论证：

| 事实 | 验证方式 |
|------|---------|
| 当前编译错误只有 1 处根因：`internal/service/order.go:7` 引用不存在的 `ent/order` | `go build ./...` |
| Ent 无 `generate.go`、无 `ent` CLI，但可用 `go run` 生成 | 实测通过 |
| 生成命令 **幂等**：在未改 Schema 时运行，`internal/database/ent/` 零文件变动 | 实测 `git status` 前后均 0 |
| `ent_edge` 无服务引用，其测试独立通过 | `grep` + `go test ./internal/sync/...` |
| Center 库当前数据量：`SpinningLines` 4 行、`Users` 4 行，其余为空 | 迁移工具统计输出 |
| PostgreSQL 容器 `igh-postgres` 运行中，密码为 `igh_dev_password` | `docker inspect` |

**Ent 代码生成命令**（Phase 1 各任务共用）：

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema
```

输出目标为 schema 目录的父目录，即 `internal/database/ent/`。

**Service 层测试命令：**

```bash
go test ./internal/service/... -v
```

**同步层测试命令：**

```bash
go test ./internal/sync/... -v
```

---

## Phase 0 — 清除残留代码，恢复可编译状态

### Task 1: Service 层测试脚手架 + LotService 适配新 Schema

`internal/service/order.go` 引用不存在的 `ent/order`，导致整个 `service` 包无法编译。必须与其它改动同批处理。

**Files:**
- Create: `internal/service/setup_test.go`
- Create: `internal/service/lot_test.go`
- Modify: `internal/service/lot.go`
- Delete: `internal/service/order.go`

**Interfaces:**
- Consumes: `ent.Client`（来自 `internal/database/ent`）
- Produces:
  - `newTestClient(t *testing.T) *ent.Client`（测试脚手架，后续所有 service 测试复用）
  - `LotService.CreateLot(ctx, *CreateLotRequest) (*LotResponse, error)`，其中 `CreateLotRequest` 字段为 `LotNumber/EdgeID/PLCLotNumber/OrderCode/ProductType/ProductSpec/PlannedQuantity`
  - `LotResponse` 字段为 `ID/LotNumber/EdgeID/PLCLotNumber/OrderCode/ProductType/ProductSpec/Status/PlannedQuantity/ActualQuantity/Progress/StartTime/EndTime/CreatedAt/UpdatedAt`

- [ ] **Step 1: 记录回退锚点**

```bash
git rev-parse HEAD
```

预期输出：`5034415`（或会话开始时的 HEAD）。保存该值。

- [ ] **Step 2: 创建测试脚手架**

创建 `internal/service/setup_test.go`：

```go
package service_test

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

// newTestClient 创建带完整 Schema 的内存 SQLite 测试客户端。
// 每个测试使用独立数据库名，避免相互污染。
func newTestClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:svctest%d?mode=memory&cache=shared&_fk=1",
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

- [ ] **Step 3: 写失败的测试**

创建 `internal/service/lot_test.go`：

```go
package service_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func TestLotService_CreateLot_WithEdgeID(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewLotService(client)

	resp, err := svc.CreateLot(ctx, &service.CreateLotRequest{
		LotNumber:       "LOT-TEST-001",
		PLCLotNumber:    "PLC-8823",
		ProductType:     "FDY",
		ProductSpec:     "150D/48F",
		PlannedQuantity: 4800,
	})
	if err != nil {
		t.Fatalf("CreateLot failed: %v", err)
	}

	if resp.LotNumber != "LOT-TEST-001" {
		t.Errorf("LotNumber = %q, want %q", resp.LotNumber, "LOT-TEST-001")
	}
	if resp.PLCLotNumber != "PLC-8823" {
		t.Errorf("PLCLotNumber = %q, want %q", resp.PLCLotNumber, "PLC-8823")
	}
	if resp.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", resp.Status, "in_progress")
	}
	if resp.PlannedQuantity != 4800 {
		t.Errorf("PlannedQuantity = %d, want 4800", resp.PlannedQuantity)
	}
	if resp.ActualQuantity != 0 {
		t.Errorf("ActualQuantity = %d, want 0", resp.ActualQuantity)
	}
	if resp.Progress != 0 {
		t.Errorf("Progress = %v, want 0", resp.Progress)
	}
}

func TestLotService_CreateLot_WithEdge(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	edge, err := client.Edge.Create().
		SetEdgeCode("edge-test-01").
		SetEdgeName("测试边端").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	svc := service.NewLotService(client)

	resp, err := svc.CreateLot(ctx, &service.CreateLotRequest{
		LotNumber:       "LOT-TEST-002",
		EdgeID:          edge.ID.String(),
		ProductType:     "POY",
		PlannedQuantity: 1200,
	})
	if err != nil {
		t.Fatalf("CreateLot failed: %v", err)
	}

	if resp.EdgeID != edge.ID.String() {
		t.Errorf("EdgeID = %q, want %q", resp.EdgeID, edge.ID.String())
	}
}
```

- [ ] **Step 4: 运行测试确认失败**

Run: `go test ./internal/service/... -run TestLotService -v`

预期：编译失败，错误为 `no required module provides package .../internal/database/ent/order`（来自 `order.go`）以及 `resp.PLCLotNumber undefined`。这正是本任务要消除的问题。

- [ ] **Step 5: 删除 OrderService**

```bash
git rm internal/service/order.go
```

- [ ] **Step 6: 改写 LotService**

把 `internal/service/lot.go` 中的请求/响应结构与相关方法替换为以下内容（保留文件顶部的 package 与 import，`import` 需去掉不再使用的 `order` 相关项；`lot` 与 `ent` 仍在使用）：

```go
// CreateLotRequest 创建批次请求
type CreateLotRequest struct {
	LotNumber       string `json:"lot_number" binding:"required,max=50"`
	EdgeID          string `json:"edge_id" binding:"omitempty,uuid"`
	PLCLotNumber    string `json:"plc_lot_number" binding:"omitempty,max=50"`
	OrderCode       string `json:"order_code" binding:"omitempty,max=50"`
	ProductType     string `json:"product_type" binding:"required,oneof=FDY POY DTY"`
	ProductSpec     string `json:"product_spec" binding:"omitempty,max=100"`
	PlannedQuantity int    `json:"planned_quantity" binding:"required,min=1"`
}

// UpdateLotRequest 更新批次请求
type UpdateLotRequest struct {
	ProductSpec     *string `json:"product_spec" binding:"omitempty,max=100"`
	PlannedQuantity *int    `json:"planned_quantity" binding:"omitempty,min=1"`
}

// LotResponse 批次响应
type LotResponse struct {
	ID              string  `json:"id"`
	LotNumber       string  `json:"lot_number"`
	EdgeID          string  `json:"edge_id,omitempty"`
	PLCLotNumber    string  `json:"plc_lot_number,omitempty"`
	OrderCode       string  `json:"order_code,omitempty"`
	ProductType     string  `json:"product_type"`
	ProductSpec     string  `json:"product_spec,omitempty"`
	Status          string  `json:"status"`
	PlannedQuantity int     `json:"planned_quantity"`
	ActualQuantity  int     `json:"actual_quantity"`
	Progress        float64 `json:"progress"`
	StartTime       *string `json:"start_time,omitempty"`
	EndTime         *string `json:"end_time,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}
```

`CreateLot` 方法体改为：

```go
// CreateLot 创建批次
func (s *LotService) CreateLot(ctx context.Context, req *CreateLotRequest) (*LotResponse, error) {
	builder := s.client.Lot.Create().
		SetLotNumber(req.LotNumber).
		SetProductType(lot.ProductType(req.ProductType)).
		SetPlannedQuantity(req.PlannedQuantity).
		SetActualQuantity(0).
		SetStatus(lot.Status("in_progress"))

	if req.EdgeID != "" {
		edgeID, err := uuid.Parse(req.EdgeID)
		if err != nil {
			return nil, fmt.Errorf("invalid edge_id: %w", err)
		}
		builder.SetEdgeID(edgeID)
	}

	if req.PLCLotNumber != "" {
		builder.SetPLCLotNumber(req.PLCLotNumber)
	}
	if req.OrderCode != "" {
		builder.SetOrderCode(req.OrderCode)
	}
	if req.ProductSpec != "" {
		builder.SetProductSpec(req.ProductSpec)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toLotResponse(created), nil
}
```

`ListLots` 的过滤参数由 `orderID` 改为 `edgeID`：

```go
// ListLots 查询批次列表
func (s *LotService) ListLots(ctx context.Context, page, pageSize int, edgeID, status string) ([]*LotResponse, int, error) {
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

`toLotResponse` 改为（注意参数名不要与外层变量冲突）：

```go
// toLotResponse 转换为响应格式
func (s *LotService) toLotResponse(l *ent.Lot) *LotResponse {
	var progress float64
	if l.PlannedQuantity > 0 {
		progress = float64(l.ActualQuantity) / float64(l.PlannedQuantity) * 100
	}

	resp := &LotResponse{
		ID:              l.ID.String(),
		LotNumber:       l.LotNumber,
		PLCLotNumber:    l.PLCLotNumber,
		OrderCode:       l.OrderCode,
		ProductType:     string(l.ProductType),
		ProductSpec:     l.ProductSpec,
		Status:          string(l.Status),
		PlannedQuantity: l.PlannedQuantity,
		ActualQuantity:  l.ActualQuantity,
		Progress:        progress,
		CreatedAt:       l.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       l.UpdatedAt.Format(time.RFC3339),
	}

	if l.EdgeID != uuid.Nil {
		resp.EdgeID = l.EdgeID.String()
	}
	if !l.StartTime.IsZero() {
		str := l.StartTime.Format(time.RFC3339)
		resp.StartTime = &str
	}
	if !l.EndTime.IsZero() {
		str := l.EndTime.Format(time.RFC3339)
		resp.EndTime = &str
	}

	return resp
}
```

`GetLot`、`UpdateLotStatus`、`DeleteLot` 三个方法不需要改动，保持原样。

需要在 import 中补上 `fmt` 与 `time`（`time` 用于 `Format(time.RFC3339)`）。

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/service/... -run TestLotService -v`

预期：`PASS`，两个测试均通过。

- [ ] **Step 8: 验证中文未损坏**

```bash
grep -c "批次" internal/service/lot.go
```

预期：输出大于 0（中文注释正常）。

- [ ] **Step 9: 提交**

```bash
git add internal/service/
git commit -m "$(cat <<'EOF'
refactor: 删除OrderService并让LotService匹配V3 Schema

Lot移除order_id依赖，改用edge_id/plc_lot_number/order_code。
新增Service层测试脚手架（内存SQLite）。

Refs: #17
EOF
)"
```

---

### Task 2: API 层删除 OrderHandler，LotHandler 适配新字段

**Files:**
- Delete: `api/center/v1/order.go`
- Modify: `api/center/v1/lot.go:79-92`（`ListLots` 的查询参数）
- Modify: `internal/server/center.go:105-115`（移除 orders 路由组）

**Interfaces:**
- Consumes: `service.LotService`（Task 1 产出的 `ListLots(ctx, page, pageSize, edgeID, status)` 签名）
- Produces: Center 的 `/v1/lots` 路由可用；`/v1/orders` 不再存在

- [ ] **Step 1: 删除 OrderHandler**

```bash
git rm api/center/v1/order.go
```

- [ ] **Step 2: 修改 LotHandler 的查询参数**

在 `api/center/v1/lot.go` 的 `ListLots` 中，把 `orderID` 相关的三处改为 `edgeID`。

将：

```go
	orderID := c.Query("order_id")
	status := c.Query("status")

	lots, total, err := h.lotService.ListLots(c.Request.Context(), page, pageSize, orderID, status)
```

改为：

```go
	edgeID := c.Query("edge_id")
	status := c.Query("status")

	lots, total, err := h.lotService.ListLots(c.Request.Context(), page, pageSize, edgeID, status)
```

同时把函数上方的 Swagger 注释中的 `@Param order_id query string false "订单ID"` 改为 `@Param edge_id query string false "边端设备ID"`。

**顺带修正本文件第 91 行已存在的实参顺序 bug**：`api.PageSuccess` 的签名是 `(data, page, pageSize, total)`，但调用处传的是 `(lots, total, page, pageSize)`。改为：

```go
	c.JSON(http.StatusOK, api.PageSuccess(lots, page, pageSize, total))
```

- [ ] **Step 3: 移除 Center 的 orders 路由**

在 `internal/server/center.go` 的 `registerRoutes` 中删除以下整块：

```go
		// 订单管理
		orderService := service.NewOrderService(s.client)
		orderHandler := centerv1.NewOrderHandler(orderService)

		orders := authorized.Group("/orders")
		{
			orders.POST("", orderHandler.CreateOrder)
			orders.GET("", orderHandler.ListOrders)
			orders.GET("/:id", orderHandler.GetOrder)
			orders.DELETE("/:id", orderHandler.DeleteOrder)
		}

```

**启动日志中的过期端点提示**位于 `cmd/center-server/main.go:55`，在 Step 4 一并修正。

- [ ] **Step 4: 修正启动日志中的过期端点提示**

在 `cmd/center-server/main.go` 第 55 行，将：

```go
	log.Println("   - Orders: http://localhost:8080/v1/orders")
```

改为：

```go
	log.Println("   - Lots:   http://localhost:8080/v1/lots")
```

并同时把第 22 行的默认 DSN 修正为实际使用的密码：

```go
	dsn := getEnv("DATABASE_URL", "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable")
```

- [ ] **Step 5: 验证编译**

Run: `go build ./cmd/center-server/ ./api/... ./internal/server/`

预期：无输出（成功）。

- [ ] **Step 6: 提交**

```bash
git add api/center/v1/lot.go internal/server/center.go cmd/center-server/main.go
git commit -m "$(cat <<'EOF'
refactor: 移除Order API并修正Center默认DSN与分页实参顺序

删除api/center/v1/order.go及其路由。修正PageSuccess调用处
参数顺序错误（原为data,total,page,pageSize）。
默认DSN改为与Docker容器一致的密码。

Refs: #17
EOF
)"
```

---

### Task 3: 修复 Center 同步层的 Order/Project 残留

**Files:**
- Modify: `internal/sync/center/handler.go:86-117`（`createLot`）
- Modify: `internal/sync/center/provider.go`（删除 `getProjects`）
- Modify: `internal/sync/center/sync_test.go`（修正 `Project` 用法与枚举值）

**Interfaces:**
- Consumes: `ent.Client`、`models.UploadEntry`/`UploadRequest`/`UploadResponse`、`models.BaseDataPullRequest`/`BaseDataPullResponse`
- Produces:
  - `UploadHandler.HandleUpload(ctx, *models.UploadRequest) (*models.UploadResponse, error)`
  - `BaseDataProvider.HandlePullRequest(ctx, *models.BaseDataPullRequest) (*models.BaseDataPullResponse, error)`，支持的表名为 `spinning_lines`、`grades`

- [ ] **Step 1: 修正 createLot 的字段映射**

在 `internal/sync/center/handler.go` 中，把 `createLot` 内从 `orderID, _ := uuid.Parse(...)` 到 `Save(ctx)` 的整段替换为：

```go
	edgeID, err := uuid.Parse(getString(data, "edge_id"))
	if err != nil {
		edgeID = uuid.Nil
	}

	builder := h.client.Lot.Create().
		SetID(id).
		SetLotNumber(getString(data, "lot_number")).
		SetProductType(lot.ProductType(getString(data, "product_type"))).
		SetPlannedQuantity(getInt(data, "planned_quantity")).
		SetActualQuantity(getInt(data, "actual_quantity")).
		SetStatus(lot.Status(getString(data, "status"))).
		SetNillableStartTime(getTimePtr(data, "start_time")).
		SetNillableEndTime(getTimePtr(data, "end_time"))

	if edgeID != uuid.Nil {
		builder.SetEdgeID(edgeID)
	}
	if v := getString(data, "plc_lot_number"); v != "" {
		builder.SetPLCLotNumber(v)
	}
	if v := getString(data, "order_code"); v != "" {
		builder.SetOrderCode(v)
	}
	if v := getString(data, "product_spec"); v != "" {
		builder.SetProductSpec(v)
	}

	_, err = builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}
```

**注意**：该函数原有的幂等检查写法是错误的 —— 它用 `h.client.Lot.Query().Where().Count(ctx)` 统计**全表**行数，只要表里有一条记录就跳过所有后续写入。改为按 `id` 精确查重：

```go
	// 检查是否已存在（幂等性）
	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	exists, err := h.client.Lot.Query().Where(lot.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("⚠️  Lot %s already exists, skipping", id)
		return nil
	}
```

这段替换原 `createLot` 开头的 `id, _ := uuid.Parse(...)` + `exists, err := ...Count(ctx)` 部分。

- [ ] **Step 2: 对 createBobbin 做同样的幂等修正**

`createBobbin` 存在完全相同的全表计数 bug。把：

```go
	id, _ := uuid.Parse(getString(data, "id"))
	lotID, _ := uuid.Parse(getString(data, "lot_id"))

	// 检查是否已存在
	exists, err := h.client.Bobbin.Query().Where().Count(ctx)
	if err != nil {
		return err
	}
	if exists > 0 {
		log.Printf("⚠️  Bobbin %s already exists, skipping", id)
		return nil
	}
```

替换为：

```go
	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotID, err := uuid.Parse(getString(data, "lot_id"))
	if err != nil {
		return fmt.Errorf("invalid lot_id %q: %w", getString(data, "lot_id"), err)
	}

	// 检查是否已存在（幂等性）
	exists, err := h.client.Bobbin.Query().Where(bobbin.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("⚠️  Bobbin %s already exists, skipping", id)
		return nil
	}
```

- [ ] **Step 3: 删除 provider 中的 Project 支持**

在 `internal/sync/center/provider.go`：

1. `getTableData` 的 switch 中删除 `case "projects": return p.getProjects(ctx)`。
2. 新增 `case "grades": return p.getGrades(ctx)`。
3. 删除整个 `getProjects` 函数。
4. 新增 `getGrades` 函数：

```go
// getGrades 获取等级基础数据
func (p *BaseDataProvider) getGrades(ctx context.Context) ([]map[string]interface{}, error) {
	grades, err := p.client.Grade.Query().
		Where(grade.IsActiveEQ(true)).
		Order(ent.Asc(grade.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(grades))
	for _, g := range grades {
		result = append(result, map[string]interface{}{
			"id":          g.ID.String(),
			"grade_type":  string(g.GradeType),
			"grade_code":  g.GradeCode,
			"grade_name":  g.GradeName,
			"description": g.Description,
			"sort_order":  g.SortOrder,
			"is_active":   g.IsActive,
		})
	}

	return result, nil
}
```

**字段名注意：** `Grade` 的启用标志字段是 `is_active`（不是 `enabled`），谓词是 `grade.IsActiveEQ(...)`，访问器是 `g.IsActive`。

5. 在 import 中补上 `"github.com/yourusername/igh-silkroad/internal/database/ent/grade"`。`uuid` 的 import 在删除 `getProjects` 后可能不再需要 —— 检查 `getSpinningLines` 是否还用到 `uuid.Nil`（用到则保留）。

- [ ] **Step 4: 修正同步层测试**

`internal/sync/center/sync_test.go` 有四处与当前 Schema 不符，逐一修改：

**4a. 测试数据库 DSN（第 17 行附近）** —— 密码错误，且 `igh_test` 库不存在，导致测试永远 `t.Skip`：

```go
	dsn := "postgres://igh:igh@localhost:5432/igh_test?sslmode=disable"
```

改为：

```go
	dsn := "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable"
```

**4b. 删除 Project 与 Order 的准备工作**（`TestUploadHandler_HandleUpload` 开头第 39–60 行）—— 这两个实体已不存在。整段 `client.Project.Create()...Save(ctx)` 与 `client.Order.Create()...Save(ctx)` 以及它们的 `if err != nil` 检查全部删除。该测试真正需要的前置数据只是下面那个 `LotID`，不需要数据库里先有批次。

**4c. 修正上传载荷的字段**（第 78–87 行附近）—— 把：

```go
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-UPLOAD-001",
					"order_id":         order.ID.String(),
					"product_type":     "FDY",
					"product_spec":     "150D/48F",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
```

改为：

```go
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-UPLOAD-001",
					"plc_lot_number":   "PLC-UPLOAD-001",
					"product_type":     "FDY",
					"product_spec":     "150D/48F",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
```

**4d. 修正 `TestBaseDataProvider_HandlePullRequest` 的字段名与枚举**（第 128–135 行附近）：

- `SetPositionCount(48)` → `SetCapacity(48)`（`SpinningLine` Schema 中该字段名为 `capacity`）
- `SetStatus("active")` → `SetStatus("running")`（`SpinningLine` 的状态枚举是 `idle/running/maintenance/offline`，没有 `active`）

**4e. 若 `client.Project` / `client.Order` 的引用出现在其它测试函数中**，用以下命令定位并同样删除：

```bash
grep -n "Project\|Order" internal/sync/center/sync_test.go
```

预期：修改后该命令**无输出**。

- [ ] **Step 5: 运行同步层测试**

Run: `go test ./internal/sync/center/... -v`

预期：`PASS`。

- [ ] **Step 6: 验证中文未损坏**

```bash
grep -c "批次\|等级" internal/sync/center/handler.go internal/sync/center/provider.go
```

预期：两个文件各输出大于 0。

- [ ] **Step 7: 提交**

```bash
git add internal/sync/center/
git commit -m "$(cat <<'EOF'
fix: 修复Center同步层的Order/Project残留与幂等检查

- createLot改用edge_id/plc_lot_number/order_code
- 修正createLot/createBobbin的幂等检查（原为全表计数，会跳过所有写入）
- provider删除Project支持，新增Grade基础数据下发
- 修正测试中的SpinningLine字段名与枚举值

Refs: #17
EOF
)"
```

---

### Task 4: 清理工具命令与 ent_edge 死代码

**Files:**
- Modify: `cmd/seed/main.go`（删除 `seedProjects`）
- Modify: `cmd/test-db/main.go`（删除 Project CRUD 段）
- Modify: `cmd/migrate/main.go`（修正默认 DSN）
- Delete: `internal/database/ent_edge/`（整目录）
- Delete: `internal/sync/edge/uploader.go`
- Delete: `internal/sync/edge/downloader.go`
- Delete: `internal/sync/edge/sync_test.go`

**Interfaces:**
- Consumes: 无
- Produces: 全仓库 `go build ./...` 与 `go test ./...` 通过

**为什么删 `ent_edge`：** 它是一套独立的 Ent schema（29 文件），其 `lot.go:35` 仍使用 `order_id`，与 V3 模型矛盾。实际运行的 Edge 服务（`cmd/edge-server/main.go`、`internal/server/edge.go`、`internal/sync/edge/client.go`）全部使用主 `ent` 包。删除依赖它的 `uploader.go`/`downloader.go` 后，该包无任何引用。

- [ ] **Step 1: 先证明 ent_edge 无人引用（不要只靠 grep）**

```bash
grep -rn "ent_edge" --include="*.go" . | grep -v "^./internal/database/ent_edge/" | grep -v "^./internal/sync/edge/"
```

预期：**除 `internal/sync/edge/` 外无输出**。若出现其它文件，停止并报告 —— 说明该包仍被使用，不能删除。

- [ ] **Step 2: 删除 ent_edge 及其唯一依赖者**

```bash
git rm -r internal/database/ent_edge/
git rm internal/sync/edge/uploader.go internal/sync/edge/downloader.go internal/sync/edge/sync_test.go
```

- [ ] **Step 3: 统一全部默认 DSN**

`igh:igh` 这个密码与 Docker 容器 `igh-postgres` 的实际密码 `igh_dev_password` 不符，导致**所有**以默认值运行的命令必然认证失败。实测有 4 处代码位置，逐一改为 `igh:igh_dev_password`：

| 文件 | 行 | 变量 |
|------|-----|------|
| `cmd/migrate/main.go` | 51 | `dsn = "postgres://igh:igh@..."` |
| `cmd/center-server/main.go` | 22 | `getEnv("DATABASE_URL", "postgres://igh:igh@...")` |
| `cmd/seed/main.go` | 19 | `dsn = "postgres://igh:igh@..."` |
| `cmd/test-db/main.go` | 46 | `dsn = "postgres://igh:igh@..."` |

改完后确认无残留：

```bash
grep -rn "igh:igh@localhost" --include="*.go" .
```

预期：**无输出**。

- [ ] **Step 4: 修正 cmd/seed**

在 `cmd/seed/main.go` 中：

1. 删除第 43 行附近的 `seedProjects(ctx, client)` 调用。
2. 删除整个 `seedProjects` 函数（约第 114 行起）。
3. 新增种子数据函数，替换为新的核心实体：

```go
// seedCoreData 创建核心业务测试数据
func seedCoreData(ctx context.Context, client *ent.Client) error {
	log.Println("🌱 Seeding core data...")

	// 边端设备
	edge, err := client.Edge.Create().
		SetEdgeCode("edge-001").
		SetEdgeName("一号边端").
		SetIPAddress("192.168.1.101").
		SetStatus(edge.StatusOnline).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create edge: %w", err)
	}
	log.Printf("  ✓ Created edge: %s (ID: %s)", edge.EdgeCode, edge.ID)

	// 纺丝线体
	line, err := client.SpinningLine.Create().
		SetLineName("A线").
		SetLineNumber("LINE-A").
		SetEdgeID(edge.ID).
		SetLocation("一车间").
		SetCapacity(48).
		SetStatus(spinningline.StatusRunning).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create spinning line: %w", err)
	}
	log.Printf("  ✓ Created spinning line: %s (ID: %s)", line.LineName, line.ID)

	// 等级基础数据
	for _, g := range []struct {
		code string
		name string
	}{
		{"A", "优等品"},
		{"B", "一等品"},
		{"C", "合格品"},
		{"D", "等外品"},
	} {
		created, err := client.Grade.Create().
			SetGradeType("final").
			SetGradeCode(g.code).
			SetGradeName(g.name).
			SetSortOrder(0).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create grade %s: %w", g.code, err)
		}
		log.Printf("  ✓ Created grade: %s", created.GradeCode)
	}

	// 批次
	lot, err := client.Lot.Create().
		SetLotNumber("FDY-2026-001-01").
		SetEdgeID(edge.ID).
		SetPLCLotNumber("PLC-2026-001").
		SetProductType(lot.ProductTypeFDY).
		SetProductSpec("150D/48F").
		SetPlannedQuantity(4800).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}
	log.Printf("  ✓ Created lot: %s (ID: %s)", lot.LotNumber, lot.ID)

	return nil
}
```

4. 在 import 中补上 `"fmt"`、`"github.com/yourusername/igh-silkroad/internal/database/ent/edge"`、`"github.com/yourusername/igh-silkroad/internal/database/ent/lot"`、`"github.com/yourusername/igh-silkroad/internal/database/ent/spinningline"`。

**注意：** 生成后的枚举常量名（如 `edge.StatusOnline`、`lot.ProductTypeFDY`）需在 Step 6 编译时确认。若名称不同，以 `go build` 报错提示的实际名称为准（生成代码中枚举值常量形如 `<Type><Value>`，Value 首字母大写）。

- [ ] **Step 5: 修正 cmd/test-db**

在 `cmd/test-db/main.go` 中：

1. 删除第 63–100 行附近的整个 Project CRUD 测试段。
2. 删除对 `internal/database/ent_edge` 的 import（第 11 行）及其使用处（第 126 行附近的 `ent_edge.NewClient`）。
3. 若删除后 `client` 变量只剩 SQLite 部分，保留主 `ent` 客户端的连接逻辑不变。

- [ ] **Step 6: 全仓库构建与测试**

```bash
go build ./... && go test ./... 2>&1 | tail -20
```

预期：`go build` 无输出；`go test` 所有包 `ok` 或 `no test files`，无 `FAIL`。

- [ ] **Step 7: 验证中文未损坏**

```bash
grep -c "边端\|批次\|等级" cmd/seed/main.go
```

预期：输出大于 0。

- [ ] **Step 8: 提交**

```bash
git add cmd/ internal/sync/edge/ internal/database/
git commit -m "$(cat <<'EOF'
chore: 删除ent_edge死代码并修复工具命令

ent_edge是第二套Ent schema（仍含order_id），未被任何服务引用——
实际Edge服务使用主ent包。同步删除依赖它的uploader/downloader。
seed与test-db改用V3实体，migrate默认DSN密码对齐Docker容器。

Refs: #17
EOF
)"
```

---

## Phase 1 — Schema 调整与双库迁移

### Task 5: 调整 Barrel↔Doffing 关系（Barrel 归属 Lot）

设计文档 §3.1：`Doffing` 与 `Barrel` 当前是语义矛盾的双向一对多，且 `Barrel.doffing_id` 为 `Required()` —— 意味着创建落纱桶前必须先有落纱记录，与「一次落纱产生一桶丝饼」的业务顺序相反。

**Files:**
- Modify: `internal/database/ent/schema/barrel.go:34-36, 80-85, 102-104`
- Modify: `internal/database/ent/schema/doffing.go:107-109`
- Regenerate: `internal/database/ent/**`

**Interfaces:**
- Consumes: 无
- Produces:
  - `Barrel` 不再有 `DoffingID` 字段、`SetDoffingID`/`ClearDoffingID` 方法、`QueryDoffing()` 边
  - `Doffing` 不再有 `QueryBarrels()` 边
  - `Barrel.Edges` 保留 `lot`（`Required`+`Unique`）与 `bobbins`

- [ ] **Step 1: 记录回退锚点**

```bash
git rev-parse HEAD
```

保存输出。

- [ ] **Step 2: 删除 Barrel 的 doffing_id 字段**

在 `internal/database/ent/schema/barrel.go` 中删除以下块：

```go
		// 关联落纱记录
		field.UUID("doffing_id", uuid.UUID{}).
			Comment("关联落纱记录ID"),

```

- [ ] **Step 3: 删除 Barrel 的 doffing 边**

在 `barrel.go` 的 `Edges()` 中删除：

```go
		// 一个落纱桶属于一个落纱记录
		edge.From("doffing", Doffing.Type).
			Ref("barrels").
			Field("doffing_id").
			Required().
			Unique(),

```

删除后 `Edges()` 应为：

```go
// Edges of the Barrel.
func (Barrel) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个落纱桶属于一个批号
		edge.From("lot", Lot.Type).
			Ref("barrels").
			Field("lot_id").
			Required().
			Unique(),

		// 一个落纱桶有多个丝饼
		edge.To("bobbins", Bobbin.Type),
	}
}
```

- [ ] **Step 4: 删除 Barrel 的 doffing_id 索引**

在 `barrel.go` 的 `Indexes()` 中删除：

```go
		// 落纱ID索引
		index.Fields("doffing_id"),

```

- [ ] **Step 5: 删除 Doffing 的 barrels 反向边**

在 `internal/database/ent/schema/doffing.go` 的 `Edges()` 中删除：

```go
		// 一个落纱记录有多个落纱桶
		edge.To("barrels", Barrel.Type),
```

删除后 `Edges()` 应为：

```go
// Edges of the Doffing.
func (Doffing) Edges() []ent.Edge {
	return []ent.Edge{
		// 一个落纱记录属于一个批次
		edge.From("lot", Lot.Type).
			Ref("doffings").
			Field("lot_id").
			Required().
			Unique(),
	}
}
```

- [ ] **Step 6: 确认 Lot 的 barrels 边仍然保留**

`internal/database/ent/schema/lot.go` 中的 `edge.To("barrels", Barrel.Type)` 是 Barrel 的 `Ref("barrels")` 目标，**必须保留**。确认该行存在：

```bash
grep -n 'barrels' internal/database/ent/schema/lot.go
```

预期：输出包含 `edge.To("barrels", Barrel.Type)`。

- [ ] **Step 7: 重新生成 Ent 代码**

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema
```

- [ ] **Step 8: 检查生成结果只包含预期改动**

```bash
git status --porcelain internal/database/ent/ | head -20
git diff --stat internal/database/ent/schema/
```

预期：`schema/` 下只有 `barrel.go`、`doffing.go` 被修改；生成的产物中出现 `barrel_query.go`、`barrel_update.go`、`doffing_query.go` 等的变化，且**没有新增目录**（如 `internal/database/ent/generate/`）。若出现非预期的新目录，说明生成 target 不对，停止并排查。

- [ ] **Step 9: 验证编译**

```bash
go build ./...
```

预期：**成功（无输出）**。已实测确认 `internal/database/ent/` 之外没有任何代码引用 `DoffingID`/`SetDoffingID`/`SetDoffing`，因此删除该字段不会破坏调用方。

- [ ] **Step 10: 若上一步失败，修正受影响的调用方**

若 `go build ./...` 意外失败，对每一处报错删除其中的 `SetDoffingID(...)` / `SetDoffing(...)` 调用与对应的 `doffingID` 变量声明。若某处逻辑依赖 doffing 关联，改为依赖 `lot_id`（Barrel 现在只归属 Lot）。

- [ ] **Step 11: 运行全量测试**

Run: `go build ./... && go test ./... 2>&1 | tail -20`

预期：构建成功，测试无 `FAIL`。

- [ ] **Step 12: 提交**

```bash
git add internal/database/ent/ internal/sync/ internal/service/ api/
git commit -m "$(cat <<'EOF'
refactor: Barrel归属Lot，解除与Doffing的强制关联

删除Barrel.doffing_id（Required导致必须先有落纱记录才能建桶，
与业务顺序相反）。落纱桶现在只通过lot_id归属批次。

Refs: #17
EOF
)"
```

---

### Task 6: 七个实体添加同步元数据字段

设计文档 §3.2：`Lot`/`Barrel`/`Bobbin`/`Doffing`/`Module`/`Pallet`/`Carton` 在 Edge 端产生、需上传到 Center，因此各需 `sync_status`/`synced_at`/`sync_retry_count`。

**Files:**
- Modify: `internal/database/ent/schema/lot.go`
- Modify: `internal/database/ent/schema/barrel.go`
- Modify: `internal/database/ent/schema/bobbin.go`
- Modify: `internal/database/ent/schema/doffing.go`
- Modify: `internal/database/ent/schema/module.go`
- Modify: `internal/database/ent/schema/pallet.go`
- Modify: `internal/database/ent/schema/carton.go`
- Create: `internal/database/ent/schema/schema_test.go`
- Regenerate: `internal/database/ent/**`

**Interfaces:**
- Consumes: 无
- Produces:
  - 每个上述实体新增字段 `sync_status`（enum `pending`/`synced`/`failed`，默认 `pending`）、`synced_at`（optional time）、`sync_retry_count`（int，默认 0）
  - 配套生成方法：`SetSyncStatus(...)`、`SetNillableSyncedAt(...)`、`SetSyncRetryCount(...)`、`AddSyncRetryCount(...)`
  - 配套谓词：`Xxx.SyncStatusEQ(...)`、`Xxx.SyncRetryCountLT(...)`
  - 生成常量：`lot.SyncStatusPending`、`lot.SyncStatusSynced`、`lot.SyncStatusFailed`（其余实体同形）

- [ ] **Step 1: 写失败的测试**

创建 `internal/database/ent/schema/schema_test.go`：

```go
package schema_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

func newTestClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:schematest%d?mode=memory&cache=shared&_fk=1",
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

func TestLot_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	if created.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, lot.SyncStatusPending)
	}
	if created.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", created.SyncRetryCount)
	}
	if !created.SyncedAt.IsZero() {
		t.Errorf("SyncedAt = %v, want zero", created.SyncedAt)
	}
}

func TestBarrel_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-002").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-SYNC-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating barrel: %v", err)
	}

	if created.SyncStatus != barrel.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, barrel.SyncStatusPending)
	}
	if created.Capacity != 9 {
		t.Errorf("Capacity = %d, want default 9", created.Capacity)
	}
}

func TestBobbin_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-003").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Bobbin.Create().
		SetBobbinNumber("BOB-SYNC-001").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(12.5).
		SetNetWeight(11.8).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating bobbin: %v", err)
	}

	if created.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, bobbin.SyncStatusPending)
	}
}

func TestSyncStatusQueryByPending(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-PENDING-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx); err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	pending, err := client.Lot.Query().
		Where(lot.SyncStatusEQ(lot.SyncStatusPending)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if pending != 1 {
		t.Errorf("pending count = %d, want 1", pending)
	}

	synced, err := client.Lot.Query().
		Where(lot.SyncStatusEQ(lot.SyncStatusSynced)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if synced != 0 {
		t.Errorf("synced count = %d, want 0", synced)
	}
}

func TestSyncRetryCountAccumulates(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Lot.Create().
		SetLotNumber("LOT-RETRY-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	updated, err := client.Lot.UpdateOneID(created.ID).
		AddSyncRetryCount(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed updating lot: %v", err)
	}
	if updated.SyncRetryCount != 1 {
		t.Errorf("SyncRetryCount = %d, want 1", updated.SyncRetryCount)
	}
}

func TestBarrelCanBeCreatedWithoutDoffing(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-NODOFF-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 设计文档 §3.1：建桶不需要先有落纱记录
	created, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-NODOFF-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("barrel requires no doffing: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}
```

该测试文件需要 import 各实体的谓词包与主包。在文件顶部 import 块中补充：

```go
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/database/ent/schema/ -v`

预期：编译失败，报 `undefined: lot.SyncStatusPending`、`created.SyncStatus undefined` 等。

- [ ] **Step 3: 向 7 个 Schema 添加同步字段**

在以下 7 个文件中，各自找到 `field.Time("created_at").` 那一行，**在其正上方**插入：

```go
		// 同步元数据（Edge → Center）
		field.Enum("sync_status").
			Values("pending", "synced", "failed").
			Default("pending").
			Comment("同步状态"),

		field.Time("synced_at").
			Optional().
			Comment("同步时间"),

		field.Int("sync_retry_count").
			Default(0).
			NonNegative().
			Comment("同步重试次数"),

```

需要修改的文件（插入点统一以 `field.Time("created_at").` 为准。各文件该行上方的注释不完全相同 —— `lot.go`/`doffing.go` 是 `// 元数据`，`module.go` 是 `// 时间戳`，`barrel.go`/`bobbin.go`/`pallet.go`/`carton.go` 则直接承接上一字段的 `Comment(...)`，没有分节注释。这不影响插入位置）：

1. `internal/database/ent/schema/lot.go`
2. `internal/database/ent/schema/barrel.go`
3. `internal/database/ent/schema/bobbin.go`
4. `internal/database/ent/schema/doffing.go`
5. `internal/database/ent/schema/module.go`
6. `internal/database/ent/schema/pallet.go`
7. `internal/database/ent/schema/carton.go`

- [ ] **Step 4: 为每个实体添加 sync_status 索引**

在同样的 7 个文件的 `Indexes()` 函数中，各添加一条。**插入位置按文件区分**（已实测各文件的实际注释）：

| 文件 | 插入位置 |
|------|---------|
| `lot.go` | `// 创建时间倒序索引` 之前 |
| `barrel.go` | `// 创建时间倒序索引` 之前 |
| `pallet.go` | `// 创建时间倒序索引` 之前 |
| `carton.go` | `// 创建时间倒序索引` 之前 |
| `bobbin.go` | `// 复合索引：批次+状态` 之前 |
| `doffing.go` | `// 落纱时间索引` 之前 |
| `module.go` | `// 第一个桶ID索引` 之前 |

插入内容：

```go
		// 同步状态索引（Edge端扫描待同步记录）
		index.Fields("sync_status"),
```

- [ ] **Step 5: 重新生成 Ent 代码**

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema
```

- [ ] **Step 6: 检查生成结果**

```bash
git status --porcelain internal/database/ent/ | wc -l
git diff --stat internal/database/ent/schema/
```

预期：`schema/` 下恰好 7 个文件被修改；生成产物有变化；无新增目录。

- [ ] **Step 7: 运行测试确认通过**

Run: `go test ./internal/database/ent/schema/ -v`

预期：`PASS`，6 个测试全通过。

若报 `undefined: lot.SyncStatusPending`，说明生成代码中的枚举常量命名与预期不同。查看实际名称：

```bash
grep -n "SyncStatusPending\|SyncStatus" internal/database/ent/lot/lot.go | head
```

以实际名称为准修正测试。

- [ ] **Step 8: 全量构建与测试**

Run: `go build ./... && go test ./... 2>&1 | tail -20`

预期：无 `FAIL`。

- [ ] **Step 9: 验证中文未损坏**

```bash
grep -c "同步状态\|同步时间\|同步重试次数" internal/database/ent/schema/*.go | grep -v ":0"
```

预期：7 个文件各输出大于 0，无 `:0` 行。

- [ ] **Step 10: 提交**

```bash
git add internal/database/ent/
git commit -m "$(cat <<'EOF'
feat: 为七个Edge端实体添加同步元数据字段

Lot/Barrel/Bobbin/Doffing/Module/Pallet/Carton 各增
sync_status(pending/synced/failed)、synced_at、sync_retry_count，
并建立 sync_status 索引供Edge端扫描待同步记录。

Refs: #17
EOF
)"
```

---

### Task 7: Module / Pallet / Carton 添加 edge_id

设计文档 §3.2「已知缺口」：这三个实体由 Edge 端设备产生，但缺少 `edge_id`，Center 无法判定来源设备。

**Files:**
- Modify: `internal/database/ent/schema/module.go`
- Modify: `internal/database/ent/schema/pallet.go`
- Modify: `internal/database/ent/schema/carton.go`
- Modify: `internal/database/ent/schema/schema_test.go`
- Regenerate: `internal/database/ent/**`

**Interfaces:**
- Consumes: Task 6 产出的同步字段
- Produces: `Module`/`Pallet`/`Carton` 各新增 `edge_id`（optional UUID）；生成 `SetEdgeID`/`SetNillableEdgeID`/`ClearEdgeID` 与谓词 `Module.EdgeIDEQ(...)`、`Module.EdgeIDNotNil()` 等

**为什么用普通字段而不是 Ent 边：** 仓库既有模式中 `Doffing.spinning_line_id`、`SpinningLine.current_lot_id`、`Pallet.palletizer_id` 都是「普通 UUID 字段 + 索引」，没有反向边。跟随该模式，避免在 `Edge` 上为三个实体各加一条 `edge.To` 的双向记账。

- [ ] **Step 1: 写失败的测试**

在 `internal/database/ent/schema/schema_test.go` 末尾追加：

```go
func TestModule_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	// 不带 edge_id 也能创建（Edge未注册场景）
	anonymous, err := client.Module.Create().
		SetModuleNumber("MOD-ANON-001").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating module without edge: %v", err)
	}
	if anonymous.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", anonymous.EdgeID)
	}

	// 带 edge_id 创建
	edge, err := client.Edge.Create().
		SetEdgeCode("edge-mod-01").
		SetEdgeName("模块测试边端").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	tagged, err := client.Module.Create().
		SetModuleNumber("MOD-TAGGED-001").
		SetEdgeID(edge.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating module with edge: %v", err)
	}
	if tagged.EdgeID != edge.ID {
		t.Errorf("EdgeID = %v, want %v", tagged.EdgeID, edge.ID)
	}

	// 按 edge_id 过滤
	count, err := client.Module.Query().
		Where(module.EdgeIDEQ(edge.ID)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("modules for edge = %d, want 1", count)
	}
}

func TestPallet_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-PALLET-EDGE-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Pallet.Create().
		SetPalletCode("PALLET-EDGE-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating pallet: %v", err)
	}
	if created.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", created.EdgeID)
	}
}

func TestCarton_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Carton.Create().
		SetCartonNumber("CARTON-EDGE-001").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating carton: %v", err)
	}
	if created.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", created.EdgeID)
	}
}
```

在 import 块中补充：

```go
	"github.com/yourusername/igh-silkroad/internal/database/ent/module"
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/database/ent/schema/ -run TestModule_EdgeIDOptional -v`

预期：编译失败，报 `anonymous.EdgeID undefined` 与 `undefined: module.EdgeIDEQ`。

- [ ] **Step 3: 向三个 Schema 添加 edge_id**

在以下 3 个文件的 `Fields()` 中，紧跟在唯一编号字段（`module_number` / `pallet_code` / `carton_number`）之后插入：

```go
		// 来源边端设备（Edge端设备产生）
		field.UUID("edge_id", uuid.UUID{}).
			Optional().
			Comment("来源边端设备ID"),

```

即：

1. `module.go` —— 插在 `field.String("module_number")...Comment("吊车编号"),` 之后
2. `pallet.go` —— 插在 `field.String("pallet_code")...Comment("托盘编号（条码）"),` 之后
3. `carton.go` —— 插在 `field.String("carton_number")...Comment("纸箱编号（条码）"),` 之后

- [ ] **Step 4: 为三个实体添加 edge_id 索引**

在 `Indexes()` 中各添加：

```go
		// 来源边端设备索引
		index.Fields("edge_id"),
```

- [ ] **Step 5: 重新生成 Ent 代码**

```bash
go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema
```

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/database/ent/schema/ -v`

预期：`PASS`，9 个测试全通过。

- [ ] **Step 7: 全量构建与测试**

Run: `go build ./... && go test ./... 2>&1 | tail -20`

预期：无 `FAIL`。

- [ ] **Step 8: 提交**

```bash
git add internal/database/ent/
git commit -m "$(cat <<'EOF'
feat: Module/Pallet/Carton 添加来源边端设备字段

这三个实体由Edge端设备（物流/码垛/包装）产生，缺少edge_id导致
Center无法按设备归属统计。采用普通UUID字段+索引，与仓库既有的
spinning_line_id/current_lot_id/palletizer_id 模式一致。

Refs: #17
EOF
)"
```

---

### Task 8: 双库迁移与验证

**Files:**
- Modify: `docs/database-migration.md`（更新 Schema 变更章节）
- 无代码文件改动

**Interfaces:**
- Consumes: Task 5–7 生成的 Schema
- Produces: 两套数据库结构与 Schema 一致

- [ ] **Step 1: 记录回退锚点**

```bash
git rev-parse HEAD
```

保存输出。

- [ ] **Step 2: 记录迁移前数据量（Review Focus 1、3）**

```bash
docker exec igh-postgres psql -U igh -d igh -c "SELECT 'spinning_lines' t, count(*) FROM spinning_lines UNION ALL SELECT 'users', count(*) FROM users UNION ALL SELECT 'barrels', count(*) FROM barrels UNION ALL SELECT 'lots', count(*) FROM lots;"
```

预期：`spinning_lines` = 4，`users` = 4，`barrels` = 0，`lots` = 0。

**把这些数字记下来** —— 迁移后要逐一比对。

- [ ] **Step 3: 重新编译迁移工具**

```bash
go build -o bin/migrate.exe ./cmd/migrate
```

预期：无输出。

- [ ] **Step 4: 执行 Center 迁移**

```bash
./bin/migrate.exe center
```

预期输出末尾为：

```
✅ Center database schema migrated successfully
```

若报 `password authentication failed`，确认 `cmd/migrate/main.go` 的默认 DSN 已按 Task 4 Step 3 修正为 `igh_dev_password`。

- [ ] **Step 5: 验证 Center 迁移结果（Review Focus 2）**

```bash
docker exec igh-postgres psql -U igh -d igh -c "\d barrels" && docker exec igh-postgres psql -U igh -d igh -c "\d modules"
```

预期：

- `barrels` 表**不再包含** `doffing_id` 列
- `barrels` 包含 `sync_status`、`synced_at`、`sync_retry_count`
- `modules` 包含 `sync_status`、`synced_at`、`sync_retry_count`，且包含 `edge_id`

再检查其余五个实体的同步字段（`barrels`/`modules` 上面已看）：

```bash
docker exec igh-postgres psql -U igh -d igh -c "
SELECT table_name, column_name FROM information_schema.columns
WHERE column_name IN ('sync_status','synced_at','sync_retry_count','edge_id')
ORDER BY table_name, column_name;"
```

预期：`sync_status`/`synced_at`/`sync_retry_count` 出现在 **7 张表**上（`barrels`, `bobbins`, `cartons`, `doffings`, `lots`, `modules`, `pallets`）；`edge_id` 出现在 **5 张表**上（`lots`, `spinning_lines` 为原有，`modules`, `pallets`, `cartons` 为本次新增）。

- [ ] **Step 6: 验证既有数据未丢失（Review Focus 1、2）**

```bash
docker exec igh-postgres psql -U igh -d igh -c "SELECT 'spinning_lines' t, count(*) FROM spinning_lines UNION ALL SELECT 'users', count(*) FROM users;"
docker exec igh-postgres psql -U igh -d igh -c "SELECT DISTINCT sync_status FROM lots;"
```

预期：`spinning_lines` 仍为 4，`users` 仍为 4。第二条查询在 `lots` 为空时返回空集，属正常。

- [ ] **Step 7: 重复执行迁移，验证幂等（Review Focus 1）**

```bash
./bin/migrate.exe center
```

预期：再次输出 `✅ Center database schema migrated successfully`，无 `failed creating schema resources` 报错。若报错，说明迁移不可重复执行 —— 这是必须修复的缺陷，停止并报告。

- [ ] **Step 8: 执行 Edge 迁移**

```bash
rm -f edge-test.db && ./bin/migrate.exe edge "file:edge-test.db?cache=shared&_fk=1"
```

预期输出末尾为 `✅ Edge database schema migrated successfully`。

- [ ] **Step 9: 验证 Edge 迁移结果（Review Focus 3）**

用 Python 的 sqlite3 模块检查（本机无 `sqlite3` CLI，但 Python 3.14 可用）：

```bash
python -c "
import sqlite3
c = sqlite3.connect('edge-test.db')
for tbl in ('barrels', 'lots', 'modules'):
    cols = [r[1] for r in c.execute(f'PRAGMA table_info({tbl})')]
    print(f'{tbl}: {cols}')
    if tbl == 'barrels':
        assert 'doffing_id' not in cols, 'doffing_id still present!'
        assert 'sync_status' in cols, 'sync_status missing!'
c.close()
print('OK: barrels has no doffing_id, has sync_status')
"
```

预期：末行输出 `OK: barrels has no doffing_id, has sync_status`，且断言未触发。

**注意：** 若 Edge 数据库中原本有数据，此步骤只验证列结构。`barrels` 表在迁移前为空，因此不存在行丢失风险；但仍需确认迁移未报错（Step 8 的输出）。

- [ ] **Step 10: 更新迁移文档**

编辑 `docs/database-migration.md`，在「数据库Schema变更」章节的三张清单中加入本次改动：

`修改的表` 列表中补充：

```markdown
- 🔧 `barrels` - 删除doffing_id（改为归属Lot）
- 🔧 `modules`/`pallets`/`cartons` - 添加edge_id字段（来源边端设备）
- 🔧 七个Edge端实体(Lot/Barrel/Bobbin/Doffing/Module/Pallet/Carton) - 添加同步字段(sync_status/synced_at/sync_retry_count)
```

并在文件末尾的「提交记录」章节追加本次的 commit hash（用 `git log --oneline -4` 查看）。

- [ ] **Step 11: 验证文档中文完好**

```bash
grep -c "同步\|迁移\|边端" docs/database-migration.md
```

预期：输出大于 0。

- [ ] **Step 12: 提交**

```bash
git add docs/database-migration.md
git commit -m "$(cat <<'EOF'
docs: 补充同步字段与Barrel关系调整的迁移记录

Refs: #17
EOF
)"
```

---

## 完成标准

本计划结束时，以下命令必须全部通过：

```bash
go build ./...                                    # 无输出
go test ./... 2>&1 | grep -c FAIL                 # 输出 0
./bin/migrate.exe center                          # ✅ 成功（可重复执行）
./bin/migrate.exe edge "file:edge-test.db?cache=shared&_fk=1"   # ✅ 成功
```

且：

- `internal/service/order.go`、`api/center/v1/order.go`、`internal/database/ent_edge/` 均已不存在
- Center 库中 `barrels` 无 `doffing_id` 列
- 七个 Edge 端实体均有 `sync_status` 列，`Module`/`Pallet`/`Carton` 均有 `edge_id` 列
- `spinning_lines` 4 行、`users` 4 行数据仍在

## 后续计划

本计划只覆盖设计文档 §10 的 Phase 0–1。**Phase 2–7 需要各自独立的计划**，原因：Phase 2 起的 Service/API 代码依赖本阶段生成的具体 Ent API 名称（生成的枚举常量名、谓词名、`SetNillable*` 方法名），只有在本阶段落地后才能写出可执行的真实代码而非猜测。

Phase 2 计划建议范围：`Edge` / `SpinningLine` / `Lot` / `Grade` / `ProductConfig` 五个基础实体的 Service + API + 单元测试。

---

## Self-Review 记录

**Spec 覆盖检查：**

| Spec 章节 | 本计划中的任务 |
|-----------|--------------|
| §1.2 目标 1（清除 Order 代码） | Task 1, 2, 3, 4 |
| §1.2 目标 2（13 实体 Service/API） | **不在本计划** —— Phase 2+ |
| §1.2 目标 3（同步机制扩展） | Task 6 提供字段基础；机制扩展属 Phase 6 |
| §3.1 Barrel 关系调整 | Task 5 |
| §3.2 同步字段 | Task 6 |
| §3.2 已知缺口（edge_id） | Task 7 |
| §10 Phase 0 | Task 1–4 |
| §10 Phase 1 | Task 5–8 |

**发现的 spec 缺口（已在本计划中处理）：**

1. **`ent_edge` 第二套 Ent 包** —— spec 未提及。项目实际存在 `internal/database/ent_edge`（29 文件，含 `order_id`），未被任何活跃服务引用。已在 Task 4 删除。
2. **同步层的幂等检查 bug** —— `handler.go` 的 `createLot`/`createBobbin` 用全表 `Count()` 判重，表非空时会静默跳过所有写入。spec §6.2 要求幂等性但未发现此实现缺陷。已在 Task 3 修正。
3. **`api.PageSuccess` 实参顺序错误** —— 4 处调用中有 3 处把 `total` 与 `page` 传反。已在 Task 2 修正 `lot.go`；`bobbin.go`、`user.go`、`api/edge/v1/*.go` 的同类问题在 Phase 2+ 重写时处理。
4. **默认 DSN 密码错误，且分散在 4 个代码文件** —— `cmd/migrate`、`cmd/center-server`、`cmd/seed`、`cmd/test-db` 各有一份 `igh:igh`，与 Docker 容器实际的 `igh_dev_password` 不符，导致**所有**以默认值运行的程序认证失败。已在 Task 4 Step 3 统一为 4 处一并修正（原稿只提了 `cmd/migrate` 一处）。
5. **同步层测试永远被跳过** —— `sync_test.go` 的 DSN 指向不存在的 `igh_test` 库且密码错误，`ent.Open` 失败后 `t.Skip`，测试从不真正执行。已在 Task 3 Step 4a 修正为实际数据库。
6. **`Grade` 的启用字段是 `is_active` 不是 `enabled`** —— 原稿的 `getGrades` 草稿用了 `grade.EnabledEQ(...)`，实测该方法不存在。已改为 `grade.IsActiveEQ(true)`。
7. **Schema 插入锚点注释不统一** —— 只有 `lot.go`/`doffing.go` 有 `// 元数据`、`module.go` 有 `// 时间戳`，`barrel.go`/`bobbin.go`/`pallet.go`/`carton.go` 无分节注释。原稿的"插入到 `// 创建时间倒序索引` 之前"对 3 个文件不成立。已改为按文件列出各自的插入位置。
8. **Task 5 Step 9 的预期写反** —— 原稿预期构建失败，但实测无任何代码引用 `DoffingID`，构建应当成功。已修正预期并保留失败时的补救步骤。

**实测确认的事实（写入计划的前置事实表）：**

| 事实 | 验证 |
|------|------|
| Ent 生成命令幂等 | 未改 Schema 时运行，`internal/database/ent/` 零变动 |
| 无代码引用 `DoffingID` | `grep` 除生成代码外无输出 |
| `SpinningLine` 状态枚举为 `idle/running/maintenance/offline` | 生成代码 `StatusValidator` |
| `Lot.ProductTypeFDY`/`StatusInProgress` 常量存在 | 生成代码 `lot.go:157,180` |
| `Barrel.Capacity` 默认 9 | Schema `Default(9)` |
| `Grade` 字段为 `grade_code`/`grade_name`/`is_active`/`sort_order` | Schema + 生成代码 |
| 本机无 `sqlite3` CLI，但 Python 3.14 含 `sqlite3` 模块 | 实测 `python -c "import sqlite3"` 通过 |
| Center 库现有数据：`spinning_lines` 4 行、`users` 4 行 | `docker exec ... psql` 实测 |
