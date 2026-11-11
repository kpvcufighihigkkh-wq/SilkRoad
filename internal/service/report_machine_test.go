package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// seedLot 造一个最小可用批次，供落纱的 lot_id 外键使用。
func seedLot(t *testing.T, client *ent.Client, number string) uuid.UUID {
	t.Helper()

	row, err := client.Lot.Create().
		SetLotNumber(number).
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed lot %s: %v", number, err)
	}

	return row.ID
}

// seedDoffing 造一条落纱。
//
// spinning_position 是 Positive() 必填项，lot_id 是 Required 外键，二者都必须设；
// operator_id 与 actual_weight 可空，用 nil 表达「未记录」。
func seedDoffing(t *testing.T, client *ent.Client, lotID, lineID uuid.UUID,
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

func TestReportService_MachineOperator_BySpinningLine(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-001")
	lineRow, err := client.SpinningLine.Create().
		SetLineName("A线").
		SetLineNumber("LINE-A").
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed spinning line: %v", err)
	}
	lineID := lineRow.ID
	w1, w2 := 100.0, 50.0
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, &w1, inside)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, &w2, inside.Add(time.Minute))

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.BySpinningLine) != 1 {
		t.Fatalf("len(BySpinningLine) = %d, want 1: %+v", len(got.BySpinningLine), got.BySpinningLine)
	}
	row := got.BySpinningLine[0]
	if row.SpinningLineID != lineID.String() {
		t.Errorf("SpinningLineID = %s, want %s", row.SpinningLineID, lineID)
	}
	// 名称来自 Center 本地线体表，不依赖 Edge 回传。
	if row.SpinningLineName == nil || *row.SpinningLineName != "A线" {
		t.Errorf("SpinningLineName = %v, want A线", row.SpinningLineName)
	}
	if row.DoffingCount != 2 {
		t.Errorf("DoffingCount = %d, want 2", row.DoffingCount)
	}
	if row.ActualWeight != 150 {
		t.Errorf("ActualWeight = %v, want 150", row.ActualWeight)
	}
}

// 线体表里查不到该 ID 时名称应为 null，而不是空串或省略键。
//
// 这不是异常：spinning_line_id 来自 Edge，Center 未必有对应行
// （Edge 未拉取过线体配置）。前端需要能区分「无名」与「无此行」。
func TestReportService_MachineOperator_UnknownLineNameIsNull(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-UNKNOWN")
	lineID := uuid.New() // Center 线体表里不存在
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, inside)

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.BySpinningLine) != 1 {
		t.Fatalf("len(BySpinningLine) = %d, want 1", len(got.BySpinningLine))
	}
	if got.BySpinningLine[0].SpinningLineName != nil {
		t.Errorf("未知线体的名称应为 nil, got %q", *got.BySpinningLine[0].SpinningLineName)
	}
}

// SUM 在整列为 NULL 时返回 NULL；若不归一化，扫描会失败或整组归零。
func TestReportService_MachineOperator_NullActualWeight(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-002")
	lineID := uuid.New()
	w := 50.0
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, inside)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, &w, inside)

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.BySpinningLine) != 1 {
		t.Fatalf("len(BySpinningLine) = %d, want 1", len(got.BySpinningLine))
	}
	if got.BySpinningLine[0].ActualWeight != 50 {
		t.Errorf("ActualWeight = %v, want 50（NULL 不应使整组归零）", got.BySpinningLine[0].ActualWeight)
	}
	if got.BySpinningLine[0].DoffingCount != 2 {
		t.Errorf("DoffingCount = %d, want 2（权重为 NULL 的记录仍是一次落纱）", got.BySpinningLine[0].DoffingCount)
	}
}

// 作废的落纱不是产量：计入会让机台产量与操作员落纱数双双虚高。
func TestReportService_MachineOperator_ExcludesCancelled(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-003")
	lineID := uuid.New()
	w := 10.0
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, &w, inside)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusCancelled, &w, inside)

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.BySpinningLine) != 1 || got.BySpinningLine[0].DoffingCount != 1 {
		t.Errorf("BySpinningLine = %+v, want 单行 doffing_count=1（排除 cancelled）", got.BySpinningLine)
	}
}

