# Center 报表聚合端点 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Center 管理后台的报表页提供「质量分布」与「机台与操作员」两个聚合查询端点。

**Architecture:** 新增 `ReportService`（`internal/service/report.go`）承载两个 Ent `GroupBy` 聚合查询，新增 `ReportHandler`（`api/center/v1/report.go`）负责时间区间与 `top` 参数的解析及错误映射，两个端点注册在 Center 已有的 `authorized` 路由组下（组级 `requireUserPrincipal` 已覆盖，不挂 `requireRole`）。

**Tech Stack:** Go 1.26.4 / Ent v0.14.6 / Gin / 测试用 `modernc.org/sqlite` 内存库。

**Spec:** `docs/superpowers/specs/2026-11-09-report-aggregation-design.md`

## Global Constraints

- **只读**：两个端点不得引入任何写路径。批次与丝饼的写路由已删除并被 `TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 钉住，该测试与 `TestRealRouter_CenterKeepsReadRoutes` 的断言强度不得放宽。
- **权限**：两个端点挂 `authorized` 组，**不挂 `requireRole`**（spec `2026-10-12-frontend-admin-design.md` §6.3：报表对全部已登录角色开放）。
- **时间区间**：`start` 闭、`end` 开；缺省 `[now-7d, now)`；RFC3339 解析；`end <= start` 报错；跨度 > 366 天报错。
- **计数单位**：`bobbin_grade` 按**行**计数，同一丝饼复检各计一次，不做去重。
- **落纱过滤**：`doffing` 聚合排除 `status = cancelled`，保留 `pending` 与 `confirmed`。
- **空分组键不丢弃**：`operator_id` / `inspector_id` 为 NULL 的行归入 `null` 组并出现在响应中。
- **`top` 参数**：缺省 10，合法区间 1–50，越界返回 400（不静默截断）。
- **编码**：所有文本文件 UTF-8 无 BOM、行尾 LF（项目 `.editorconfig` / `.gitattributes` 强制）。
- **提交**：每个任务单独 commit，信息包含 `Refs:` 行。

## Review Focus

以下是 spec 隐含但没有任何单个任务测试能覆盖的输入类别，按最容易伤到用户的程度排序。每一行都在下方指定的任务里配了测试。

1. **同一丝饼的多次质检**：用户会看到合格率被返修拉低，期望它反映「质检次数」而非「丝饼数」——Task 2 的 `复检各计一次` 用例钉住它。
2. **未记录操作员/检测员的记录**：这些行可能占相当比例，用户期望它们出现在统计里而不是消失——Task 3 的 `null 组` 用例钉住它。
3. **作废的落纱**：用户会把机台产量与操作员落纱数对齐比较，作废记录混进来会让两边都虚高——Task 3 的 `取消的落纱` 用例钉住它。
4. **空库 / 空区间**：报表在刚部署或查询窄区间时必然遇到，用户期望是 `0` 和空数组，而不是 `null` 或 500——Task 2、Task 3 的边界用例钉住它。
5. **前端传了离谱的区间或 `top`**：用户期望明确的 400 而不是被静默截断后收到一份看似正常的报表——Task 1 的解析用例钉住它。

---

## 文件结构

| 文件 | 责任 |
|---|---|
| `internal/service/report.go` | `ReportService` 与两个响应结构体；聚合查询与空值归一化 |
| `internal/service/report_test.go` | 服务层测试（内存 SQLite，自造种子） |
| `api/center/v1/report.go` | `ReportHandler` + `parseTimeRange` / `parseTopN` |
| `api/center/v1/report_test.go` | 两个解析函数的表驱动测试 |
| `api/center/v1/export_test.go` | 追加两个包私有函数的测试别名 |
| `internal/server/center.go` | 在 `authorized` 组内注册 `/v1/reports` 子组 |
| `internal/server/real_router_test.go` | 路由存在性、设备凭证拒绝、viewer 放行 |

---

### Task 1: 参数解析（`parseTimeRange` / `parseTopN`）

**Files:**
- Create: `api/center/v1/report.go`
- Create: `api/center/v1/report_test.go`
- Modify: `api/center/v1/export_test.go`

**Interfaces:**
- Consumes: 无。
- Produces:
  - `parseTimeRange(startStr, endStr string, now time.Time) (time.Time, time.Time, error)`
  - `parseTopN(raw string) (int, error)`
  - 测试别名：`var ParseTimeRange = parseTimeRange`、`var ParseTopN = parseTopN`

- [ ] **Step 1: 写失败的测试**

在 `api/center/v1/report_test.go`（`package v1_test`）：

```go
func TestParseTimeRange(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		start   string
		end     string
		wantErr bool
		wantStart time.Time
		wantEnd   time.Time
	}{
		{"都不传 → 最近七天", "", "", false,
			now.Add(-7 * 24 * time.Hour), now},
		{"只传 start", "2026-11-01T00:00:00Z", "", false,
			time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), now},
		{"只传 end", "", "2026-11-05T00:00:00Z", false,
			time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)},
		{"start 晚于 end", "2026-11-09T00:00:00Z", "2026-11-08T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 等于 end", "2026-11-09T00:00:00Z", "2026-11-09T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 格式非法", "2026/11/01", "", true, time.Time{}, time.Time{}},
		{"end 格式非法", "", "not-a-time", true, time.Time{}, time.Time{}},
		{"跨度 367 天", "2025-11-08T00:00:00Z", "2026-11-10T00:00:00Z", true, time.Time{}, time.Time{}},
		{"跨度恰好 366 天", "2025-11-09T00:00:00Z", "2026-11-10T00:00:00Z", false,
			time.Date(2025, 11, 9, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := centerv1.ParseTimeRange(tc.start, tc.end, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got start=%v end=%v", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Errorf("got [%v, %v), want [%v, %v)", start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestParseTopN(t *testing.T) {
	cases := []struct {
		raw     string
		want    int
		wantErr bool
	}{
		{"", 10, false},
		{"1", 1, false},
		{"50", 50, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"51", 0, true},
		{"abc", 0, true},
	}

	for _, tc := range cases {
		t.Run("top="+tc.raw, func(t *testing.T) {
			got, err := centerv1.ParseTopN(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./api/center/v1/ -run 'TestParseTimeRange|TestParseTopN' -v`
Expected: 编译失败，`undefined: centerv1.ParseTimeRange` / `ParseTopN`。

- [ ] **Step 3: 实现**

在 `api/center/v1/report.go` 实现两个函数，包名 `v1`，import `errors` / `fmt` / `strconv` / `time`：

- `parseTimeRange`：两者皆空 → `end = now, start = now.Add(-7*24*time.Hour)`；只传 `start` → `end = now`；只传 `end` → `start = end.Add(-7*24*time.Hour)`。时间用 `time.Parse(time.RFC3339, ...)` 解析，失败时用 `fmt.Errorf` 在 message 中带上参数名（`start` / `end`）。若 `!end.After(start)` 返回错误。若 `end.Sub(start) > 366*24*time.Hour` 返回错误。返回值统一 `.UTC()`。
- `parseTopN`：空串返回 `10, nil`；`strconv.Atoi` 失败返回错误；`< 1 || > 50` 返回错误。
- 用 `errors.New` 定义两个包级错误变量 `errEndNotAfterStart`、`errRangeTooLong`，便于测试与将来判等（非必需，但避免字符串比较）。

同时在 `api/center/v1/export_test.go` 追加：

```go
// 报表参数解析函数是包私有的，v1_test 无法直接调用。
var ParseTimeRange = parseTimeRange
var ParseTopN = parseTopN
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./api/center/v1/ -run 'TestParseTimeRange|TestParseTopN' -v`
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add api/center/v1/report.go api/center/v1/report_test.go api/center/v1/export_test.go
git commit -m "feat: 报表端点的时间区间与 top 参数解析"
```

---

### Task 2: `ReportService.QualityDistribution`

**Files:**
- Create: `internal/service/report.go`
- Create: `internal/service/report_test.go`

**Interfaces:**
- Consumes: Task 1 的 `parseTimeRange`（仅 handler 用，本任务不涉及）。
- Produces:
  - `func NewReportService(client *ent.Client) *ReportService`
  - `func (s *ReportService) QualityDistribution(ctx context.Context, start, end time.Time, top int) (*QualityDistributionResponse, error)`
  - 类型：`QualityDistributionResponse`、`QualifiedStats`、`GradeCount`、`DefectCount`

```go
type QualityDistributionResponse struct {
	Range     ReportRange   `json:"range"`
	Qualified QualifiedStats `json:"qualified"`
	ByGrade   []GradeCount  `json:"by_grade"`
	TopDefects []DefectCount `json:"top_defects"`
}

type ReportRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type QualifiedStats struct {
	QualifiedCount   int     `json:"qualified_count"`
	UnqualifiedCount int     `json:"unqualified_count"`
	UnspecifiedCount int     `json:"unspecified_count"`
	TotalCount       int     `json:"total_count"`
	QualifiedRate    float64 `json:"qualified_rate"`
}

type GradeCount struct {
	Grade string `json:"grade"`
	Count int    `json:"count"`
}

type DefectCount struct {
	DefectReason string `json:"defect_reason"`
	Count        int    `json:"count"`
}
```

- [ ] **Step 1: 写失败的测试**

在 `internal/service/report_test.go`（`package service_test`），用 `setup_test.go` 的 `newTestClient(t)`。先写一个种子辅助函数：

```go
// seedGrades 造 n 条质检记录。isQualified 用 *bool 三态；
// defectReason 为空串时不设置（落 NULL）。
func seedGrades(t *testing.T, client *ent.Client, records []gradeSeed) {
	t.Helper()
	ctx := context.Background()
	for i, r := range records {
		b := client.BobbinGrade.Create().
			SetBobbinID(uuid.New()).
			SetGrade(r.grade).
			SetInspectedAt(r.inspectedAt)
		if r.isQualified != nil {
			b.SetNillableIsQualified(r.isQualified)
		}
		if r.defectReason != "" {
			b.SetDefectReason(r.defectReason)
		}
		if _, err := b.Save(ctx); err != nil {
			t.Fatalf("seed grade #%d: %v", i, err)
		}
	}
}
```

（`gradeSeed` 为测试内私有 struct，字段 `grade string`、`inspectedAt time.Time`、`isQualified *bool`、`defectReason string`；`boolPtr` 已在 `internal/service/user_test.go` 中定义，同包可直接复用。）

然后写这些用例：

| 测试函数 | 种子 | 断言 |
|---|---|---|
| `TestReportService_QualityDistribution_QualifiedRate` | 6 条：3 true / 1 false / 2 NULL | `QualifiedRate == 0.75`、`UnspecifiedCount == 2`、`TotalCount == 6` |
| `TestReportService_QualityDistribution_AllUnspecified` | 2 条全 NULL | `QualifiedRate == 0`、`TotalCount == 2` |
| `TestReportService_QualityDistribution_RepeatInspectionCountedTwice` | 同一 `BobbinID` 两条（false + true） | `TotalCount == 2`、`QualifiedRate == 0.5` |
| `TestReportService_QualityDistribution_GradeGroups` | A×2、B×1 | `ByGrade` 两行，A 在前且 `Count == 2` |
| `TestReportService_QualityDistribution_TopDefectsTruncates` | 毛丝×5、断头×3、油污×1，`top=2` | 返回 2 行，首行 `DefectReason == "毛丝"` |
| `TestReportService_QualityDistribution_SkipsNullAndEmptyDefect` | 一条 NULL、一条空串、一条「毛丝」 | `TopDefects` 只有 1 行 |
| `TestReportService_QualityDistribution_TimeWindow` | `inspected_at` 落在窗口内、外各一条 | 只计入窗口内那条 |
| `TestReportService_QualityDistribution_EmptyRange` | 空库 | `TotalCount == 0`、`ByGrade != nil` 且 `len == 0` |

`RepeatInspectionCountedTwice` 用显式 `SetBobbinID(fixedID)` 造两条同 ID 记录。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestReportService_QualityDistribution -v`
Expected: 编译失败，`undefined: service.ReportService` / `NewReportService`。

- [ ] **Step 3: 实现**

在 `internal/service/report.go`：

```go
type ReportService struct{ client *ent.Client }
func NewReportService(client *ent.Client) *ReportService
```

`QualityDistribution` 的实现要点（签名与结构体见上）：

1. 构造基础查询：`s.client.BobbinGrade.Query().Where(bobbingrade.InspectedAtGTE(start), bobbingrade.InspectedAtLT(end))`。
2. 三态计数用三次 `Count`，比一次 `GroupBy(FieldIsQualified)` 更直白：分别对 `IsQualified(true)`、`IsQualified(false)`、`IsQualifiedIsNil()` 计数（`IsQualifiedIsNil()` 已存在于 `ent/bobbingrade/where.go`），再加总。三次查询的 `Where` 与第 1 步相同。
3. `QualifiedRate = float64(qualified) / float64(qualified+unqualified)`，分母为 0 时取 `0`；结果保留三位小数（`math.Round(x*1000)/1000`）。
4. `ByGrade`：`GroupBy(bobbingrade.FieldGrade).Aggregate(ent.Count()).Scan(ctx, &rows)`，`rows` 元素类型 `struct{ Grade string; Count int }`；排序：`Count` 降序，同值按 `Grade` 升序。结果用 `make([]GradeCount, 0, len(rows))` 初始化，保证空结果是 `[]` 而非 `null`。
5. `TopDefects`：`GroupBy(bobbingrade.FieldDefectReason).Aggregate(ent.Count())`，`Where` 追加 `bobbingrade.DefectReasonNotNil(), bobbingrade.DefectReasonNEQ("")`；排序同上；截断到 `top` 行。
6. `Range` 用 `start.UTC().Format(time.RFC3339)` / `end.UTC().Format(time.RFC3339)` 填充。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestReportService_QualityDistribution -v`
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/service/report.go internal/service/report_test.go
git commit -m "feat: 质量分布聚合查询"
```

---

### Task 3: `ReportService.MachineOperator`

**Files:**
- Modify: `internal/service/report.go`
- Modify: `internal/service/report_test.go`

**Interfaces:**
- Consumes: Task 2 的 `ReportService`、`NewReportService`、`ReportRange`。
- Produces:
  - `func (s *ReportService) MachineOperator(ctx context.Context, start, end time.Time) (*MachineOperatorResponse, error)`
  - 类型：`MachineOperatorResponse`、`SpinningLineStat`、`OperatorStat`、`InspectorStat`

```go
type MachineOperatorResponse struct {
	Range          ReportRange       `json:"range"`
	BySpinningLine []SpinningLineStat `json:"by_spinning_line"`
	ByOperator     []OperatorStat    `json:"by_operator"`
	ByInspector    []InspectorStat   `json:"by_inspector"`
}

type SpinningLineStat struct {
	SpinningLineID string  `json:"spinning_line_id"`
	DoffingCount   int     `json:"doffing_count"`
	ActualWeight   float64 `json:"actual_weight"`
}

type OperatorStat struct {
	OperatorID   *string `json:"operator_id"`   // NULL 组为 null，不带 omitempty
	DoffingCount int     `json:"doffing_count"`
}

type InspectorStat struct {
	InspectorID     *string `json:"inspector_id"` // 同上
	InspectionCount int     `json:"inspection_count"`
}
```

- [ ] **Step 1: 写失败的测试**

在 `internal/service/report_test.go` 追加种子辅助（`doffing` 的 `lot_id` 是必填外键，必须先造 lot）：

```go
// seedDoffing 造一条落纱。lotID 由调用方先建批次取得。
//
// spinning_position 是 Positive() 必填项，lot_id 是 Required 外键，二者都必须设。
func seedDoffing(t *testing.T, client *ent.Client, lotID uuid.UUID, lineID uuid.UUID,
	operatorID *uuid.UUID, status doffing.Status, weight *float64, at time.Time) {
	t.Helper()

	b := client.Doffing.Create().
		SetLotID(lotID).
		SetSpinningLineID(lineID).
		SetSpinningPosition(1).
		SetStatus(status).
		SetDoffingTime(at)
	if operatorID != nil {
		b.SetOperatorID(*operatorID)
	}
	if weight != nil {
		b.SetActualWeight(*weight)
	}
	if _, err := b.Save(context.Background()); err != nil {
		t.Fatalf("seed doffing: %v", err)
	}
}
```

用例：

| 测试函数 | 种子 | 断言 |
|---|---|---|
| `TestReportService_MachineOperator_BySpinningLine` | 同一条线两条落纱，`actual_weight` 100 / 50 | `DoffingCount == 2`、`ActualWeight == 150` |
| `TestReportService_MachineOperator_NullActualWeight` | 一条 `actual_weight` 未设置、一条 50 | `ActualWeight == 50`（NULL 不使整组归零） |
| `TestReportService_MachineOperator_ExcludesCancelled` | 同一机台 `confirmed` + `cancelled` | `DoffingCount == 1` |
| `TestReportService_MachineOperator_NullOperatorGroup` | 一条有 `operator_id`、一条 NULL | `len(ByOperator) == 2`，其中一条 `OperatorID == nil` |
| `TestReportService_MachineOperator_ByInspector` | 两个检测员各 1 / 2 条质检 | 计数分别为 1 与 2 |
| `TestReportService_MachineOperator_NullInspectorGroup` | 一条 `inspector_id` 为 NULL | 该行 `InspectorID == nil` 且出现在结果中 |
| `TestReportService_MachineOperator_TimeWindow` | `doffing_time` 落在窗口内、外各一条 | 只计入窗口内那条 |
| `TestReportService_MachineOperator_EmptyRange` | 空库 | 三个切片均非 nil 且长度为 0 |

`NullOperatorGroup` 与 `NullInspectorGroup` 是 Review Focus 第 2 条对应的防线：扫描目标若写成 `uuid.UUID`，NULL 会静默变成 `uuid.Nil`，两个用例会各自失败。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/service/ -run TestReportService_MachineOperator -v`
Expected: 编译失败，`undefined: (*service.ReportService).MachineOperator`。

- [ ] **Step 3: 实现**

`MachineOperator` 要点：

1. 落纱基础查询：`s.client.Doffing.Query().Where(doffing.DoffingTimeGTE(start), doffing.DoffingTimeLT(end), doffing.StatusNEQ(doffing.StatusCancelled))`。
2. `BySpinningLine`：`GroupBy(doffing.FieldSpinningLineID).Aggregate(ent.Count(), ent.Sum(doffing.FieldActualWeight))`。扫描目标用匿名 struct，求和字段用 `sql.NullFloat64`（`database/sql`），再归一化为 `0`。注意 `ent.Sum` 的扫描列名取决于 ent 生成的聚合别名——**以运行结果为准**，若 Scan 报列不匹配，先打印 `GroupBy(...).Aggregate(...)` 生成的 SQL（`grbuild` 支持 `.Scan` 前用 `client.Debug()`）确认别名，再改 struct tag。
3. `ByOperator`：`GroupBy(doffing.FieldOperatorID).Aggregate(ent.Count())`，扫描目标 `struct{ OperatorID *uuid.UUID; Count int }`；`nil` 转为响应里的 `nil *string`，非 nil 时 `id.String()` 取址。
4. `ByInspector`：`s.client.BobbinGrade.Query().Where(bobbinGrade.InspectedAtGTE(start), InspectedAtLT(end)).GroupBy(bobbingrade.FieldInspectorID).Aggregate(ent.Count())`，扫描目标同理用 `*uuid.UUID`。
5. 三个切片用 `make([]T, 0, n)` 初始化；排序：计数降序，同值按键升序（`nil` 排最后）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/service/ -run TestReportService_MachineOperator -v`
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/service/report.go internal/service/report_test.go
git commit -m "feat: 机台与操作员聚合查询"
```

---

### Task 4: `ReportHandler` 与路由注册

**Files:**
- Modify: `api/center/v1/report.go`
- Modify: `internal/server/center.go`
- Modify: `internal/server/real_router_test.go`

**Interfaces:**
- Consumes: Task 1 的 `parseTimeRange` / `parseTopN`；Task 2、3 的 `NewReportService`、`QualityDistribution`、`MachineOperator`。
- Produces: 两条活路由 `GET /v1/reports/quality-distribution`、`GET /v1/reports/machine-operator`。

- [ ] **Step 1: 写失败的测试**

在 `internal/server/real_router_test.go` 追加：

```go
func TestRealRouter_ReportRoutesExist(t *testing.T) {
	router, _, _ := newRealRouter(t)

	paths := map[string]bool{}
	for _, r := range router.Routes() {
		paths[r.Path] = true
	}

	for _, want := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		if !paths[want] {
			t.Errorf("route %s not registered", want)
		}
	}
}

func TestRealRouter_DeviceTokenBlockedFromReports(t *testing.T) {
	router, deviceToken, _ := newRealRouter(t)

	for _, path := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+deviceToken)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("设备凭证访问 %s → %d, want 403", path, w.Code)
		}
	}
}

func TestRealRouter_ViewerPassesReportGuard(t *testing.T) {
	router, viewerToken := newRealRouterWithRole(t, "viewer")

	for _, path := range []string{
		"/v1/reports/quality-distribution",
		"/v1/reports/machine-operator",
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		router.ServeHTTP(w, req)

		if w.Code == http.StatusForbidden {
			t.Errorf("viewer 访问 %s 被 403 拒绝，报表应对全部已登录角色开放", path)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/server/ -run 'TestRealRouter_(ReportRoutesExist|DeviceTokenBlockedFromReports|ViewerPassesReportGuard)' -v`
Expected: `ReportRoutesExist` 报 `route ... not registered`；另两条返回 404 而非 403，均变红。

- [ ] **Step 3: 实现 handler 与路由**

`api/center/v1/report.go` 追加：

```go
type ReportHandler struct{ reportService *service.ReportService }
func NewReportHandler(s *service.ReportService) *ReportHandler

func (h *ReportHandler) QualityDistribution(c *gin.Context)
func (h *ReportHandler) MachineOperator(c *gin.Context)
```

两个 handler 的结构一致：
1. `start, end, err := parseTimeRange(c.Query("start"), c.Query("end"), time.Now())`；失败 → `400` + `api.Error(api.CodeInvalidParams, err.Error())`。
2. `QualityDistribution` 额外 `top, err := parseTopN(c.Query("top"))`，失败同上。
3. 调用 service，错误 → `500` + `api.Error(api.CodeServerError, err.Error())`。
4. 成功 → `c.JSON(http.StatusOK, api.Success(resp))`。

`internal/server/center.go`：在 `authorized` 组内、`bobbins` 块之后追加：

```go
// 报表对全部已登录角色开放（spec 2026-10-12 §6.3），故不挂 requireRole。
reportService := service.NewReportService(s.client)
reportHandler := centerv1.NewReportHandler(reportService)

reports := authorized.Group("/reports")
{
    reports.GET("/quality-distribution", reportHandler.QualityDistribution)
    reports.GET("/machine-operator", reportHandler.MachineOperator)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/server/ -run TestRealRouter -v`
Expected: 全部 PASS，含既有的 `TestRealRouter_CenterHasNoLotOrBobbinWriteRoutes` 与 `TestRealRouter_CenterKeepsReadRoutes`。

- [ ] **Step 5: 全量验证**

Run: `go build ./... && go test ./...`
Expected: 8 个包 `ok`，0 `FAIL`。

- [ ] **Step 6: 提交**

```bash
git add api/center/v1/report.go internal/server/center.go internal/server/real_router_test.go
git commit -m "feat: 挂载报表聚合端点"

Refs: #8
```

---

## 计划自检

- **spec 覆盖**：§2.1 六项指标 → Task 2（1–3）+ Task 3（4–6）；§3.1 权限 → Task 4 Step 1 三条路由测试；§3.2 时间参数 → Task 1；§3.2.1 计数单位 → Task 2 的 `RepeatInspectionCountedTwice`；§3.2.2 落纱过滤 → Task 3 的 `ExcludesCancelled`；§3.3 `top` 参数 → Task 1 与 Task 2 的 `TopDefectsTruncates`；§4 实现结构 → 文件结构表；§7 验收标准 → Task 4 Step 5。
- **不做项**：看板一、五维度合格率、平均分、包装单元、分页、导出均无对应任务，与 §2.2 一致。
- **类型一致性**：`ReportRange` 由 Task 2 定义、Task 3 复用；`parseTimeRange` 与 `parseTopN` 的签名在 Task 1 定稿，Task 4 按此调用。
