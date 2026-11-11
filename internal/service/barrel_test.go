package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/service"
)

func seedBarrelBobbins(t *testing.T, client *ent.Client, lotID uuid.UUID, n int) []string {
	t.Helper()

	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		row, err := client.Bobbin.Create().
			SetBobbinNumber("BRL-" + uuid.NewString()[:8]).
			SetLotID(lotID).
			SetSpinningPosition(i + 1).
			SetGrossWeight(5.4).
			SetNetWeight(5.0).
			SetStatus(bobbin.StatusCompleted).
			Save(context.Background())
		if err != nil {
			t.Fatalf("seed bobbin #%d: %v", i, err)
		}
		out = append(out, row.ID.String())
	}

	return out
}

func TestBarrelService_CreateBarrel_DefaultsCapacityAndActiveStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	lotID := seedLot(t, client, "BRL-LOT-001")

	got, err := svc.CreateBarrel(context.Background(), &service.CreateBarrelRequest{
		LotID: lotID.String(),
	})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	if got.Status != string(barrel.StatusActive) {
		t.Errorf("Status = %q, want active", got.Status)
	}
	if got.Capacity != 9 {
		t.Errorf("Capacity = %d, want 9（默认容量）", got.Capacity)
	}
	if got.CurrentCount != 0 {
		t.Errorf("CurrentCount = %d, want 0", got.CurrentCount)
	}
	if got.BarrelNumber == "" {
		t.Error("BarrelNumber 为空：未生成桶号")
	}
	if got.FilledAt != nil {
		t.Errorf("FilledAt = %v, want null（未装满）", got.FilledAt)
	}
}

func TestBarrelService_CreateBarrel_RejectsUnknownLot(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)

	_, err := svc.CreateBarrel(context.Background(), &service.CreateBarrelRequest{
		LotID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("want error for unknown lot")
	}
	if svcErr, ok := err.(*service.ServiceError); !ok || svcErr.Code != 40001 {
		t.Errorf("want ServiceError(40001), got %#v", err)
	}
}

func TestBarrelService_LoadBobbins_SetsPositionsAndCount(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-002")
	bobbins := seedBarrelBobbins(t, client, lotID, 3)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	got, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("LoadBobbins: %v", err)
	}
	if got.CurrentCount != 3 {
		t.Errorf("CurrentCount = %d, want 3", got.CurrentCount)
	}

	rows, err := client.Bobbin.Query().Where(bobbin.IDIn(mustUUIDs(t, bobbins)...)).All(ctx)
	if err != nil {
		t.Fatalf("query bobbins: %v", err)
	}
	for i, b := range rows {
		if b.BarrelID.String() != got.ID {
			t.Errorf("丝锭 %s 的 BarrelID = %s, want %s", b.BobbinNumber, b.BarrelID, got.ID)
		}
		// 位置按请求顺序落库：rows 的顺序与 IDIn 入参一致。
		if b.BarrelPosition != i+1 {
			t.Errorf("丝锭 %s 的 BarrelPosition = %d, want %d", b.BobbinNumber, b.BarrelPosition, i+1)
		}
	}
}

// 装满即自动转 full 并写 filled_at：这是客观事实，不需要操作员再确认一次。
func TestBarrelService_LoadBobbins_AutofillsWhenFull(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-003")
	bobbins := seedBarrelBobbins(t, client, lotID, 2)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{
		LotID:    lotID.String(),
		Capacity: 2,
	})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	got, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{1, 2},
	})
	if err != nil {
		t.Fatalf("LoadBobbins: %v", err)
	}

	if got.Status != string(barrel.StatusFull) {
		t.Errorf("Status = %q, want full（装到容量上限）", got.Status)
	}
	if got.FilledAt == nil {
		t.Error("FilledAt 为空：装满时必须记录时间")
	}
}

// 物理约束：一个锭位只能放一枚丝锭。
func TestBarrelService_LoadBobbins_RejectsOccupiedPosition(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-004")
	bobbins := seedBarrelBobbins(t, client, lotID, 2)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}
	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins[:1],
		SpinPositions: []int{1},
	}); err != nil {
		t.Fatalf("first load: %v", err)
	}

	_, err = svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins[1:],
		SpinPositions: []int{1},
	})
	if err == nil {
		t.Fatal("want error：锭位 1 已被占用")
	}
}

func TestBarrelService_LoadBobbins_RejectsPositionBeyondCapacity(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-005")
	bobbins := seedBarrelBobbins(t, client, lotID, 1)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{
		LotID:    lotID.String(),
		Capacity: 3,
	})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{4},
	}); err == nil {
		t.Fatal("want error：锭位超出容量")
	}
}

