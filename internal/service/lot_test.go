package service_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
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