// operator_id 为 NULL 的落纱归入 null 组，不得被丢弃。
//
// 这条同时是扫描类型的防线：扫进非指针 uuid.UUID 时 NULL 会静默变成
// uuid.Nil，与真实零值 UUID 无法区分，测试会在这里变红。
func TestReportService_MachineOperator_NullOperatorGroup(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-004")
	lineID := uuid.New()
	opID := uuid.New()
	seedDoffing(t, client, lotID, lineID, &opID, doffing.StatusConfirmed, nil, inside)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, inside)

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.ByOperator) != 2 {
		t.Fatalf("len(ByOperator) = %d, want 2（有 ID 与 NULL 各一组）: %+v", len(got.ByOperator), got.ByOperator)
	}

	var sawNil, sawReal bool
	for _, row := range got.ByOperator {
		switch {
		case row.OperatorID == nil:
			sawNil = true
		case *row.OperatorID == opID.String():
			sawReal = true
		}
	}
	if !sawNil {
		t.Error("缺少 operator_id 为 null 的分组")
	}
	if !sawReal {
		t.Errorf("缺少 operator_id = %s 的分组", opID)
	}
}

func TestReportService_MachineOperator_ByInspector(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	insp1, insp2 := uuid.New(), uuid.New()
	// inspector_id 没有走 gradeSeed（它不覆盖该字段），逐条显式 Create。
	for i, insp := range []uuid.UUID{insp1, insp2, insp1} {
		if _, err := client.BobbinGrade.Create().
			SetBobbinID(uuid.New()).
			SetInspectorID(insp).
			SetGrade("A").
			SetInspectedAt(inside.Add(time.Duration(i) * time.Minute)).
			Save(context.Background()); err != nil {
			t.Fatalf("seed inspector grade #%d: %v", i, err)
		}
	}

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	counts := map[string]int{}
	for _, row := range got.ByInspector {
		if row.InspectorID != nil {
			counts[*row.InspectorID] = row.InspectionCount
		}
	}
	if counts[insp1.String()] != 2 {
		t.Errorf("insp1 计数 = %d, want 2", counts[insp1.String()])
	}
	if counts[insp2.String()] != 1 {
		t.Errorf("insp2 计数 = %d, want 1", counts[insp2.String()])
	}
}

func TestReportService_MachineOperator_NullInspectorGroup(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: inside}, // inspector_id 为 NULL
	})

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.ByInspector) != 1 {
		t.Fatalf("len(ByInspector) = %d, want 1: %+v", len(got.ByInspector), got.ByInspector)
	}
	if got.ByInspector[0].InspectorID != nil {
		t.Errorf("InspectorID = %v, want nil（NULL 组不得变成零值 UUID）", *got.ByInspector[0].InspectorID)
	}
	if got.ByInspector[0].InspectionCount != 1 {
		t.Errorf("InspectionCount = %d, want 1", got.ByInspector[0].InspectionCount)
	}
}

// 落纱端点的时间字段是 doffing_time；错接成 created_at 等字段不会有任何报错。
func TestReportService_MachineOperator_TimeWindow(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, before, after := reportWindow()

	lotID := seedLot(t, client, "LOT-MO-005")
	lineID := uuid.New()
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, before)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, inside)
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, end) // 右开
	seedDoffing(t, client, lotID, lineID, nil, doffing.StatusConfirmed, nil, after)

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if len(got.BySpinningLine) != 1 || got.BySpinningLine[0].DoffingCount != 1 {
		t.Errorf("BySpinningLine = %+v, want 单行 doffing_count=1（[start,end) 左闭右开）", got.BySpinningLine)
	}
}

func TestReportService_MachineOperator_EmptyRange(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, _, _, _ := reportWindow()

	got, err := svc.MachineOperator(context.Background(), start, end)
	if err != nil {
		t.Fatalf("MachineOperator: %v", err)
	}

	if got.BySpinningLine == nil || got.ByOperator == nil || got.ByInspector == nil {
		t.Error("空结果应为空切片而非 nil（JSON 应输出 [] 而非 null）")
	}
	if len(got.BySpinningLine)+len(got.ByOperator)+len(got.ByInspector) != 0 {
		t.Errorf("空库应返回空结果，得到 %+v", got)
	}
}
