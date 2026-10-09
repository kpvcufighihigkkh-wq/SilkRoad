# Edge→Center 更新同步实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 Edge 侧对已存在记录的修改（称重、质检、落纱状态流转）真正同步到 Center，消除"报告成功却什么都没写"的假成功链路。

**Architecture:** 四环一起通：Edge 侧用 ent Hook 在业务写入时自动置脏（重置 `sync_status`/`sync_retry_count` 并清 `synced_at`），采集器据此收集；Center 侧把 create 改为 upsert，并识别唯一约束冲突按业务编号认领；跨端 `lot_id` 不采信载荷、改按 `lot_number` 解引用。数据流严格单向 Edge → Center。

**Tech Stack:** Go 1.26.4、Ent v0.14.6、Gin v1.12.0、PostgreSQL（Center）/ SQLite（Edge，`modernc.org/sqlite`）

**Spec:** `docs/superpowers/specs/2026-10-08-sync-update-design.md`（本计划论证所依据的设计文档，执行者需同时阅读）

## Global Constraints

- **模块路径** `github.com/yourusername/igh-silkroad` 是刻意的，不要"修正"它。
- **编码**：所有文本文件 UTF-8 无 BOM；行尾 LF（`\n`）。
- **Ent 生成**：schema 变更后必须运行 `go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema`，并把生成产物一并提交。生成产物**禁止手工编辑**。
- **Ent 命名**：连续大写会被小写化。`plc_lot_number` → `SetPlcLotNumber`，**不是** `SetPLCLotNumber`。
- **枚举常量位置**：在**各自生成包**中（`lot.SyncStatusPending`、`bobbin.SyncStatusPending`、`doffing.SyncStatusPending`），**不在** ent 根包。`ent.BobbinSyncStatusPending` 不存在。
- **提交**：每个逻辑单元一次提交；提交信息含 `Refs: #17`；禁止 `--no-verify`；禁止对 `main` 强推。
- **不放松测试**：若修复导致既有测试失败，报告冲突，不得为通过而放宽断言。
- **Chinese 完整性**：写入含中文的文件后验证 `grep -c "[一-龥]"` 可读，且无 U+FFFD。
- **`-race` 在本环境不可用**（无 gcc/cgo）；并发正确性只能靠阅读与针对性测试验证，报告中须注明。

## Review Focus

以下是设计文档暗示、但初始任务测试容易漏掉、且最可能伤害现场操作者的输入类别。每条都在其归属任务中有对应测试：

1. **`sync_retry_count` 已达上限的记录被本地修改后**，应当重新获得完整重试预算并回到采集队列，而不是被 `markExhausted` 立刻打回 `failed`。（Task 2）
2. **同步器自身的写入**（`markSynced`/`markRetried`）不得触发脏标记，否则记录永远无法离开 `pending`，形成无限重传。（Task 2）
3. **Center 已存在同编号但不同 UUID 的行**时，上传应当认领该行并更新，而不是报唯一约束错误或新建重复行。（Task 5）
4. **载荷中的 `lot_id` 指向 Center 不存在的行**（Edge 本地 UUID）时，落纱/丝饼应按 `lot_number` 挂到 Center 已有的批次上，而不是写入错误外键或失败。（Task 6）
5. **同一丝饼质检两次**应当留下两条历史记录，而不是互相覆盖。（Task 7）

---

### Task 1: 修复 schema 生成阻塞并给 BobbinGrade 加同步字段

**背景（必读）：** `BobbinGrade` 与 `ProductConfig` 两个 schema 文件**从未被 ent 生成过**——它们的 `Annotations()` 返回类型写成了 `[]entsql.Annotation`，而 ent 要求 `[]schema.Annotation`，导致 `types.Implements` 判定为假，`entc/load/load.go:147` 直接 `continue` 静默跳过，**无任何报错**。`ent generate` 退出码 0 却零产出。

**Files:**
- Modify: `internal/database/ent/schema/bobbingrade.go:20,98,103,124`
- Modify: `internal/database/ent/schema/productconfig.go:20,92,97,115`
- Regenerate: `internal/database/ent/**`（生成产物）

**Interfaces:**
- Consumes: 无（本任务是最底层）
- Produces: 可用的 `ent.BobbinGrade` 类型与 `internal/database/ent/bobbingrade` 包；`BobbinGrade` 新增 `sync_status`/`synced_at`/`sync_retry_count` 三个字段

- [ ] **Step 1: 写失败的验证测试**

创建 `internal/database/ent/schema/bobbingrade_gen_test.go`：

```go
package schema_test

import (
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
)

// BobbinGrade 必须已被 ent 生成，且带同步字段。
//
// 背景：本实体此前因 Annotations() 签名不匹配被 ent 静默跳过，
// 生成包里根本没有它，故这里用生成包的存在性作为回归防线。
func TestBobbinGrade_IsGeneratedWithSyncFields(t *testing.T) {
	if got := bobbingrade.FieldSyncStatus; got != "sync_status" {
		t.Errorf("FieldSyncStatus = %q, want %q", got, "sync_status")
	}
	if got := bobbingrade.FieldSyncedAt; got != "synced_at" {
		t.Errorf("FieldSyncedAt = %q, want %q", got, "synced_at")
	}
	if got := bobbingrade.FieldSyncRetryCount; got != "sync_retry_count" {
		t.Errorf("FieldSyncRetryCount = %q, want %q", got, "sync_retry_count")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/database/ent/schema/ -run TestBobbinGrade_IsGenerated -v`
Expected: FAIL — 编译失败 `no required module provides package .../ent/bobbingrade`（该包不存在）

- [ ] **Step 3: 修正两个 schema 的 Annotations 签名**

`internal/database/ent/schema/bobbingrade.go`：把 import 中的 `"entgo.io/ent/dialect/entsql"` 换成 `"entgo.io/ent/schema"`（注意与同目录其他文件的 `ent` 导入并存时的别名冲突处理，必要时代码内用 `entschema "entgo.io/ent/schema"`），并把第 124 行改为：

```go
// Annotations of the BobbinGrade.
func (BobbinGrade) Annotations() []schema.Annotation {
	return nil
}
```

`internal/database/ent/schema/productconfig.go`：同样把 import 换掉，把第 115 行改为：

```go
// Annotations of the ProductConfig.
func (ProductConfig) Annotations() []schema.Annotation {
	return nil
}
```

**注意：** 这两个文件的 `Annotations()` 方法体都是 `return nil`，改签名不丢语义。若文件里还有别处用到 `entsql`（如 `SchemaType(map[string]string{"postgres": "jsonb"})` 是**字段级**的，不属 `entsql` 包），保留相应 import。

- [ ] **Step 4: 给 BobbinGrade 加同步字段与移除唯一索引**

在 `internal/database/ent/schema/bobbingrade.go` 的 `Fields()` 末尾（`updated_at` 之后）追加：

```go
		// 同步元数据（Edge → Center）
		field.Enum("sync_status").
			Values("pending", "synced", "failed").
			Default("pending").
			Comment("同步状态"),

		field.Time("synced_at").
			Optional().
			Comment("同步完成时间"),

		field.Int("sync_retry_count").
			Default(0).
			Comment("同步重试次数"),
```

同时把 `Indexes()` 中的 `bobbin_id` 唯一索引改为非唯一，并加同步状态索引：

```go
		// 丝锭ID索引（非唯一：同一丝饼可有多次质检，保留历史）
		index.Fields("bobbin_id"),
```

```go
		// 同步状态索引（Edge 端扫描待同步记录）
		index.Fields("sync_status"),
```

**为什么删唯一索引：** 该索引原本带注释「一个丝锭只有一条质检记录」，与「每次质检新增一条、保留历史」的设计决策冲突。若不删，第二次质检会撞唯一约束。

- [ ] **Step 5: 重新生成 ent 代码**

Run: `go run -mod=mod entgo.io/ent/cmd/ent generate ./internal/database/ent/schema`

验证生成确实发生（这是本任务的核心验收点）：

Run: `go run -mod=mod entgo.io/ent/cmd/ent describe ./internal/database/ent/schema`
Expected: 输出中同时包含 `BobbinGrade:` 与 `ProductConfig:`（此前只有 11 个 schema，现在应为 13 个）

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./internal/database/ent/schema/ -run TestBobbinGrade_IsGenerated -v`
Expected: PASS

Run: `go build ./... && go vet ./...`
Expected: 无输出

- [ ] **Step 7: 验证中文并提交**

```bash
grep -c "[一-龥]" internal/database/ent/schema/bobbingrade.go
git add internal/database/ent/
git commit -m "$(cat <<'EOF'
fix: 修复BobbinGrade与ProductConfig未被生成的问题并加同步字段

两文件的 Annotations() 返回 []entsql.Annotation 而 ent 要求
[]schema.Annotation，导致 types.Implements 为假、加载器静默跳过，
ent generate 退出码 0 却从未生成这两个实体。

同时为 BobbinGrade 加 sync_status/synced_at/sync_retry_count，
并把 bobbin_id 的唯一索引改为非唯一（质检需保留历史）。

