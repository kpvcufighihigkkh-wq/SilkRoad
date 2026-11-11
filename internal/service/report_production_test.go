package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// lotSeed 描述一条待造的批次。
type lotSeed struct {
	number    string
	product   lot.ProductType
	status    lot.Status
	planned   int
	actual    int
	edgeID    *uuid.UUID
	startTime *time.Time
	createdAt time.Time
}

func seedOverviewLots(t *testing.T, client *ent.Client, seeds []lotSeed) {
	t.Helper()

	for _, s := range seeds {
		b := client.Lot.Create().
			SetLotNumber(s.number).
			SetProductType(s.product).
			SetStatus(s.status).
			SetPlannedQuantity(s.planned).
			SetActualQuantity(s.actual).
			SetCreatedAt(s.createdAt)
		if s.edgeID != nil {
			b.SetEdgeID(*s.edgeID)
		}
		if s.startTime != nil {
			b.SetStartTime(*s.startTime)
		}
		if _, err := b.Save(context.Background()); err != nil {
			t.Fatalf("seed lot %s: %v", s.number, err)
		}
	}
}

// seedEdge 造一台边端设备。lot.edge_id 是外键，引用不存在的设备会被
// SQLite 直接拒绝（FOREIGN KEY constraint failed）。
func seedEdge(t *testing.T, client *ent.Client, code string) uuid.UUID {
	t.Helper()

	row, err := client.Edge.Create().
		SetEdgeCode(code).
		SetEdgeName(code).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed edge %s: %v", code, err)
	}

	return row.ID
}

// 看板一的区间口径：COALESCE(start_time, created_at)。
//
// 两个方向都必须钉住：
//   - start_time 为空但 created_at 在区间内 → 计入（否则历史数据整批消失）；
//   - start_time 有值且在区间外、created_at 在区间内 → 排除（否则一条早于
//     区间的批次会因为它「恰好在这段时间被同步进来」而混进本期产量）。
func TestReportService_ProductionOverview_RangeUsesCOALESCE(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, before, after := reportWindow()

	insidePtr := inside
	beforePtr := before
	afterPtr := after

	seedOverviewLots(t, client, []lotSeed{
		// 无开工时刻，创建时刻在区间内 → 计入
		{number: "L-NO-START", product: lot.ProductTypeFDY, status: lot.StatusInProgress,
			planned: 100, actual: 40, createdAt: inside},
		// 开工时刻早于区间，创建时刻在区间内 → 排除
		{number: "L-EARLY-START", product: lot.ProductTypeFDY, status: lot.StatusCompleted,
			planned: 200, actual: 200, startTime: &beforePtr, createdAt: inside},
		// 开工时刻晚于区间 → 排除
		{number: "L-LATE-START", product: lot.ProductTypePOY, status: lot.StatusInProgress,
			planned: 300, actual: 0, startTime: &afterPtr, createdAt: inside},
		// 开工时刻在区间内、创建时刻在区间外（补录） → 计入
		{number: "L-START-IN", product: lot.ProductTypeDTY, status: lot.StatusCompleted,
			planned: 50, actual: 55, startTime: &insidePtr, createdAt: before},
	})

	got, err := svc.ProductionOverview(context.Background(), start, end)
	if err != nil {
		t.Fatalf("ProductionOverview: %v", err)
	}

	if got.Totals.LotCount != 2 {
		t.Fatalf("totals.lot_count = %d, want 2（只应计入 L-NO-START 与 L-START-IN）", got.Totals.LotCount)
	}
	if got.Totals.PlannedQuantity != 150 {
		t.Errorf("totals.planned_quantity = %d, want 150", got.Totals.PlannedQuantity)
	}
	if got.Totals.ActualQuantity != 95 {
		t.Errorf("totals.actual_quantity = %d, want 95", got.Totals.ActualQuantity)
	}
	// 95/150 = 0.633...
	if got.Totals.CompletionRate != 0.633 {
		t.Errorf("totals.completion_rate = %v, want 0.633", got.Totals.CompletionRate)
	}
}

