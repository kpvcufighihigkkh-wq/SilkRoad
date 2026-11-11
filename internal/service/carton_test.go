package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// seedDTYLot 造一个 DTY 批次：纸箱只适用于 DTY。
func seedDTYLot(t *testing.T, client *ent.Client, number string) uuid.UUID {
	t.Helper()

	row, err := client.Lot.Create().
		SetLotNumber(number).
		SetProductType("DTY").
		SetPlannedQuantity(100).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed DTY lot: %v", err)
	}
	return row.ID
}

func TestCartonService_CreateCarton_RejectsNonDTYLot(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	// seedLot 造的是 FDY 批次
	lotID := seedLot(t, client, "CTN-LOT-FDY")

	_, err := svc.CreateCarton(context.Background(), &service.CreateCartonRequest{
		LotID: lotID.String(),
	})
	if err == nil {
		t.Fatal("want error：纸箱只适用于 DTY 批次")
	}
	if svcErr, ok := err.(*service.ServiceError); !ok || svcErr.Code != 30001 {
		t.Errorf("want ServiceError(30001), got %#v", err)
	}
}

func TestCartonService_CreateCarton_Defaults(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	lotID := seedDTYLot(t, client, "CTN-LOT-001")

	got, err := svc.CreateCarton(context.Background(), &service.CreateCartonRequest{
		LotID: lotID.String(),
	})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	if got.Status != string(carton.StatusPacking) {
		t.Errorf("Status = %q, want packing", got.Status)
	}
	if got.Capacity != 12 {
		t.Errorf("Capacity = %d, want 12（默认容量）", got.Capacity)
	}
	if got.CartonNumber == "" {
		t.Error("CartonNumber 为空：未生成箱号")
	}
	if got.PalletID != nil {
		t.Errorf("PalletID = %v, want null", got.PalletID)
	}
}

func TestCartonService_PackBobbins_SetsOwnershipAndCount(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-002")
	bobbins := seedPackableBobbins(t, client, lotID, 2)

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	got, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins})
	if err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}
	if got.BobbinCount != 2 {
		t.Errorf("BobbinCount = %d, want 2", got.BobbinCount)
	}

	rows, err := client.Bobbin.Query().Where(bobbin.IDIn(mustUUIDs(t, bobbins)...)).All(ctx)
	if err != nil {
		t.Fatalf("query bobbins: %v", err)
	}
	for _, b := range rows {
		if b.CartonID.String() != c.ID {
			t.Errorf("丝锭 %s 的 CartonID = %s, want %s", b.BobbinNumber, b.CartonID, c.ID)
		}
	}
}

func TestCartonService_PackBobbins_RejectsOverCapacity(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-003")
	bobbins := seedPackableBobbins(t, client, lotID, 3)

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{
		LotID:    lotID.String(),
		Capacity: 2,
	})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins}); err == nil {
		t.Fatal("want error：超出纸箱容量")
	}
}

// 一个丝锭只能有一个上级容器：已码盘的不能装箱。
func TestCartonService_PackBobbins_RejectsPalletizedBobbin(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-004")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	palletSvc := service.NewPalletService(client)
	p, err := palletSvc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := palletSvc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins}); err == nil {
		t.Fatal("want error：已码盘的丝锭不能装箱")
	}
}

func TestCartonService_PackBobbins_IsIdempotent(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-005")
	bobbins := seedPackableBobbins(t, client, lotID, 2)

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	req := &service.PackIntoCartonRequest{BobbinIDs: bobbins}
	if _, err := svc.PackBobbins(ctx, c.ID, req); err != nil {
		t.Fatalf("first pack: %v", err)
	}
	got, err := svc.PackBobbins(ctx, c.ID, req)
	if err != nil {
		t.Fatalf("second pack: %v", err)
	}
	if got.BobbinCount != 2 {
		t.Errorf("BobbinCount = %d, want 2（重复提交不应重复计数）", got.BobbinCount)
	}
}

func TestCartonService_SealCarton_DerivesWeightFromBobbins(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-006")
	bobbins := seedPackableBobbins(t, client, lotID, 2) // 每枚净重 5.0

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	got, err := svc.SealCarton(ctx, c.ID, &service.SealCartonRequest{})
	if err != nil {
		t.Fatalf("SealCarton: %v", err)
	}
	if got.Status != string(carton.StatusPacked) {
		t.Errorf("Status = %q, want packed", got.Status)
	}
	if got.TotalWeight == nil || *got.TotalWeight != 10 {
		t.Errorf("TotalWeight = %v, want 10（两枚 × 5.0 kg）", got.TotalWeight)
	}
	if got.PackedAt == nil {
		t.Error("PackedAt 为空")
	}
}

func TestCartonService_SealCarton_RejectsEmptyCarton(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-007")
	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	if _, err := svc.SealCarton(ctx, c.ID, &service.SealCartonRequest{}); err == nil {
		t.Fatal("want error：空箱不能封箱")
	}
}

func TestCartonService_PackBobbins_RejectsAfterSeal(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-008")
	bobbins := seedPackableBobbins(t, client, lotID, 3)

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins[:1]}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}
	if _, err := svc.SealCarton(ctx, c.ID, &service.SealCartonRequest{}); err != nil {
		t.Fatalf("SealCarton: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins[1:]}); err == nil {
		t.Fatal("want error：封箱后不能再装箱")
	}
}

// 装箱改变丝锭归属，必须重新排队同步。
func TestCartonService_PackBobbins_ResetsBobbinSyncState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-SYNC")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	id := mustUUIDs(t, bobbins)[0]
	if _, err := client.Bobbin.UpdateOneID(id).
		SetSyncStatus(bobbin.SyncStatusSynced).
		SetSyncRetryCount(4).
		Save(ctx); err != nil {
		t.Fatalf("seed synced bobbin: %v", err)
	}

	c, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	row, err := client.Bobbin.Get(ctx, id)
	if err != nil {
		t.Fatalf("reload bobbin: %v", err)
	}
	if row.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want pending", row.SyncStatus)
	}
	if row.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", row.SyncRetryCount)
	}
}

func TestCartonService_ListCartons_FiltersByPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "CTN-LOT-009")
	if _, err := svc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()}); err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}

	// 没有任何纸箱挂在托盘上，按托盘过滤应为 0
	got, total, err := svc.ListCartons(ctx, 1, 20, "", "", uuid.NewString())
	if err != nil {
		t.Fatalf("ListCartons: %v", err)
	}
	if total != 0 || len(got) != 0 {
		t.Errorf("按托盘过滤 total=%d len=%d, want 0/0", total, len(got))
	}

	_, all, err := svc.ListCartons(ctx, 1, 20, lotID.String(), "", "")
	if err != nil {
		t.Fatalf("ListCartons: %v", err)
	}
	if all != 1 {
		t.Errorf("按批次过滤 total=%d, want 1", all)
	}
}