Refs: #17
EOF
)"
```

---

### Task 2: Edge 侧脏标记 Hook

**Files:**
- Create: `internal/database/ent/hook/dirty.go`
- Create: `internal/database/ent/hook/dirty_test.go`
- Modify: `cmd/edge-server/main.go`（挂载 hook）

**Interfaces:**
- Consumes: Task 1 生成的实体类型
- Produces:
  - `hook.NewDirtyMarker() ent.Hook` — 返回可挂到 `client.Use(...)` 的 hook
  - 行为契约：对 `lots`/`bobbins`/`doffings` 的 Update/DeleteOne 变更，若改动字段**不全**是同步字段，则置 `sync_status = pending`、`sync_retry_count = 0`、清 `synced_at`

- [ ] **Step 1: 写失败的测试**

创建 `internal/database/ent/hook/dirty_test.go`：

```go
package hook_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/hook"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	_ "modernc.org/sqlite"
)

var dirtyTestCounter int64

func newDirtyClient(t *testing.T) *ent.Client {
	t.Helper()
	name := fmt.Sprintf("file:dirtytest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&dirtyTestCounter, 1))
	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("enable fk: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	client.Use(hook.NewDirtyMarker())
	return client
}

// 业务字段被改 → 记录重新变 pending
func TestDirtyMarker_BusinessWriteMarksPending(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-1").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusSynced).
		SetSyncRetryCount(3).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := row.Update().SetStatus(lot.StatusCompleted).Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q（业务字段被改应重新置脏）", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0（置脏应重置重试计数，否则会被 markExhausted 立刻打回 failed）", got.SyncRetryCount)
	}
}

// 只改同步字段 → 不得再次置脏（否则 markSynced 会被拦回 pending，无限重传）
func TestDirtyMarker_SyncOnlyWriteDoesNotReDirty(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-2").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 模拟 markSynced：只改同步字段
	if err := client.Lot.Update().
		Where(lot.IDEQ(row.ID)).
		SetSyncStatus(lot.SyncStatusSynced).
		SetSyncRetryCount(0).
		Exec(ctx); err != nil {
		t.Fatalf("sync-only update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusSynced {
		t.Errorf("SyncStatus = %q, want %q（只改同步字段不得被拦回 pending）", got.SyncStatus, lot.SyncStatusSynced)
	}
}

// failed 记录被编辑 → 复活为 pending 并重置计数
func TestDirtyMarker_RevivesFailedRecord(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-3").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusFailed).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := row.Update().SetStatus(lot.StatusPaused).Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q（编辑应复活 failed 记录）", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/database/ent/hook/ -v`
Expected: FAIL — 编译失败 `undefined: hook.NewDirtyMarker`

- [ ] **Step 3: 实现 dirty marker**

创建 `internal/database/ent/hook/dirty.go`：

```go
package hook

import (
	"context"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
)

// syncOnlyFields 是「同步元数据」字段集合。
//
// 用途：判断一次写入是否只动了同步状态本身。同步器（markSynced /
// markRetried）的写入只涉及这些字段，必须被排除，否则它把记录标成
// synced 之后会立刻被本 hook 拦回 pending，形成无限重传。
var syncOnlyFields = map[string]struct{}{
	"sync_status":       {},
	"synced_at":         {},
	"sync_retry_count":  {},
}

// NewDirtyMarker 返回一个把业务写入重新标记为待同步的 ent Hook。
//
// 挂在 Edge 侧的 ent.Client 上（client.Use(...)）。挂在 Center 侧没有意义：
// Center 不采集任何数据。
//
// 只处理采集器认得的表（lots / bobbins / doffings）。对采集器不认的表置
// pending 会造出永远排不出去的积压：retry_count 恒为 0，markExhausted
// 也碰不到它们。新增实体时需同步扩展本函数与 uploader 的 collectors()。
func NewDirtyMarker() ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			switch m.Op() {
			case ent.OpUpdate, ent.OpUpdateOne, ent.OpDelete, ent.OpDeleteOne:
				if shouldMarkDirty(m) {
					markDirty(m)
				}
			}
			return next.Mutate(ctx, m)
		})
	}
}

// shouldMarkDirty 判断本次写入是否应当置脏。
//
// 若改动的字段全部属于同步元数据，则不置脏（那是同步器自己的写入）。
func shouldMarkDirty(m ent.Mutation) bool {
	fields := m.Fields()
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if _, ok := syncOnlyFields[f]; !ok {
			return true
		}
	}
	return false
}