// 一个丝锭只能有一个上级容器：已码盘的不能装桶。
func TestBarrelService_LoadBobbins_RejectsPalletizedBobbin(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-006")
	bobbins := seedBarrelBobbins(t, client, lotID, 1)

	palletSvc := service.NewPalletService(client)
	p, err := palletSvc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := palletSvc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{
		BobbinIDs: bobbins,
	}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}
	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{1},
	}); err == nil {
		t.Fatal("want error：已码盘的丝锭不能装桶")
	}
}

func TestBarrelService_LoadBobbins_RejectsCrossLotBobbin(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "BRL-LOT-A")
	lotB := seedLot(t, client, "BRL-LOT-B")
	foreign := seedBarrelBobbins(t, client, lotB, 1)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotA.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}
	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     foreign,
		SpinPositions: []int{1},
	}); err == nil {
		t.Fatal("want error：丝锭与桶不属于同一批次")
	}
}

func TestBarrelService_LoadBobbins_IsIdempotent(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-007")
	bobbins := seedBarrelBobbins(t, client, lotID, 2)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	req := &service.LoadBobbinsRequest{BobbinIDs: bobbins, SpinPositions: []int{1, 2}}
	if _, err := svc.LoadBobbins(ctx, b.ID, req); err != nil {
		t.Fatalf("first load: %v", err)
	}
	got, err := svc.LoadBobbins(ctx, b.ID, req)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if got.CurrentCount != 2 {
		t.Errorf("CurrentCount = %d, want 2（重复提交不应重复计数）", got.CurrentCount)
	}
}

// 装锭会改变丝锭归属，必须重新排队同步，否则 Center 永远不知道它在哪个桶。
func TestBarrelService_LoadBobbins_ResetsBobbinSyncState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-SYNC")
	bobbins := seedBarrelBobbins(t, client, lotID, 1)

	id := mustUUIDs(t, bobbins)[0]
	if _, err := client.Bobbin.UpdateOneID(id).
		SetSyncStatus(bobbin.SyncStatusSynced).
		SetSyncRetryCount(3).
		Save(ctx); err != nil {
		t.Fatalf("seed synced bobbin: %v", err)
	}

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}
	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{1},
	}); err != nil {
		t.Fatalf("LoadBobbins: %v", err)
	}

	row, err := client.Bobbin.Get(ctx, id)
	if err != nil {
		t.Fatalf("reload bobbin: %v", err)
	}
	if row.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want pending（装桶后必须重新排队）", row.SyncStatus)
	}
	if row.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", row.SyncRetryCount)
	}
}

func TestBarrelService_SealBarrel_RejectsEmptyBarrel(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-008")
	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}

	if _, err := svc.SealBarrel(ctx, b.ID); err == nil {
		t.Fatal("want error：空桶不能封桶")
	}
}

func TestBarrelService_SealBarrel_PacksAndIsIdempotent(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "BRL-LOT-009")
	bobbins := seedBarrelBobbins(t, client, lotID, 1)

	b, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateBarrel: %v", err)
	}
	if _, err := svc.LoadBobbins(ctx, b.ID, &service.LoadBobbinsRequest{
		BobbinIDs:     bobbins,
		SpinPositions: []int{1},
	}); err != nil {
		t.Fatalf("LoadBobbins: %v", err)
	}

	got, err := svc.SealBarrel(ctx, b.ID)
	if err != nil {
		t.Fatalf("SealBarrel: %v", err)
	}
	if got.Status != string(barrel.StatusPacked) {
		t.Errorf("Status = %q, want packed", got.Status)
	}

	// 重复封桶是幂等的，不报错
	if _, err := svc.SealBarrel(ctx, b.ID); err != nil {
		t.Errorf("重复封桶应幂等，得到错误：%v", err)
	}
}

func TestBarrelService_ListBarrels_FiltersByLotAndStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewBarrelService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "BRL-LOT-010")
	lotB := seedLot(t, client, "BRL-LOT-011")

	if _, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotA.String()}); err != nil {
		t.Fatalf("CreateBarrel A: %v", err)
	}
	if _, err := svc.CreateBarrel(ctx, &service.CreateBarrelRequest{LotID: lotB.String()}); err != nil {
		t.Fatalf("CreateBarrel B: %v", err)
	}

	got, total, err := svc.ListBarrels(ctx, 1, 20, lotA.String(), "")
	if err != nil {
		t.Fatalf("ListBarrels: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Errorf("按批次过滤 total=%d len=%d, want 1/1", total, len(got))
	}

	_, full, err := svc.ListBarrels(ctx, 1, 20, "", string(barrel.StatusFull))
	if err != nil {
		t.Fatalf("ListBarrels: %v", err)
	}
	if full != 0 {
		t.Errorf("status=full total=%d, want 0", full)
	}
}
