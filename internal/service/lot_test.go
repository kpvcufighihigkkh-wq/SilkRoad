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

// 批次由 Edge 产生，创建即投产：start_time 必须在 CreateLot 时写定。
//
// 这一列是看板一的区间过滤字段，此前全仓库无人写它，导致看板无法交付。
// 本用例钉住「创建时非零」，以及落库后确实落到了列上（而不是只出现在响应里）。
func TestLotService_CreateLot_WritesStartTime(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewLotService(client)

	resp, err := svc.CreateLot(context.Background(), &service.CreateLotRequest{
		LotNumber:       "EDGE-2026-001",
		ProductType:     "FDY",
		PlannedQuantity: 500,
	})
	if err != nil {
		t.Fatalf("CreateLot: %v", err)
	}

	if resp.Status != string(lot.StatusInProgress) {
		t.Errorf("status = %q, want in_progress", resp.Status)
	}
	if resp.StartTime == nil {
		t.Fatal("响应缺少 start_time")
	}

	row, err := client.Lot.Get(context.Background(), uuid.MustParse(resp.ID))
	if err != nil {
		t.Fatalf("重新读取批次: %v", err)
	}
	if row.StartTime.IsZero() {
		t.Fatal("lot.start_time 落库后仍为零值")
	}
	if !row.EndTime.IsZero() {
		t.Errorf("新建批次的 end_time 应为空，got %v", row.EndTime)
	}
}

// 重复批次号必须被拒绝：lot_number 有唯一约束，靠数据库报错再转成 40002
// 会把约束错误泄漏成 500，前端无法提示「批次号已存在」。
func TestLotService_CreateLot_RejectsDuplicateNumber(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewLotService(client)
	req := &service.CreateLotRequest{
		LotNumber:       "EDGE-2026-DUP",
		ProductType:     "POY",
		PlannedQuantity: 100,
	}

	if _, err := svc.CreateLot(context.Background(), req); err != nil {
		t.Fatalf("首次 CreateLot: %v", err)
	}

	_, err := svc.CreateLot(context.Background(), req)
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		t.Fatalf("重复批次号应返回 ServiceError, got %T: %v", err, err)
	}
	if svcErr.Code != 40002 {
		t.Errorf("error code = %d, want 40002（冲突）", svcErr.Code)
	}
}

// 状态流转写 end_time，且是幂等的：重复提交 completed 不把结束时刻推后。
func TestLotService_UpdateLotStatus_WritesEndTimeOnce(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewLotService(client)

	created, err := svc.CreateLot(context.Background(), &service.CreateLotRequest{
		LotNumber:       "EDGE-2026-002",
		ProductType:     "DTY",
		PlannedQuantity: 100,
	})
	if err != nil {
		t.Fatalf("CreateLot: %v", err)
	}

	actual := 98
	completed, err := svc.UpdateLotStatus(context.Background(), created.ID, &service.UpdateLotStatusRequest{
		Status:         "completed",
		ActualQuantity: &actual,
	})
	if err != nil {
		t.Fatalf("UpdateLotStatus(completed): %v", err)
	}
	if completed.EndTime == nil {
		t.Fatal("完工后缺少 end_time")
	}
	if completed.ActualQuantity != 98 {
		t.Errorf("actual_quantity = %d, want 98", completed.ActualQuantity)
	}
	firstEnd := *completed.EndTime

	// 隔一小段时间重复提交：end_time 必须保持第一次的值。
	time.Sleep(10 * time.Millisecond)
	again, err := svc.UpdateLotStatus(context.Background(), created.ID, &service.UpdateLotStatusRequest{
		Status: "completed",
	})
	if err != nil {
		t.Fatalf("UpdateLotStatus(重复 completed): %v", err)
	}
	if again.EndTime == nil || *again.EndTime != firstEnd {
		t.Errorf("重复提交改写了 end_time: 首次 %s, 再次 %v", firstEnd, again.EndTime)
	}
}

// 非法状态直接拒绝，且不改动任何列。
func TestLotService_UpdateLotStatus_RejectsUnknownStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewLotService(client)

	created, err := svc.CreateLot(context.Background(), &service.CreateLotRequest{
		LotNumber:       "EDGE-2026-003",
		ProductType:     "FDY",
		PlannedQuantity: 10,
	})
	if err != nil {
		t.Fatalf("CreateLot: %v", err)
	}

	_, err = svc.UpdateLotStatus(context.Background(), created.ID, &service.UpdateLotStatusRequest{
		Status: "flying",
	})
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		t.Fatalf("非法状态应返回 ServiceError, got %T: %v", err, err)
	}
	if svcErr.Code != 10001 {
		t.Errorf("error code = %d, want 10001（参数错误）", svcErr.Code)
	}
}