// markDirty 把记录重置为待同步并归还完整重试预算。
//
// 计数必须归零：采集器的过滤条件是 sync_status = pending 且
// sync_retry_count < 上限，而 markExhausted 会把「pending 且计数已达上限」
// 的行打回 failed。只置 pending 而不重置计数，会让一条计数已达上限的
// 记录在编辑后立刻被打回 failed —— 编辑再次丢失。
//
// 类型 switch 覆盖三个可采集实体。枚举常量在各自生成包中，
// 不在 ent 根包（ent.BobbinSyncStatusPending 不存在）。
func markDirty(m ent.Mutation) {
	switch mm := m.(type) {
	case *ent.LotMutation:
		mm.SetSyncStatus(lot.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	case *ent.BobbinMutation:
		mm.SetSyncStatus(bobbin.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	case *ent.DoffingMutation:
		mm.SetSyncStatus(doffing.SyncStatusPending)
		mm.SetSyncRetryCount(0)
		mm.ClearSyncedAt()
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/database/ent/hook/ -v`
Expected: 3 个测试全部 PASS

- [ ] **Step 5: 挂载到 Edge 侧 client**

修改 `cmd/edge-server/main.go`，在 `ent.NewClient(...)` 之后、`Schema.Create` 之前加入：

```go
	// 脏标记：业务字段被修改时自动把记录重置为待同步。
	// 必须挂在 Edge 侧 —— Center 不采集数据，置脏没有消费者。
	client.Use(enthook.NewDirtyMarker())
```

并补 import：

```go
	enthook "github.com/yourusername/igh-silkroad/internal/database/ent/hook"
```

- [ ] **Step 6: 验证变异能证伪**

手工实验（不是提交内容，验证后还原）：

1. 把 `shouldMarkDirty` 的 `if _, ok := syncOnlyFields[f]; !ok` 改为 `if false`（即"所有写入都置脏"）
2. Run: `go test ./internal/database/ent/hook/ -run TestDirtyMarker_SyncOnlyWriteDoesNotReDirty -v`
3. Expected: **FAIL** — 证明该测试确实在防无限重传
4. 还原

- [ ] **Step 7: 全量验证并提交**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`
Expected: 无输出

```bash
grep -c "[一-龥]" internal/database/ent/hook/dirty.go
git add internal/database/ent/hook/ cmd/edge-server/main.go
git commit -m "$(cat <<'EOF'
feat: 新增Edge侧脏标记Hook，业务写入自动重置同步状态

业务字段被修改时把 sync_status 置回 pending、sync_retry_count 归零、
清 synced_at，并复活 failed 记录。靠机制而非约定保证，避免新增写入点
漏改。只改同步字段的写入（如 markSynced）被排除，否则会无限重传。

Refs: #17
EOF
)"
```

---

### Task 3: Edge 上传载荷补齐字段

**Files:**
- Modify: `internal/sync/edge/uploader.go`（`BobbinQuery` 与 `lotToEntry`）
- Modify: `internal/sync/edge/uploader_test.go`

**Interfaces:**
- Consumes: 无
- Produces: 载荷中新增 `grade`、`tare_weight`（bobbins）、`lot_number`（bobbins）；`lots` 载荷保持不变

- [ ] **Step 1: 写失败的测试**

在 `internal/sync/edge/uploader_test.go` 末尾追加：

```go
// 丝饼载荷必须带上 grade、tare_weight，以及父行的 lot_number
// （Center 靠 lot_number 解引用父行，不采信载荷里的 lot_id）
func TestUploader_BobbinPayloadCarriesGradeTareAndLotNumber(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-PAYLOAD-1").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}

	if _, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-PAYLOAD-1").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		SetTareWeight(0.3).
		SetGrade("A").
		Save(ctx); err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending: %v", err)
	}

	var bobbinEntry *models.UploadEntry
	for i := range entries {
		if entries[i].Table == "bobbins" {
			bobbinEntry = &entries[i]
			break
		}
	}
	if bobbinEntry == nil {
		t.Fatal("未收集到丝饼条目")
	}

	if got := bobbinEntry.Data["grade"]; got != "A" {
		t.Errorf("grade = %v, want 有值（质检等级必须上传）", got)
	}
	if _, ok := bobbinEntry.Data["tare_weight"]; !ok {
		t.Error("载荷缺少 tare_weight")
	}
	if got := bobbinEntry.Data["lot_number"]; got != "LOT-PAYLOAD-1" {
		t.Errorf("lot_number = %v, want %q（Center 靠它解引用父行）", got, "LOT-PAYLOAD-1")
	}
}
```

`Goods` 处需确认 `models` 已导入（`github.com/yourusername/igh-silkroad/internal/sync/models`）；若未导入则补上。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/edge/ -run TestUploader_BobbinPayloadCarries -v`
Expected: FAIL — `grade = <nil>`、`载荷缺少 tare_weight`、`lot_number = <nil>`

- [ ] **Step 3: 补齐载荷字段**

修改 `internal/sync/edge/uploader.go` 的 `BobbinQuery`，在 `data` 中追加字段（放在 `"label_printed"` 之后）：

```go
			"tare_weight":       r.TareWeight,
			"grade":             r.Grade,
			"lot_number":        lotNumber, // 父行业务编号，供 Center 解引用
```

该函数需要先查出父行的 `lot_number`。在遍历 `rows` 之前收集批次 ID 并一次性查询：

```go
	// 收集本批涉及的 lot ID，一次性查出业务编号，
	// 供 Center 按编号解引用父行（载荷里的 lot_id 是 Edge 本地 UUID，
	// Center 认领后 ID 可能不同，故必须带上编号）。
	lotIDs := make([]uuid.UUID, 0, len(rows))
	seen := make(map[uuid.UUID]struct{}, len(rows))
	for _, r := range rows {
		if _, ok := seen[r.LotID]; !ok {
			seen[r.LotID] = struct{}{}
			lotIDs = append(lotIDs, r.LotID)
		}
	}
	lotNumbers := make(map[uuid.UUID]string, len(lotIDs))
	if len(lotIDs) > 0 {
		lots, err := u.client.Lot.Query().Where(lot.IDIn(lotIDs...)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("query parent lots: %w", err)
		}
		for _, l := range lots {
			lotNumbers[l.ID] = l.LotNumber
		}
	}
```

然后在循环内取用：

```go
		lotNumber := lotNumbers[r.LotID]
```

**注意：** 若父批次查不到（`lotNumber` 为空字符串），仍需照常上传——Center 会在解引用时拒绝并让该条计重试，这是预期行为，不要在这里静默跳过。

`lotToEntry` **不需要改**：`lots` 载荷已含 `lot_number`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/sync/edge/ -v`
Expected: 全部 PASS（含原有测试）

- [ ] **Step 5: 验证变异能证伪**

手工实验（验证后还原）：删除 `"lot_number": lotNumber,` 一行

Run: `go test ./internal/sync/edge/ -run TestUploader_BobbinPayloadCarries -v`
Expected: **FAIL**

- [ ] **Step 6: 提交**

```bash
grep -c "[一-龥]" internal/sync/edge/uploader.go
git add internal/sync/edge/
git commit -m "$(cat <<'EOF'
feat: 丝饼上传载荷补齐grade、tare_weight与lot_number

等级与皮重此前根本不在载荷里，故无条件丢失。lot_number 是 Center
按业务编号解引用父行的依据（载荷里的 lot_id 是 Edge 本地 UUID）。

Refs: #17
EOF
)"
```

---

### Task 4: Center 侧 upsert 与唯一约束认领

**Files:**
- Modify: `internal/sync/center/handler.go`（`createLot`、`createBobbin`）
- Modify: `internal/sync/center/sync_test.go`

**Interfaces:**
- Consumes: Task 3 的载荷（新增字段）
- Produces:
  - `func (h *UploadHandler) upsertLot(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error`
  - `func (h *UploadHandler) upsertBobbin(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error`
  - 行为契约：先按 ID 查；未命中再按业务编号查；命中则更新该行（保留其 ID）；都未命中则插入；唯一约束冲突时重试一次按编号认领

- [ ] **Step 1: 写失败的测试**

在 `internal/sync/center/sync_test.go` 末尾追加：

```go
// 重传同一批两次：Center 不得出现重复行，且字段应被更新
func TestUpsertLot_RepublishUpdatesInPlace(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	handler := center.NewUploadHandler(client)
	lotID := uuid.New()

	mk := func(status string) *models.UploadRequest {
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "lots", Operation: "create", ID: lotID,
				Data: map[string]interface{}{
					"id": lotID.String(), "lot_number": "LOT-UPSERT-1",
					"product_type": "FDY", "planned_quantity": 10,
					"actual_quantity": 0, "status": status,
				},
			}},
		}
	}

	if _, err := handler.HandleUpload(ctx, mk("in_progress")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, mk("completed")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	n, err := client.Lot.Query().Where(lot.LotNumberEQ("LOT-UPSERT-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（重传应更新而非新建）", n)
	}

	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Status) != "completed" {
		t.Errorf("Status = %q, want %q（重传应更新字段）", got.Status, "completed")
	}
}

// Center 已存在同编号但不同 UUID 的行 → 应认领该行，不新建、不报错
func TestUpsertLot_ClaimsExistingByLotNumber(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	// Center 侧先存在一行（模拟人工/历史创建），ID 与 Edge 本地不同
	existing, err := client.Lot.Create().
		SetLotNumber("LOT-CLAIM-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	edgeLocalID := uuid.New() // Edge 本地是另一个 UUID
	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "lots", Operation: "create", ID: edgeLocalID,
			Data: map[string]interface{}{
				"id": edgeLocalID.String(), "lot_number": "LOT-CLAIM-1",
				"product_type": "FDY", "planned_quantity": 20,
				"actual_quantity": 0, "status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	n, err := client.Lot.Query().Where(lot.LotNumberEQ("LOT-CLAIM-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（应认领既有行，不得新建重复）", n)
	}

	got, err := client.Lot.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if got.PlannedQuantity != 20 {
		t.Errorf("PlannedQuantity = %d, want 20（认领后应更新该行）", got.PlannedQuantity)
	}
}
```

并在文件末尾追加测试辅助（若已存在同名函数则复用，不要重复定义）：

```go
// edgeRowID 创建（或复用）一条 Edge 行并返回其 ID。
// 上传路径要求 req.EdgeID 是设备行的 UUID。
func edgeRowID(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if row, err := client.Edge.Query().First(ctx); err == nil {
		return row.ID
	}
	row, err := client.Edge.Create().
		SetEdgeCode("edge-upsert-test").
		SetEdgeName("测试设备").
		SetIPAddress("192.168.2.84").
		Save(ctx)
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}
	return row.ID
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/center/ -run "TestUpsertLot" -v`
Expected: FAIL — `TestUpsertLot_RepublishUpdatesInPlace` 报 `行数 = 1` 通过但 `Status` 未更新（幂等分支 `return nil` 直接跳过）；`TestUpsertLot_ClaimsExistingByLotNumber` 报唯一约束错误导致 `applied = 0`

- [ ] **Step 3: 实现认领查询与 upsert**

修改 `internal/sync/center/handler.go`。首先把 `createLot` 重命名为 `upsertLot` 并重写主体：

```go
// upsertLot 插入或更新批次记录。
//
// 幂等与认领：先按 ID 查；未命中再按 lot_number 查（该字段唯一）。
// 编号命中说明 Center 侧已存在同一业务批次但 UUID 不同（历史数据或人工
// 创建），此时认领该行并更新字段，**保留 Center 行的 ID** —— 这样 Center
// 侧既有的 lot_id 引用（落纱/丝饼）继续有效。
//
// 为何不能只按 ID 判断：Edge 与 Center 各自用 uuid.New 生成主键，
// 同一业务批次在两端的 ID 不同，只按 ID 查必然放行，随后撞唯一约束。
//
// edgeID 为鉴权层推导的权威设备身份；载荷中的 edge_id 一律不采信。
func (h *UploadHandler) upsertLot(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotNumber := getString(data, "lot_number")
	if lotNumber == "" {
		return fmt.Errorf("missing lot_number for entry %s（按编号认领需要它）", id)
	}

	target, err := h.resolveLot(ctx, id, lotNumber)
	if err != nil {
		return err
	}

	// 未命中任何既有行：按 Edge 的 ID 新建
	if target == nil {
		created, err := h.client.Lot.Create().
			SetID(id).
			SetLotNumber(lotNumber).
			SetProductType(lot.ProductType(getString(data, "product_type"))).
			SetPlannedQuantity(getInt(data, "planned_quantity")).
			SetActualQuantity(getInt(data, "actual_quantity")).
			SetStatus(lot.Status(getString(data, "status"))).
			SetNillableStartTime(getTimePtr(data, "start_time")).
			SetNillableEndTime(getTimePtr(data, "end_time")).
			SetNillablePlcLotNumber(getStringPtr(data, "plc_lot_number")).
			SetNillableOrderCode(getStringPtr(data, "order_code")).
			SetNillableProductSpec(getStringPtr(data, "product_spec")).
			Save(ctx)
		if err != nil {
			// 并发或历史数据导致唯一约束冲突：重查一次并转更新
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveLot(ctx, id, lotNumber); rerr == nil && again != nil {
					return h.updateLotRow(ctx, again, data, edgeID)
				}
			}
			return fmt.Errorf("create lot: %w", err)
		}
		log.Printf("✅ Created lot: %s", created.LotNumber)
		return nil
	}

	return h.updateLotRow(ctx, target, data, edgeID)
}

// resolveLot 按 ID 再按业务编号定位既有行；都不存在返回 (nil, nil)。
func (h *UploadHandler) resolveLot(ctx context.Context, id uuid.UUID, lotNumber string) (*ent.Lot, error) {
	byID, err := h.client.Lot.Query().Where(lot.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Lot.Query().Where(lot.LotNumberEQ(lotNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// updateLotRow 以 Edge 侧数据更新既有行。整行以 Edge 为准（单向归属）。
func (h *UploadHandler) updateLotRow(ctx context.Context, row *ent.Lot, data map[string]interface{}, edgeID uuid.UUID) error {
	if err := row.Update().
		SetProductType(lot.ProductType(getString(data, "product_type"))).
		SetPlannedQuantity(getInt(data, "planned_quantity")).
		SetActualQuantity(getInt(data, "actual_quantity")).
		SetStatus(lot.Status(getString(data, "status"))).
		SetNillableStartTime(getTimePtr(data, "start_time")).
		SetNillableEndTime(getTimePtr(data, "end_time")).
		SetPlcLotNumber(getString(data, "plc_lot_number")).
		SetOrderCode(getString(data, "order_code")).
		SetProductSpec(getString(data, "product_spec")).
		Exec(ctx); err != nil {
		return fmt.Errorf("update lot: %w", err)
	}
	return nil
}
```

**关于可选字段的重复：** `LotCreate` 与 `LotUpdateOne` 是两个不同的 builder 类型，无法共用一个设置函数。

**做法：两条路径各自内联，不要抽接口。** 字段只有三个（`plc_lot_number`/`order_code`/`product_spec`），重复成本低于抽象成本。注意两侧的 setter 名不同：

- Create 侧用 **Nillable** 版本（可传 nil）：`SetNillablePlcLotNumber`、`SetNillableOrderCode`、`SetNillableProductSpec`
- Update 侧用非 Nillable 版本（空串即清空）：`SetPlcLotNumber`、`SetOrderCode`、`SetProductSpec`

两处都加注释指向对方，说明"改名需同步另一处"。

`upsertBobbin` 同理：按 `id` 再按 `bobbin_number` 定位（与 `upsertLot` 结构相同，此处不再重复代码，执行者按同样模式改写，注意 `bobbin` 需 `SetLotID(lotID)` 与 `edgeID` 无关——`bobbins` 表无 `edge_id` 列）。

在 `handleCreate` 中把两处调用改为 upsert 版本：

```go
func (h *UploadHandler) handleCreate(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	switch entry.Table {
	case "lots":
		return h.upsertLot(ctx, entry, edgeID)
	case "bobbins":
		return h.upsertBobbin(ctx, entry, edgeID)
	default:
		return fmt.Errorf("unknown table: %s", entry.Table)
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/sync/center/ -v`
Expected: 全部 PASS（含原有测试）

- [ ] **Step 5: 验证变异能证伪**

手工实验（验证后还原）：

1. 把 `resolveLot` 里按编号查询的那段删掉（只按 ID 查）
2. Run: `go test ./internal/sync/center/ -run TestUpsertLot_ClaimsExistingByLotNumber -v`
3. Expected: **FAIL** — 撞唯一约束，`applied = 0`
4. 还原

- [ ] **Step 6: 提交**

```bash
grep -c "[一-龥]" internal/sync/center/handler.go
git add internal/sync/center/
git commit -m "$(cat <<'EOF'
feat: Center同步路径改为upsert并支持按业务编号认领

原实现只按主键做幂等检查，而 Edge 与 Center 各自生成 UUID，
同一业务批次在两端 ID 不同，故必然放行并撞唯一约束，
且约束错误被当作普通错误计入重试，5 次后永久 failed。

改为按 ID 再按业务编号定位，命中即认领并更新该行（保留其 ID），
并识别 ent.IsConstraintError 后重查转更新。

Refs: #17
EOF
)"
```

---

### Task 5: 按业务编号解引用父行

**Files:**
- Modify: `internal/sync/center/handler.go`（`upsertBobbin` 及后续 `createDoffing` 的父行解析）
- Modify: `internal/sync/center/sync_test.go`

**Interfaces:**
- Consumes: Task 4 的 `resolveLot`、Task 3 的载荷 `lot_number`
- Produces: `func (h *UploadHandler) resolveParentLotID(ctx context.Context, data map[string]interface{}) (uuid.UUID, error)` — 按载荷的 `lot_number` 查 Center 的批次行并返回其 ID

- [ ] **Step 1: 写失败的测试**

在 `internal/sync/center/sync_test.go` 末尾追加：

```go
// 载荷里的 lot_id 是 Edge 本地 UUID，Center 必须按 lot_number 解引用，
// 把子记录挂到 Center 自己的批次行上。
func TestUploadBobbin_DereferencesParentByLotNumber(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	// Center 侧已有的批次（UUID 与 Edge 本地不同）
	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-DEREF-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	edgeLocalLotID := uuid.New() // Edge 本地的批次 UUID，Center 不认识
	bobbinID := uuid.New()

	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "bobbins", Operation: "create", ID: bobbinID,
			Data: map[string]interface{}{
				"id":                bobbinID.String(),
				"bobbin_number":     "BOBBIN-DEREF-1",
				"lot_id":            edgeLocalLotID.String(), // 陷阱：会被忽略
				"lot_number":        "LOT-DEREF-1",           // 真依据
				"spinning_position": 1,
				"gross_weight":      1.5,
				"net_weight":        1.2,
				"status":            "producing",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		t.Fatalf("get bobbin: %v", err)
	}
	if got.LotID != centerLot.ID {
		t.Errorf("LotID = %v, want %v（必须按编号解引用到 Center 的批次，而非采信载荷的 lot_id）",
			got.LotID, centerLot.ID)
	}
	if got.LotID == edgeLocalLotID {
		t.Error("采信了载荷中的 lot_id —— 这是安全缺陷")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/center/ -run TestUploadBobbin_DereferencesParent -v`
Expected: FAIL — 报外键约束失败（`edgeLocalLotID` 在 Center 不存在），`applied = 0`

- [ ] **Step 3: 实现解引用**

在 `internal/sync/center/handler.go` 中新增：

```go
// resolveParentLotID 按载荷的 lot_number 查 Center 侧的批次行，返回其 ID。
//
// 为何不采信载荷里的 lot_id：那是 Edge 本地生成的 UUID。Center 可能已
// 存在同一业务编号、但主键不同的行（历史数据或人工创建），此时若直接
// 用载荷的 ID 写外键，要么撞外键约束失败，要么挂到错误的行上。
// 与整个同步链路的原则一致：身份由 Center 推导，载荷只提供业务键。
func (h *UploadHandler) resolveParentLotID(ctx context.Context, data map[string]interface{}) (uuid.UUID, error) {
	lotNumber := getString(data, "lot_number")
	if lotNumber == "" {
		return uuid.Nil, fmt.Errorf("missing lot_number：无法定位父批次")
	}
	parent, err := h.client.Lot.Query().Where(lot.LotNumberEQ(lotNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// 父行尚未到达：让本条目计重试，等批次上传后再挂载。
			// 不写 NULL 外键（lot_id 是 Required），也不静默跳过。
			return uuid.Nil, fmt.Errorf("parent lot %q not found: %w", lotNumber, err)
		}
		return uuid.Nil, err
	}
	return parent.ID, nil
}
```

把 **两处** `lotID` 的来源都从载荷改为解引用。

**注意：Task 4 新增了 `updateBobbinRow`，它同样解析载荷的 `lot_id`。只改 `upsertBobbin` 会让插入路径解引用、认领路径仍采信载荷 —— 本任务新增的测试会在第二遍就失败。** 两处都要改：

```go
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}
```

原先是 `uuid.Parse(getString(data, "lot_id"))`，该行在两处都删除。用 grep 确认改完后非测试代码中不再有该写法：

```bash
grep -n 'uuid.Parse(getString(data, "lot_id"))' internal/sync/center/handler.go
```

预期：无输出。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/sync/center/ -v`
Expected: 全部 PASS

- [ ] **Step 5: 验证变异能证伪**

手工实验（验证后还原）：把 `resolveParentLotID` 改回 `uuid.Parse(getString(data, "lot_id"))`（两处改回任一处都应让测试变红——若只有一处变红说明另一处仍未接入）

Run: `go test ./internal/sync/center/ -run TestUploadBobbin_DereferencesParent -v`
Expected: **FAIL**（外键约束）

- [ ] **Step 6: 提交**

```bash
grep -c "[一-龥]" internal/sync/center/handler.go
git add internal/sync/center/
git commit -m "$(cat <<'EOF'
feat: Center按lot_number解引用父批次，不采信载荷中的lot_id

载荷里的 lot_id 是 Edge 本地 UUID，与 Center 行主键可能不同。
直接使用会撞外键或挂错行；改为按业务编号查询父行并以 Center 的 ID 落库。

Refs: #17
EOF
)"
```

---

### Task 6: 打通 doffings 同步

**Files:**
- Modify: `internal/sync/center/handler.go`（新增 `upsertDoffing` 与 `handleCreate` 分支）
- Modify: `internal/sync/edge/uploader.go`（`collectors()` 加入 doffings）
- Modify: `internal/sync/center/sync_test.go`、`internal/sync/edge/uploader_test.go`

**Interfaces:**
- Consumes: Task 5 的 `resolveParentLotID`
- Produces:
  - `func (h *UploadHandler) upsertDoffing(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error`
  - `collectors()` 新增 `{table: "doffings", collect: u.collectDoffings}`

- [ ] **Step 1: 写失败的测试**

在 `internal/sync/center/sync_test.go` 末尾追加：

```go
// 落纱上传：按 lot_number 挂到 Center 的批次，且状态与重量落库
func TestUpsertDoffing_PersistsWithDereferencedLot(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-DOFF-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	lineID := uuid.New()
	doffingID := uuid.New()
	handler := center.NewUploadHandler(client)

	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "doffings", Operation: "create", ID: doffingID,
			Data: map[string]interface{}{
				"id":                doffingID.String(),
				"lot_id":            uuid.New().String(), // 陷阱：忽略
				"lot_number":        "LOT-DOFF-1",
				"spinning_line_id":  lineID.String(),
				"spinning_position": 3,
				"bobbin_number":     "BOBBIN-DOFF-1",
				"actual_weight":     1.75,
				"grade":             "B",
				"status":            "confirmed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Doffing.Get(ctx, doffingID)
	if err != nil {
		t.Fatalf("get doffing: %v", err)
	}
	if got.LotID != centerLot.ID {
		t.Errorf("LotID = %v, want %v", got.LotID, centerLot.ID)
	}
	if string(got.Status) != "confirmed" {
		t.Errorf("Status = %q, want %q", got.Status, "confirmed")
	}
	if got.ActualWeight != 1.75 {
		t.Errorf("ActualWeight = %v, want 1.75", got.ActualWeight)
	}
	if got.Grade != "B" {
		t.Errorf("Grade = %q, want %q", got.Grade, "B")
	}
}

// 落纱状态流转：同一落纱重传两次，第二次应更新状态而非新建
func TestUpsertDoffing_SecondUploadUpdatesStatus(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-DOFF-2").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx); err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	doffingID := uuid.New()
	handler := center.NewUploadHandler(client)
	mk := func(status string) *models.UploadRequest {
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "doffings", Operation: "create", ID: doffingID,
				Data: map[string]interface{}{
					"id": doffingID.String(), "lot_number": "LOT-DOFF-2",
					"spinning_line_id": uuid.New().String(), "spinning_position": 1,
					"status": status,
				},
			}},
		}
	}

	if _, err := handler.HandleUpload(ctx, mk("pending")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, mk("cancelled")); err != nil {
		t.Fatalf("second: %v", err)
	}

	n, err := client.Doffing.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("落纱行数 = %d, want 1（状态流转应更新同一行）", n)
	}
	got, err := client.Doffing.Get(ctx, doffingID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Status) != "cancelled" {
		t.Errorf("Status = %q, want %q", got.Status, "cancelled")
	}
}
```

在 `internal/sync/edge/uploader_test.go` 末尾追加：

```go
// collectors 必须包含 doffings（Center 已实现接收）
func TestUploader_CollectsDoffings(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-COLLECT-DOFF").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	if _, err := client.Doffing.Create().
		SetSpinningLineID(uuid.New()).
		SetSpinningPosition(1).
		SetLotID(lotRow.ID).
		SetStatus(doffing.StatusPending).
		Save(ctx); err != nil {
		t.Fatalf("create doffing: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Table == "doffings" {
			found = true
			if e.Data["lot_number"] == nil {
				t.Error("落纱载荷缺少 lot_number（Center 靠它解引用父批次）")
			}
		}
	}
	if !found {
		t.Error("未收集到 doffings —— Center 已实现接收，应当被采集")
	}
}
```

（`doffing` 包需导入：`github.com/yourusername/igh-silkroad/internal/database/ent/doffing`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/center/ -run TestUpsertDoffing -v`
Expected: FAIL — `unknown table: doffings`

Run: `go test ./internal/sync/edge/ -run TestUploader_CollectsDoffings -v`
Expected: FAIL — `未收集到 doffings`

- [ ] **Step 3: 实现 Center 侧落纱接收**

在 `internal/sync/center/handler.go` 的 `handleCreate` 中增加分支：

```go
	case "doffings":
		return h.upsertDoffing(ctx, entry, edgeID)
```

新增函数（注意 `doffing` 载荷的 `spinning_line_id` **不是外键**，直接按 UUID 落库即可）：

```go
// upsertDoffing 插入或更新落纱记录。
//
// 父批次按 lot_number 解引用（见 resolveParentLotID）。
// spinning_line_id 在 schema 中不是外键（Doffing.Edges() 只声明 lot），
// 故直接落库，不需要查父行。
//
// 状态流转（pending → confirmed → cancelled）走同一行的更新，
// 故每次状态变更重传都应命中原行。
func (h *UploadHandler) upsertDoffing(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}
	lineID, err := uuid.Parse(getString(data, "spinning_line_id"))
	if err != nil {
		return fmt.Errorf("invalid spinning_line_id %q: %w", getString(data, "spinning_line_id"), err)
	}

	existing, err := h.client.Doffing.Query().Where(doffing.IDEQ(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	if existing == nil {
		_, err = h.client.Doffing.Create().
			SetID(id).
			SetLotID(lotID).
			SetSpinningLineID(lineID).
			SetSpinningPosition(getInt(data, "spinning_position")).
			SetStatus(doffing.Status(getString(data, "status"))).
			SetNillableBobbinNumber(getStringPtr(data, "bobbin_number")).
			SetNillableActualWeight(getFloatPtr(data, "actual_weight")).
			SetNillableGrade(getStringPtr(data, "grade")).
			SetNillableCancelReason(getStringPtr(data, "cancel_reason")).
			SetNillableDoffingTime(getTimePtr(data, "doffing_time")).
			SetNillableConfirmedAt(getTimePtr(data, "confirmed_at")).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create doffing: %w", err)
		}
		log.Printf("✅ Created doffing: %s", id)
		return nil
	}

	if err := existing.Update().
		SetLotID(lotID).
		SetSpinningLineID(lineID).
		SetSpinningPosition(getInt(data, "spinning_position")).
		SetStatus(doffing.Status(getString(data, "status"))).
		SetNillableBobbinNumber(getStringPtr(data, "bobbin_number")).
		SetNillableActualWeight(getFloatPtr(data, "actual_weight")).
		SetNillableGrade(getStringPtr(data, "grade")).
		SetNillableCancelReason(getStringPtr(data, "cancel_reason")).
		SetNillableConfirmedAt(getTimePtr(data, "confirmed_at")).
		Exec(ctx); err != nil {
		return fmt.Errorf("update doffing: %w", err)
	}
	return nil
}
```

补 import：`"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"`。

- [ ] **Step 4: 实现 Edge 侧采集**

修改 `internal/sync/edge/uploader.go` 的 `collectors()`：

```go
func (u *Uploader) collectors() []uploadableEntity {
	return []uploadableEntity{
		{table: "lots", collect: u.collectLots},
		{table: "doffings", collect: u.collectDoffings}, // Center 已实现接收
		{table: "bobbins", collect: u.BobbinQuery},
	}
}
```

同时更新 `collectors()` 上方注释中「重新加入的条件」段落——`doffings` 的条件已满足，从待加入列表移除；`barrels` 仍保持待加入（Edge 侧无建桶路径）。

并给 `collectDoffings` 的载荷补上 `lot_number`（与 Task 3 的丝饼同理）。该函数当前直接构造 `models.UploadEntry`，需先查父批次编号：

```go
	// 收集本批涉及的批次 ID，批量查出业务编号
	doffingLotIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		doffingLotIDs = append(doffingLotIDs, r.LotID)
	}
	lotNumbers, err := u.lotNumbersFor(ctx, doffingLotIDs)
	if err != nil {
		return nil, err
	}
```

在 `Data` 中增加：

```go
				"lot_number":        lotNumbers[r.LotID],
```

把 Task 3 中内联的"收集父批次编号"逻辑抽成共用方法：

```go
// lotNumbersFor 批量查出给定记录的父批次业务编号。
// Center 靠 lot_number 解引用父行（载荷里的 lot_id 是 Edge 本地 UUID）。
// 查不到的批次返回空串，由 Center 拒绝该条并计重试 —— 不在此静默跳过。
func (u *Uploader) lotNumbersFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	uniq := make(map[uuid.UUID]struct{}, len(ids))
	list := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := uniq[id]; ok {
			continue
		}
		uniq[id] = struct{}{}
		list = append(list, id)
	}
	out := make(map[uuid.UUID]string, len(list))
	if len(list) == 0 {
		return out, nil
	}
	lots, err := u.client.Lot.Query().Where(lot.IDIn(list...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query parent lots: %w", err)
	}
	for _, l := range lots {
		out[l.ID] = l.LotNumber
	}
	return out, nil
}
```

并把 Task 3 在 `BobbinQuery` 内联的那段替换为 `u.lotNumbersFor(ctx, lotIDs)`。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/sync/center/ ./internal/sync/edge/ -v`
Expected: 全部 PASS

- [ ] **Step 6: 验证变异能证伪**

手工实验（验证后还原）：从 `collectors()` 中删掉 doffings 那一行

Run: `go test ./internal/sync/edge/ -run TestUploader_CollectsDoffings -v`
Expected: **FAIL**

- [ ] **Step 7: 提交**

```bash
grep -c "[一-龥]" internal/sync/center/handler.go internal/sync/edge/uploader.go
git add internal/sync/
git commit -m "$(cat <<'EOF'
feat: 打通落纱同步（Center接收 + Edge采集）

落纱此前只采集不上传的中间态：Center 无 doffings 分支，采集器也未启用。
现补 upsertDoffing 并按 lot_number 解引用父批次；collectors 加入 doffings，
每次状态流转（pending→confirmed/cancelled）走同一行的更新。

Refs: #17
EOF
)"
```

---

### Task 7: BobbinGrade 启用（质检历史）

**Files:**
- Modify: `internal/service/bobbin.go`（`InspectBobbin` 写入 BobbinGrade）
- Modify: `internal/sync/center/handler.go`（新增 `upsertBobbinGrade`）
- Modify: `internal/sync/edge/uploader.go`（采集 BobbinGrade）
- Modify: 各对应测试文件

**Interfaces:**
- Consumes: Task 1 生成的 `ent.BobbinGrade` 与 `bobbingrade` 包
- Produces:
  - `InspectBobbin` 额外写入一条 `BobbinGrade`
  - `func (h *UploadHandler) upsertBobbinGrade(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error`
  - `collectors()` 新增 `{table: "bobbin_grades", collect: u.collectBobbinGrades}`

- [ ] **Step 1: 写失败的测试**

在 `internal/service/bobbin_test.go` 末尾追加：

```go
// 质检必须留下历史记录：同一丝饼质检两次 → 两条 BobbinGrade
func TestInspectBobbin_AppendsGradeHistory(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-INSPECT-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-INSPECT-1").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{Grade: "A"}); err != nil {
		t.Fatalf("first inspect: %v", err)
	}
	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{Grade: "B"}); err != nil {
		t.Fatalf("second inspect: %v（重检不得因唯一约束失败）", err)
	}

	n, err := client.BobbinGrade.Query().
		Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).
		Count(ctx)
	if err != nil {
		t.Fatalf("count grades: %v", err)
	}
	if n != 2 {
		t.Errorf("质检记录数 = %d, want 2（应保留历史，不覆盖）", n)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestInspectBobbin_AppendsGradeHistory -v`
Expected: FAIL — `质检记录数 = 0`

- [ ] **Step 3: 实现质检写入**

修改 `internal/service/bobbin.go` 的 `InspectBobbin`，在更新 `bobbin.grade` 之后追加写入质检历史：

```go
	// 保留完整质检历史（每次质检新增一条），同时把最新等级同步到 bobbin.grade。
	// 不覆盖历史：缺陷原因、评分等在现场只产生一次，丢失即不可恢复。
	if _, err := s.client.BobbinGrade.Create().
		SetBobbinID(bobbinID).
		SetGrade(req.Grade).
		SetNillableInspectorID(inspectorID). // 空串时传 nil
		SetNillableDefectReason(defectReason). // 空串时传 nil
		Save(ctx); err != nil {
		return err
	}
```

其中 `inspectorID` 与 `defectReason` 需从 `req` 解析（`InspectBobbinRequest` 已有 `InspectorID string` 与 `DefectNote string`）：

```go
	var inspectorID *string
	if req.InspectorID != "" {
		inspectorID = &req.InspectorID
	}
	var defectReason *string
	if req.DefectNote != "" {
		defectReason = &req.DefectNote
	}
```

**注意：** `SetNillableInspectorID` 接受 `*uuid.UUID`（schema 中该字段为 UUID），故 `InspectorID` 需先 `uuid.Parse`；解析失败按无效参数处理（返回 `ServiceError`），不要静默丢弃。

同时把 `InspectBobbin` 里那条 TODO 注释（`// TODO: 需要在schema中添加defect_note字段`）删掉——`BobbinGrade.defect_reason` 已经是落点。

- [ ] **Step 4: 实现 Edge 采集与 Center 接收**

Edge 侧在 `internal/sync/edge/uploader.go` 新增：

```go
// collectBobbinGrades 查询待同步质检记录。
//
// 质检记录只增不改，故上传语义天然是 create，无需 upsert 与字段认领。
func (u *Uploader) collectBobbinGrades(ctx context.Context, limit int) ([]models.UploadEntry, error) {
	rows, err := u.client.BobbinGrade.Query().
		Where(
			bobbingrade.SyncStatusEQ(bobbingrade.SyncStatusPending),
			bobbingrade.SyncRetryCountLT(maxRetryCount),
		).
		Order(ent.Asc(bobbingrade.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query bobbin_grades: %w", err)
	}

	out := make([]models.UploadEntry, 0, len(rows))
	for _, r := range rows {
		data := map[string]interface{}{
			"id":         r.ID.String(),
			"bobbin_id":  r.BobbinID.String(),
			"grade":      r.Grade,
			"is_qualified": r.IsQualified,
			"inspected_at": r.InspectedAt,
		}
		if r.InspectorID != nil {
			data["inspector_id"] = *r.InspectorID
		}
		if r.DefectReason != "" {
			data["defect_reason"] = r.DefectReason
		}
		out = append(out, models.UploadEntry{
			Table:     "bobbin_grades",
			Operation: "create",
			ID:        r.ID,
			Data:      data,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}
```

在 `collectors()` 中加入（**放在 bobbins 之后**——`bobbin_grades.bobbin_number` 不需要解引用，但保持"父先子后"的阅读一致性）：

```go
		{table: "bobbin_grades", collect: u.collectBobbinGrades},
```

并在 `markableTables` 中加入 `"bobbin_grades"`（否则 `markSynced`/`markRetried`/`markExhausted` 会跳过它——`forEachTable` 按该白名单遍历）。

Center 侧在 `handleCreate` 增加分支与函数：

```go
	case "bobbin_grades":
		return h.upsertBobbinGrade(ctx, entry, edgeID)
```

```go
// upsertBobbinGrade 插入质检记录（只增不改）。
//
// Center 侧不采信载荷的 bobbin_id 直接落库，而是按 bobbin_number 找到
// 本地丝饼行 —— 与 lot 同理，两端 UUID 不同。
func (h *UploadHandler) upsertBobbinGrade(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}

	exists, err := h.client.BobbinGrade.Query().Where(bobbingrade.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		// 只增语义：同 ID 重传视为已处理，不更新（质检结论不应被覆盖）
		return nil
	}

	bobbinNumber := getString(data, "bobbin_number")
	if bobbinNumber == "" {
		return fmt.Errorf("missing bobbin_number：无法定位质检对象")
	}
	parent, err := h.client.Bobbin.Query().Where(bobbin.BobbinNumberEQ(bobbinNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fmt.Errorf("parent bobbin %q not found: %w", bobbinNumber, err)
		}
		return err
	}

	_, err = h.client.BobbinGrade.Create().
		SetID(id).
		SetBobbinID(parent.ID).
		SetGrade(getString(data, "grade")).
		SetIsQualified(getBool(data, "is_qualified")).
		SetNillableInspectedAt(getTimePtr(data, "inspected_at")).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create bobbin grade: %w", err)
	}
	return nil
}
```

**注意：** `inspected_at` 在 schema 中是 `field.Time("inspected_at").Default(time.Now)`——**非 Optional**，故用 `SetInspectedAt`（不接受 nil），不是 `SetNillableInspectedAt`。需要一个取默认值的辅助：

```go
// getTimeOrNow 读取时间字段；缺失时返回当前时间。
// 供非 Optional 的时间列使用（如 BobbinGrade.inspected_at）。
func getTimeOrNow(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case time.Time:
			return t
		case string:
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				return parsed
			}
		}
	}
	return time.Now()
}
```

`collectBobbinGrades` 的载荷必须带 `bobbin_number`——`BobbinGrade` schema 中**没有**该字段，它是 Edge 侧为让 Center 定位父行而附加的，不在本地表里。

在 `internal/sync/edge/uploader.go` 中按 Task 6 同样的模式新增：

```go
// bobbinNumbersFor 批量查出给定丝饼的业务编号。
// BobbinGrade 本地表没有 bobbin_number 字段，Center 需要它来定位父丝饼行
// （两端 UUID 不同，与 lot 同理）。
func (u *Uploader) bobbinNumbersFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	uniq := make(map[uuid.UUID]struct{}, len(ids))
	list := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := uniq[id]; ok {
			continue
		}
		uniq[id] = struct{}{}
		list = append(list, id)
	}
	out := make(map[uuid.UUID]string, len(list))
	if len(list) == 0 {
		return out, nil
	}
	rows, err := u.client.Bobbin.Query().Where(bobbin.IDIn(list...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query parent bobbins: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = r.BobbinNumber
	}
	return out, nil
}
```

在 `collectBobbinGrades` 中调用它补进载荷：

```go
	parentIDs := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		parentIDs = append(parentIDs, r.BobbinID)
	}
	bobbinNumbers, err := u.bobbinNumbersFor(ctx, parentIDs)
	if err != nil {
		return nil, err
	}
```

然后在构造 `data` 时加入：

```go
			"bobbin_number": bobbinNumbers[r.BobbinID],
```

同时把 Task 7 的 Center 侧 `upsertBobbinGrade` 中依赖 `bobbin.BobbinNumberEQ` 的部分与上面保持一致——`bobbin_number` 是 payload 字段，不是本地列。

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/service/ ./internal/sync/center/ ./internal/sync/edge/ -v`
Expected: 全部 PASS

- [ ] **Step 6: 验证变异能证伪**

手工实验（验证后还原）：在 `InspectBobbin` 中删掉写 `BobbinGrade` 的那段

Run: `go test ./internal/service/ -run TestInspectBobbin_AppendsGradeHistory -v`
Expected: **FAIL**（记录数 = 0）

- [ ] **Step 7: 提交**

```bash
grep -c "[一-龥]" internal/service/bobbin.go internal/sync/center/handler.go internal/sync/edge/uploader.go
git add internal/
git commit -m "$(cat <<'EOF'
feat: 启用BobbinGrade，质检明细随同步上传并保留历史

InspectBobbin 此前只改 bobbin.grade，从不写 BobbinGrade —— 质检的
缺陷原因、评分等在现场只产生一次，丢失即不可恢复。现每次质检新增
一条记录，并把 markableTables 与 collectors 一并扩展。

Refs: #17
EOF
)"
```

---

### Task 8: failed 重传端点（含全量重传）

**Files:**
- Create: `internal/sync/edge/resync.go`
- Create: `internal/sync/edge/resync_test.go`
- Modify: `internal/server/edge.go`（路由与 handler）

**Interfaces:**
- Consumes: 各实体的同步字段
- Produces:
  - `func (u *Uploader) ResyncFailed(ctx context.Context, table string) (int, error)` — 重置指定表的 failed 记录为 pending
  - `func (u *Uploader) ResyncAll(ctx context.Context, table string) (int, error)` — 重置指定表全部 synced 记录为 pending
  - `POST /v1/sync/resync` body: `{"mode": "failed"|"all", "table": "lots"|"bobbins"|"doffings"|"bobbin_grades"|""}`（table 空表示全部可采集表）

- [ ] **Step 1: 写失败的测试**

创建 `internal/sync/edge/resync_test.go`：

```go
package edge_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