// 三组分布各自聚合，总数与分组之和必须一致 —— 若某次聚合的谓词写错，
// 分组会静默地只覆盖一部分数据，而看板上的总数仍是对的。
func TestReportService_ProductionOverview_Breakdowns(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	edgeA := seedEdge(t, client, "EDGE-A")
	edgeB := seedEdge(t, client, "EDGE-B")

	seedOverviewLots(t, client, []lotSeed{
		{number: "L1", product: lot.ProductTypeFDY, status: lot.StatusInProgress,
			planned: 100, actual: 30, edgeID: &edgeA, createdAt: inside},
		{number: "L2", product: lot.ProductTypeFDY, status: lot.StatusCompleted,
			planned: 100, actual: 100, edgeID: &edgeA, createdAt: inside},
		{number: "L3", product: lot.ProductTypeDTY, status: lot.StatusCompleted,
			planned: 200, actual: 180, edgeID: &edgeB, createdAt: inside},
		// 无来源设备 → null 组
		{number: "L4", product: lot.ProductTypePOY, status: lot.StatusCancelled,
			planned: 50, actual: 0, createdAt: inside},
	})

	got, err := svc.ProductionOverview(context.Background(), start, end)
	if err != nil {
		t.Fatalf("ProductionOverview: %v", err)
	}

	if got.Totals.LotCount != 4 {
		t.Fatalf("totals.lot_count = %d, want 4", got.Totals.LotCount)
	}

	sumByStatus := 0
	for _, s := range got.ByStatus {
		sumByStatus += s.LotCount
	}
	if sumByStatus != got.Totals.LotCount {
		t.Errorf("by_status 计数之和 = %d, 与 totals.lot_count = %d 不一致", sumByStatus, got.Totals.LotCount)
	}

	// by_status 按计数降序：completed 最多（2），in_progress（1）与 cancelled（1）次之。
	if len(got.ByStatus) != 3 || got.ByStatus[0].Status != string(lot.StatusCompleted) {
		t.Errorf("by_status = %+v, 期望首行为 completed", got.ByStatus)
	}

	if len(got.ByProduct) != 3 {
		t.Errorf("by_product 行数 = %d, want 3", len(got.ByProduct))
	}
	for _, p := range got.ByProduct {
		if p.ProductType == string(lot.ProductTypeDTY) {
			if p.PlannedQuantity != 200 || p.ActualQuantity != 180 {
				t.Errorf("DTY 汇总 = %d/%d, want 180/200", p.ActualQuantity, p.PlannedQuantity)
			}
		}
	}

	if len(got.ByEdge) != 3 {
		t.Fatalf("by_edge 行数 = %d, want 3（edgeA、edgeB、null）", len(got.ByEdge))
	}
	var nullEdge *service.EdgeLotStat
	for i := range got.ByEdge {
		if got.ByEdge[i].EdgeID == nil {
			nullEdge = &got.ByEdge[i]
		}
	}
	if nullEdge == nil {
		t.Fatalf("by_edge 缺少 null 组（无来源设备的批次）: %+v", got.ByEdge)
	}
	if nullEdge.LotCount != 1 {
		t.Errorf("null 组 lot_count = %d, want 1", nullEdge.LotCount)
	}
	if nullEdge.EdgeID != nil {
		t.Errorf("null 组 edge_id 应为 null, got %q", *nullEdge.EdgeID)
	}
}

// 区间内没有批次时，响应必须是空数组而不是 null ——
// 前端对 null 调 .map 会直接白屏。
func TestReportService_ProductionOverview_EmptyRange(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, _, _, _ := reportWindow()

	got, err := svc.ProductionOverview(context.Background(), start, end)
	if err != nil {
		t.Fatalf("ProductionOverview: %v", err)
	}

	if got.Totals.LotCount != 0 || got.Totals.CompletionRate != 0 {
		t.Errorf("空区间 totals = %+v, want 零值", got.Totals)
	}
	if got.ByStatus == nil || got.ByProduct == nil || got.ByEdge == nil {
		t.Errorf("空区间下分组应为空切片而非 nil: %+v", got)
	}
}