func TestResyncFailed_ResetsFailedToPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-1").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusFailed).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncFailed(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncFailed: %v", err)
	}
	if n != 1 {
		t.Errorf("重置条数 = %d, want 1", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}

func TestResyncAll_ResetsSyncedToPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-2").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusSynced).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncAll(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncAll: %v", err)
	}
	if n != 1 {
		t.Errorf("重置条数 = %d, want 1", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if !got.SyncedAt.IsZero() {
		t.Errorf("SyncedAt 应被清空，实际 %v", got.SyncedAt)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/sync/edge/ -run TestResync -v`
Expected: FAIL — `undefined: u.ResyncFailed`

- [ ] **Step 3: 实现 resync**

创建 `internal/sync/edge/resync.go`：

```go
package edge

import (
	"context"
	"fmt"

	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
)

// ResyncFailed 把指定表中重试超限的记录重置为待同步。
//
// 用途：网络长期故障后记录被标记 failed（仓库原本没有任何回收路径，
// failed 即等于数据终点）。修好环境后由运维显式触发，归还完整重试预算。
// table 为空表示全部可采集表。
//
// 重置同时清空 synced_at 并把 sync_retry_count 归零 —— 只改状态而不
// 重置计数的话，采集器的 `SyncRetryCountLT(上限)` 过滤会把它们继续挡在
// 门外，而 markExhausted 又会立刻把它们打回 failed。
func (u *Uploader) ResyncFailed(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{statuses: []string{"failed"}})
}

// ResyncAll 把指定表中全部 synced 记录重置为待同步（全量重传）。
//
// 用途：载荷新增字段（如 grade/tare_weight）后，存量已 synced 的记录
// 不会自动重传，其新字段在 Center 永远为空。Center 已是 upsert，
// 重传不会产生重复行，故由运维按需触发即可。
func (u *Uploader) ResyncAll(ctx context.Context, table string) (int, error) {
	return u.resetByTable(ctx, table, resetOptions{statuses: []string{"synced"}})
}

type resetOptions struct {
	statuses []string
}

// resetByTable 按表分发重置操作。table 为空表示全部。
//
// 与 collectors()/markableTables 同源：这里能重置的表必须是可采集的表，
// 否则重置只会造出永远排不出去的积压。
func (u *Uploader) resetByTable(ctx context.Context, table string, opts resetOptions) (int, error) {
	tables := []string{"lots", "bobbins", "doffings", "bobbin_grades"}
	if table != "" {
		found := false
		for _, t := range tables {
			if t == table {
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("unknown table %q", table)
		}
		tables = []string{table}
	}

	total := 0
	for _, t := range tables {
		n, err := u.resetOneTable(ctx, t, opts)
		if err != nil {
			return total, fmt.Errorf("reset %s: %w", t, err)
		}
		total += n
	}
	return total, nil
}

// resetOneTable 对单表执行重置。返回受影响行数。
func (u *Uploader) resetOneTable(ctx context.Context, table string, opts resetOptions) (int, error) {
	switch table {
	case "lots":
		return u.client.Lot.Update().
			Where(lot.SyncStatusEQ(lot.SyncStatus(opts.statuses[0]))).
			SetSyncStatus(lot.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbins":
		return u.client.Bobbin.Update().
			Where(bobbin.SyncStatusEQ(bobbin.SyncStatus(opts.statuses[0]))).
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "doffings":
		return u.client.Doffing.Update().
			Where(doffing.SyncStatusEQ(doffing.SyncStatus(opts.statuses[0]))).
			SetSyncStatus(doffing.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	case "bobbin_grades":
		return u.client.BobbinGrade.Update().
			Where(bobbingrade.SyncStatusEQ(bobbingrade.SyncStatus(opts.statuses[0]))).
			SetSyncStatus(bobbingrade.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx)
	default:
		return 0, fmt.Errorf("unknown table %q", table)
	}
}
```

**注意：** `SyncStatusEQ` 的签名是 `func SyncStatusEQ(v SyncStatus) predicate.Lot`（各实体 where.go 中均有），单值过滤用它即可，不要用 `SyncStatusIn` 包一层。`SyncStatusIn` 只用于 `resetByTable` 需要多状态时——本计划的 `resetOptions.statuses` 目前只放一个元素，故一律用 `EQ`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/sync/edge/ -run TestResync -v`
Expected: PASS

- [ ] **Step 5: 实现 HTTP 端点**

在 `internal/server/edge.go` 的 `sync` 组内注册：

```go
			sync.POST("/resync", s.handleSyncResync) // 重置同步状态（failed 重传 / 全量重传）
```

并新增 handler（放在 `handleSyncDownload` 之后）：

```go
// handleSyncResync 重置本地同步状态，触发重传。
//
// mode=failed：把 failed 记录重置为 pending（修复长期故障后的积压）
// mode=all：把 synced 记录也一并重置（载荷新增字段后的存量回填）
//
// 需要显式调用而非自动重试：不给自动化开无限重试的口子，
// failed 的定位是"等人看一眼"，不是数据终点。
func (s *EdgeServer) handleSyncResync(c *gin.Context) {
	var req struct {
		Mode  string `json:"mode" binding:"required,oneof=failed all"`
		Table string `json:"table"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}

	var (
		n   int
		err error
	)
	switch req.Mode {
	case "failed":
		n, err = s.uploader.ResyncFailed(c.Request.Context(), req.Table)
	case "all":
		n, err = s.uploader.ResyncAll(c.Request.Context(), req.Table)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(gin.H{"reset": n, "mode": req.Mode, "table": req.Table}))
}
```

- [ ] **Step 6: 全量验证并提交**

Run: `go build ./... && go vet ./... && go test ./... 2>&1 | grep -E "FAIL|panic"`
Expected: 无输出

```bash
grep -c "[一-龥]" internal/sync/edge/resync.go internal/server/edge.go
git add internal/
git commit -m "$(cat <<'EOF'
feat: 新增同步状态重置端点（failed重传与全量重传）

failed 此前是数据终点：仓库没有任何回收路径，重试超限的记录永久滞留，
需人工改库。现提供显式重置端点，并提供全量重传以回填载荷新增字段导致的
存量空值（Center 已是 upsert，重传不产生重复）。

Refs: #17
EOF
)"
```

---

### Task 9: 删除 Center 越权写路由，Edge 补写路由

**Files:**
- Modify: `internal/server/center.go`（删 7 条路由）
- Modify: `internal/server/edge.go`（加 2 条路由）
- Modify: `api/edge/v1/bobbin.go`（新增两个 handler）
- Modify: `internal/server/real_router_test.go`（补路由表断言）

**Interfaces:**
- Consumes: Task 8 的 `Uploader.ResyncFailed`/`ResyncAll`（同属 Edge 侧改动，本任务只动路由与 handler）
- Produces: Center 移除 7 条写路由；Edge 新增 `PUT /v1/bobbins/:id/status` 与 `POST /v1/bobbins/:id/print`

**背景：** 数据归属是**严格单向**——丝饼的全部信息来自 Edge，Center 不写；批次 Center 只读汇总。Center 现有的 7 条写路由（`lots.POST`、`lots.PUT /:id/status`、`lots.DELETE`、`bobbins.POST`、`bobbins.PUT /:id/status`、`bobbins.POST /:id/print`、`bobbins.DELETE`）是设计定稿前的过渡产物（提交 `deaf696`，早于归属定稿 `5034415`），且在本仓库内**无任何调用方**。

- [ ] **Step 1: 写失败的测试**

在 `internal/server/real_router_test.go` 末尾追加：

```go
// 数据归属严格单向：Center 不得保留批次/丝饼的写路由。
// 这七条是设计定稿前的过渡产物，保留会与「Edge 是数据所有者」相抵。
func TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes(t *testing.T) {
	router, _, _ := newRealRouter(t)

	forbidden := []struct{ method, path string }{
		{http.MethodPost, "/v1/lots"},
		{http.MethodPut, "/v1/lots/:id/status"},
		{http.MethodDelete, "/v1/lots/:id"},
		{http.MethodPost, "/v1/bobbins"},
		{http.MethodPut, "/v1/bobbins/:id/status"},
		{http.MethodPost, "/v1/bobbins/:id/print"},
		{http.MethodDelete, "/v1/bobbins/:id"},
	}
	registered := make(map[string]struct{})
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	for _, f := range forbidden {
		if _, ok := registered[f.method+" "+f.path]; ok {
			t.Errorf("Center 不应注册 %s %s（数据归属单向：Center 只读）", f.method, f.path)
		}
	}
}

// 但只读端点必须保留
func TestRealRouter_CenterKeepsReadRoutes(t *testing.T) {
	router, _, _ := newRealRouter(t)
	registered := make(map[string]struct{})
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	for _, want := range []string{
		"GET /v1/lots",
		"GET /v1/lots/:id",
		"GET /v1/bobbins",
		"GET /v1/bobbins/:id",
	} {
		if _, ok := registered[want]; !ok {
			t.Errorf("缺少只读路由 %s", want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/server/ -run "TestRealRouter_CenterHasNo|TestRealRouter_CenterKeeps" -v`
Expected: `CenterHasNo...` FAIL（7 条路由仍存在）；`CenterKeeps...` PASS

- [ ] **Step 3: 删除 Center 的写路由**

在 `internal/server/center.go` 中，把 `lots` 与 `bobbins` 两组内除 GET 之外的注册行删除：

```go
		lots := authorized.Group("/lots")
		{
			lots.GET("", lotHandler.ListLots)
			lots.GET("/:id", lotHandler.GetLot)
			// 写接口不在 Center：批次由 Edge 创建并上传（数据归属单向）
		}

		bobbins := authorized.Group("/bobbins")
		{
			bobbins.GET("", bobbinHandler.ListBobbins)
			bobbins.GET("/:id", bobbinHandler.GetBobbin)
			// 写接口不在 Center：丝饼的全部信息来自 Edge
		}
```

删除后，若 `lotHandler`/`bobbinHandler` 的写方法变成无引用（`CreateLot`/`UpdateLotStatus`/`DeleteLot`/`CreateBobbin`/`UpdateBobbinStatus`/`MarkBobbinPrinted`/`DeleteBobbin`），**保留它们**——它们在 Edge 侧或将来可能被复用，且删除会波及 `internal/service` 与既有测试。在提交信息中说明保留原因。

**同时删除未使用的 import**（若 `api/center/v1` 下某文件的 handler 不再被 center.go 引用，Go 会报未使用——实际是包级引用，不会报错，故无需处理）。

- [ ] **Step 4: 给 Edge 补两个写端点**

在 `api/edge/v1/bobbin.go` 末尾新增：

```go
// UpdateBobbinStatus 更新丝饼状态（Edge 侧写入，Center 只读）
func (h *BobbinHandler) UpdateBobbinStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}
	if err := h.bobbinService.UpdateBobbinStatus(c.Request.Context(), c.Param("id"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, api.Success(gin.H{"updated": true}))
}

// MarkBobbinPrinted 标记标签已打印（Edge 侧写入）
func (h *BobbinHandler) MarkBobbinPrinted(c *gin.Context) {
	if err := h.bobbinService.MarkBobbinPrinted(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}
	c.JSON(http.StatusOK, api.Success(gin.H{"printed": true}))
}
```

**注意：** `api` 与 `gin` 的 import 与该文件既有 handler 一致（照抄文件顶部已有的 import 块）。本仓库没有共享的 error 转换辅助，既有 handler 一律内联 `c.JSON(status, api.Error(code, err.Error()))`，照此风格写。

在 `internal/server/edge.go` 的 `bobbins` 组内注册：

```go
			bobbins.PUT("/:id/status", bobbinHandler.UpdateBobbinStatus) // 更新状态
			bobbins.POST("/:id/print", bobbinHandler.MarkBobbinPrinted)  // 标记已打印
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/server/ -v`
Expected: 全部 PASS

- [ ] **Step 6: 验证变异能证伪**

手工实验（验证后还原）：把 `lots.POST` 一行加回 `center.go`

Run: `go test ./internal/server/ -run TestRealRouter_CenterHasNo -v`
Expected: **FAIL**

- [ ] **Step 7: 提交**

```bash
grep -c "[一-龥]" internal/server/center.go internal/server/edge.go api/edge/v1/bobbin.go
git add api/ internal/
git commit -m "$(cat <<'EOF'
refactor: 删除Center的批次/丝饼写路由，Edge补齐现场写入口

数据归属严格单向：丝饼全部信息来自 Edge，批次 Center 只读汇总。
Center 的 7 条写路由是设计定稿前的过渡产物（deaf696, 2026-09-28），
早于归属定稿（5034415），且仓库内无任何调用方。

删路由后 bobbin.status 与 label_printed 在 Edge 侧也无写入方，
故补 PUT /:id/status 与 POST /:id/print（Service 方法已存在）。
handler 方法保留未删——它们在 Edge 侧使用，删除会波及其它包。

Refs: #17
EOF
)"
```

---

### Task 10: 修正设计文档与 spec 的归属矛盾

**Files:**
- Modify: `docs/superpowers/specs/2026-09-29-backend-service-api-design.md`
- Modify: `docs/superpowers/specs/2026-10-08-sync-update-design.md`（把勘察发现的三处修正写回）

**背景：** spec 自身矛盾——§364 归属表写「Lot 只读汇总 | Edge 创建」，而 §272-276 接口清单给 Center 了 `POST/DELETE /v1/lots`。§367 又要求 Center「质检改等级」，与本轮的 grade 归 Edge 相抵。项目 `CLAUDE.md` 把"代码变文档跟"列为硬性规则。

- [ ] **Step 1: 修正 spec 的接口清单**

在 `docs/superpowers/specs/2026-09-29-backend-service-api-design.md` 的「生产管理」章节中，把批次端点改为只读：

```
GET    /v1/lots                          批次列表（全局）
GET    /v1/lots/:id                      批次详情
GET    /v1/doffings                      落纱记录（汇总）
GET    /v1/doffings/:id                  落纱详情
```

并在该代码块后加一段说明：

```markdown
**写接口归属：** 批次的创建与状态变更由 **Edge 端**负责（§364 归属表：`Lot | 只读汇总 | 创建/查询本地 | Edge 创建，上传至 Center`）。Center 侧仅提供只读汇总，不提供 `POST`/`PUT`/`DELETE`。丝饼同理——其全部信息来自 Edge。
```

- [ ] **Step 2: 修正 §291 的 grade 端点**

把 §291 的 `PUT /v1/bobbins/:id/grade 更新等级` 改为：

```
PUT    /v1/bobbins/:id/grade             更新等级（Edge 端）
```

并同步修正 §367 的归属表述：

```markdown
| Bobbin | 只读 | 创建/查询/称重/质检 | Edge 产生（含等级与重量），Center 只读 |
```

- [ ] **Step 3: 把勘察发现写回设计文档**

在 `docs/superpowers/specs/2026-10-08-sync-update-design.md` 的 §2.8 中修正 BobbinGrade 的描述：

```markdown
**启用的真实成本（已核实）：**

- `BobbinGrade` 与 `ProductConfig` 此前**从未被 ent 生成过**：其 `Annotations()` 返回类型写成 `[]entsql.Annotation`，而 ent 要求 `[]schema.Annotation`，导致 `types.Implements` 为假并被加载器静默跳过（`entc/load/load.go:147`），`ent generate` 退出码 0 却零产出。启用前必须先修签名。
- `InspectBobbin` 当前**完全不写 `BobbinGrade`**，只 `SetGrade` 到 `bobbin` 表。
- `BobbinGrade.Edges()` 返回 `nil`——没有任何外键约束，`bobbin_id` 有值但不指向 `bobbin` 表。
- `BobbinGrade` 原本**没有** `sync_status`/`sync_retry_count`/`synced_at`，需加 schema 字段 + 迁移 + 重新生成 ent。
- 其 `bobbin_id` 原带**唯一索引**（注释「一个丝锭只有一条质检记录」），与「只增历史」冲突，必须改为非唯一。
```

并把 §1.2 第 3 行更正为：

```markdown
| 3 | 上传载荷不含 `grade` 与 `tare_weight`（Center 的 `createBobbin` 已能接收这两个字段，是 Edge 侧没发） | `internal/sync/edge/uploader.go` |
```

- [ ] **Step 4: 验证文档一致性**

Run: `grep -n "POST   /v1/lots\|DELETE /v1/lots" docs/superpowers/specs/2026-09-29-backend-service-api-design.md`
Expected: 无输出（写端点已移除）

Run: `grep -c "质检改等级" docs/superpowers/specs/2026-09-29-backend-service-api-design.md`
Expected: `0`

- [ ] **Step 5: 提交**

```bash
grep -c "[一-龥]" docs/superpowers/specs/2026-09-29-backend-service-api-design.md docs/superpowers/specs/2026-10-08-sync-update-design.md
git add docs/
git commit -m "$(cat <<'EOF'
docs: 修正spec的归属矛盾与BobbinGrade的生成前提

spec 归属表（§364 Lot 只读汇总）与接口清单（§272-276 给 Center 写端点）
自相矛盾；§291/§367 又要求 Center 改等级。统一为单向：Edge 写、Center 读。
并把 BobbinGrade 未被生成的根因（Annotations 签名不匹配）写回设计文档。

Refs: #17
EOF
)"
```

---

## 完成标准

```bash
go build ./... && go vet ./...                      # 无输出
go test ./... 2>&1 | grep -c FAIL                   # 0
go run -mod=mod entgo.io/ent/cmd/ent describe ./internal/database/ent/schema | grep -c "BobbinGrade:"   # 1
```

且下列行为逐条有测试断言：

- 业务字段被修改后记录重新变 `pending`，计数归零（Task 2）
- 只改同步字段的写入不被拦回 `pending`（Task 2，防无限重传）
- `failed` 记录被编辑后复活（Task 2）
- 载荷含 `grade`/`tare_weight`/`lot_number`（Task 3）
- 重传同一批不产生重复行且字段被更新（Task 4）
- Center 已有同编号行时被认领而非报错（Task 4）
- 丝饼按 `lot_number` 挂到 Center 的批次（Task 5）
- 落纱落库且状态流转走同一行（Task 6）
- 同一丝饼质检两次留下两条记录（Task 7）
- `ResyncFailed`/`ResyncAll` 重置状态与计数（Task 8）
- Center 不再注册批次/丝饼写路由（Task 9）

## 已知未覆盖（本计划范围外）

- `delete` 同步（涉及软删/硬删语义）
- `barrels` 采集（Edge 侧无建桶路径，无数据可同步）
- Edge 侧 `lots`/`bobbins` 创建 API（属 Phase 2–5）
- `Module`/`Pallet`/`Carton` 同步与其 Center 只读端点
- Center 侧 `Edge.status` 超时离线判定
- `BobbinGrade` 的 `inspection_details`/`scores`/`final_score` 明细字段同步（本轮只同步 `grade`/`is_qualified`/`inspector_id`/`defect_reason`/`inspected_at`）
