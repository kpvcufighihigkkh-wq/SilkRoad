package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/pallet"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// seedPackedBobbins 在指定批次下造 n 个可码盘的丝锭。
func seedPackableBobbins(t *testing.T, client *ent.Client, lotID uuid.UUID, n int) []string {
	t.Helper()

	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		row, err := client.Bobbin.Create().
			SetBobbinNumber("PACK-" + uuid.NewString()[:8]).
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

func TestPalletService_CreatePallet_GeneratesCodeAndBuildingStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	lotID := seedLot(t, client, "PAL-LOT-001")

	got, err := svc.CreatePallet(context.Background(), &service.CreatePalletRequest{
		LotID: lotID.String(),
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if got.Status != string(pallet.StatusBuilding) {
		t.Errorf("Status = %q, want building", got.Status)
	}
	if got.PalletCode == "" {
		t.Error("PalletCode 为空：未生成托盘号")
	}
	if got.BobbinsCount != 0 {
		t.Errorf("BobbinsCount = %d, want 0", got.BobbinsCount)
	}
	// 未称重时三层重量必须是 null 而不是 0
	if got.NetWeight != nil || got.GrossWeight != nil || got.TareWeight != nil {
		t.Errorf("未封托时重量应为 null，得到 net=%v gross=%v tare=%v",
			got.NetWeight, got.GrossWeight, got.TareWeight)
	}
}

func TestPalletService_CreatePallet_RejectsUnknownLot(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)

	_, err := svc.CreatePallet(context.Background(), &service.CreatePalletRequest{
		LotID: uuid.NewString(),
	})
	if err == nil {
		t.Fatal("want error for unknown lot")
	}
	if svcErr, ok := err.(*service.ServiceError); !ok || svcErr.Code != 40001 {
		t.Errorf("want ServiceError(40001), got %#v", err)
	}
}

func TestPalletService_PackBobbins_SetsOwnershipAndCount(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-002")
	bobbins := seedPackableBobbins(t, client, lotID, 3)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	got, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins})
	if err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}
	if got.BobbinsCount != 3 {
		t.Errorf("BobbinsCount = %d, want 3", got.BobbinsCount)
	}

	// 丝锭必须真的挂到托盘上，且状态推进到 packed
	rows, err := client.Bobbin.Query().Where(bobbin.IDIn(mustUUIDs(t, bobbins)...)).All(ctx)
	if err != nil {
		t.Fatalf("query bobbins: %v", err)
	}
	for _, b := range rows {
		if b.PalletID.String() != p.ID {
			t.Errorf("丝锭 %s 的 PalletID = %s, want %s", b.BobbinNumber, b.PalletID, p.ID)
		}
		if b.Status != bobbin.StatusPacked {
			t.Errorf("丝锭 %s 状态 = %s, want packed", b.BobbinNumber, b.Status)
		}
	}
}

// 重复提交同一批丝锭不得重复计数。
func TestPalletService_PackBobbins_IsIdempotent(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-003")
	bobbins := seedPackableBobbins(t, client, lotID, 2)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("first pack: %v", err)
	}
	got, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins})
	if err != nil {
		t.Fatalf("second pack: %v", err)
	}

	if got.BobbinsCount != 2 {
		t.Errorf("BobbinsCount = %d, want 2（重复提交不应重复计数）", got.BobbinsCount)
	}
}

// 丝锭已码在别的托盘上时必须拒绝：否则原托盘的计数不会回退。
func TestPalletService_PackBobbins_RejectsBobbinOwnedByAnotherPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-004")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	first, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, first.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("pack to first: %v", err)
	}

	second, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	_, err = svc.PackBobbins(ctx, second.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins})
	if err == nil {
		t.Fatal("want error：丝锭已属于其它托盘")
	}
}

// 丝锭与托盘必须同批次。
func TestPalletService_PackBobbins_RejectsCrossLotBobbin(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "PAL-LOT-A")
	lotB := seedLot(t, client, "PAL-LOT-B")
	foreign := seedPackableBobbins(t, client, lotB, 1)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotA.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: foreign}); err == nil {
		t.Fatal("want error：丝锭与托盘不属于同一批次")
	}
}

func TestPalletService_SealPallet_ComputesNetAndCompletes(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-005")
	bobbins := seedPackableBobbins(t, client, lotID, 2)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	got, err := svc.SealPallet(ctx, p.ID, &service.SealPalletRequest{
		GrossWeight: 208.5,
		TareWeight:  18.5,
	})
	if err != nil {
		t.Fatalf("SealPallet: %v", err)
	}

	if got.Status != string(pallet.StatusCompleted) {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.NetWeight == nil || *got.NetWeight != 190 {
		t.Errorf("NetWeight = %v, want 190（毛重 - 皮重）", got.NetWeight)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt 为空")
	}
}

// 空托盘封托没有意义：账实不符。
func TestPalletService_SealPallet_RejectsEmptyPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-006")
	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.SealPallet(ctx, p.ID, &service.SealPalletRequest{
		GrossWeight: 20,
		TareWeight:  18,
	}); err == nil {
		t.Fatal("want error：空托盘不能封托")
	}
}

// 封托后不得再码垛：重量已称定，追加丝锭会让账实不符。
func TestPalletService_PackBobbins_RejectsAfterSeal(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-007")
	bobbins := seedPackableBobbins(t, client, lotID, 3)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins[:1]}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}
	if _, err := svc.SealPallet(ctx, p.ID, &service.SealPalletRequest{GrossWeight: 20, TareWeight: 18}); err != nil {
		t.Fatalf("SealPallet: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins[1:]}); err == nil {
		t.Fatal("want error：封托后不能再码垛")
	}
}

func TestPalletService_ListPallets_FiltersByLotAndStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "PAL-LOT-008")
	lotB := seedLot(t, client, "PAL-LOT-009")

	if _, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotA.String()}); err != nil {
		t.Fatalf("CreatePallet A: %v", err)
	}
	if _, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotB.String()}); err != nil {
		t.Fatalf("CreatePallet B: %v", err)
	}

	got, total, err := svc.ListPallets(ctx, 1, 20, lotA.String(), "")
	if err != nil {
		t.Fatalf("ListPallets: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Errorf("按批次过滤 total=%d len=%d, want 1/1", total, len(got))
	}

	// 全部都是 building，按 completed 筛应为 0
	_, completed, err := svc.ListPallets(ctx, 1, 20, "", string(pallet.StatusCompleted))
	if err != nil {
		t.Fatalf("ListPallets: %v", err)
	}
	if completed != 0 {
		t.Errorf("status=completed total=%d, want 0", completed)
	}
}

func mustUUIDs(t *testing.T, raws []string) []uuid.UUID {
	t.Helper()

	out := make([]uuid.UUID, 0, len(raws))
	for _, r := range raws {
		id, err := uuid.Parse(r)
		if err != nil {
			t.Fatalf("invalid uuid %q: %v", r, err)
		}
		out = append(out, id)
	}
	return out
}

// 码盘必须让丝锭重新排队同步：Center 靠载荷里的 pallet_code 才能挂上托盘。
// 已 synced 的丝锭若不动同步状态，采集器只抓 pending，关联永远到不了 Center。
func TestPalletService_PackBobbins_ResetsBobbinSyncState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-SYNC")
	bobbinIDs := seedPackableBobbins(t, client, lotID, 1)

	// 模拟「丝锭早已同步过一次」
	id := mustUUIDs(t, bobbinIDs)[0]
	if err := client.Bobbin.UpdateOneID(id).
		SetSyncStatus(bobbin.SyncStatusSynced).
		SetSyncRetryCount(3).
		SetSyncedAt(time.Now()).
		Exec(ctx); err != nil {
		t.Fatalf("seed synced bobbin: %v", err)
	}

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbinIDs}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	row, err := client.Bobbin.Get(ctx, id)
	if err != nil {
		t.Fatalf("reload bobbin: %v", err)
	}
	if row.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want pending（码盘后必须重新排队）", row.SyncStatus)
	}
	if row.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0（重试预算耗尽的行会一直被采集器过滤掉）", row.SyncRetryCount)
	}
	if !row.SyncedAt.IsZero() {
		t.Errorf("SyncedAt = %v, want 零值（已排队的行不应带上次同步时间）", row.SyncedAt)
	}
}

// 封托同理：托盘已同步过再封托，Center 否则永远停在 building、三层重量为空。
func TestPalletService_SealPallet_ResetsPalletSyncState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "PAL-LOT-SYNC-SEAL")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}

	// 模拟「托盘已同步过一次（building 状态）」
	pid := mustUUIDs(t, []string{p.ID})[0]
	if err := client.Pallet.UpdateOneID(pid).
		SetSyncStatus(pallet.SyncStatusSynced).
		SetSyncRetryCount(2).
		SetSyncedAt(time.Now()).
		Exec(ctx); err != nil {
		t.Fatalf("seed synced pallet: %v", err)
	}

	if _, err := svc.SealPallet(ctx, p.ID, &service.SealPalletRequest{GrossWeight: 30, TareWeight: 5}); err != nil {
		t.Fatalf("SealPallet: %v", err)
	}

	row, err := client.Pallet.Get(ctx, pid)
	if err != nil {
		t.Fatalf("reload pallet: %v", err)
	}
	if row.SyncStatus != pallet.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want pending（封托后必须重新排队）", row.SyncStatus)
	}
	if row.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", row.SyncRetryCount)
	}
	if !row.SyncedAt.IsZero() {
		t.Errorf("SyncedAt = %v, want 零值", row.SyncedAt)
	}
}

// DTY 托盘只能码箱，直接码锭的接口必须拒绝它。
func TestPalletService_PackBobbins_RejectsCartonPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-001")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.PackBobbins(ctx, p.ID, &service.PackBobbinsRequest{
		BobbinIDs: bobbins,
	}); err == nil {
		t.Fatal("want error：DTY 码箱托盘不能码丝锭")
	}
}

// 码锭托盘不能码箱。
func TestPalletService_PackCartons_RejectsBobbinPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-002")
	cartons := seedSealedCartons(t, client, lotID, 1)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.PackCartons(ctx, p.ID, &service.PackCartonsRequest{
		CartonIDs: cartons,
	}); err == nil {
		t.Fatal("want error：直接码锭托盘不能码纸箱")
	}
}

func TestPalletService_PackCartons_SetsPalletAndCount(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-003")
	cartons := seedSealedCartons(t, client, lotID, 3)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	got, err := svc.PackCartons(ctx, p.ID, &service.PackCartonsRequest{CartonIDs: cartons})
	if err != nil {
		t.Fatalf("PackCartons: %v", err)
	}
	if got.CartonsCount != 3 {
		t.Errorf("CartonsCount = %d, want 3", got.CartonsCount)
	}
	if got.BobbinsCount != 0 {
		t.Errorf("BobbinsCount = %d, want 0（码箱托盘不装丝锭）", got.BobbinsCount)
	}

	rows, err := client.Carton.Query().Where(carton.IDIn(mustUUIDs(t, cartons)...)).All(ctx)
	if err != nil {
		t.Fatalf("query cartons: %v", err)
	}
	for _, c := range rows {
		if c.PalletID.String() != p.ID {
			t.Errorf("纸箱 %s 的 PalletID = %s, want %s", c.CartonNumber, c.PalletID, p.ID)
		}
	}
}

// 未封箱的纸箱不能码垛：箱里还能装，挂在托盘上会让账实不符。
func TestPalletService_PackCartons_RejectsUnsealedCarton(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	cartonSvc := service.NewCartonService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-004")
	bobbins := seedPackableBobbins(t, client, lotID, 1)

	c, err := cartonSvc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
	if err != nil {
		t.Fatalf("CreateCarton: %v", err)
	}
	if _, err := cartonSvc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{BobbinIDs: bobbins}); err != nil {
		t.Fatalf("PackBobbins: %v", err)
	}
	// 刻意不封箱

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	if _, err := svc.PackCartons(ctx, p.ID, &service.PackCartonsRequest{
		CartonIDs: []string{c.ID},
	}); err == nil {
		t.Fatal("want error：未封箱的纸箱不能码垛")
	}
}

func TestPalletService_PackCartons_RejectsCartonOnAnotherPallet(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-005")
	cartons := seedSealedCartons(t, client, lotID, 1)

	first, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackCartons(ctx, first.ID, &service.PackCartonsRequest{CartonIDs: cartons}); err != nil {
		t.Fatalf("first pack: %v", err)
	}

	second, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackCartons(ctx, second.ID, &service.PackCartonsRequest{CartonIDs: cartons}); err == nil {
		t.Fatal("want error：纸箱已码在其它托盘上")
	}
}

func TestPalletService_PackCartons_IsIdempotent(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-006")
	cartons := seedSealedCartons(t, client, lotID, 2)

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}

	req := &service.PackCartonsRequest{CartonIDs: cartons}
	if _, err := svc.PackCartons(ctx, p.ID, req); err != nil {
		t.Fatalf("first pack: %v", err)
	}
	got, err := svc.PackCartons(ctx, p.ID, req)
	if err != nil {
		t.Fatalf("second pack: %v", err)
	}
	if got.CartonsCount != 2 {
		t.Errorf("CartonsCount = %d, want 2（重复提交不应重复计数）", got.CartonsCount)
	}
}

// 码箱改变纸箱归属，必须重新排队同步。
func TestPalletService_PackCartons_ResetsCartonSyncState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	ctx := context.Background()

	lotID := seedDTYLot(t, client, "PAL-CARTON-007")
	cartons := seedSealedCartons(t, client, lotID, 1)

	id := mustUUIDs(t, cartons)[0]
	if _, err := client.Carton.UpdateOneID(id).
		SetSyncStatus(carton.SyncStatusSynced).
		SetSyncRetryCount(3).
		Save(ctx); err != nil {
		t.Fatalf("seed synced carton: %v", err)
	}

	p, err := svc.CreatePallet(ctx, &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "carton",
	})
	if err != nil {
		t.Fatalf("CreatePallet: %v", err)
	}
	if _, err := svc.PackCartons(ctx, p.ID, &service.PackCartonsRequest{CartonIDs: cartons}); err != nil {
		t.Fatalf("PackCartons: %v", err)
	}

	row, err := client.Carton.Get(ctx, id)
	if err != nil {
		t.Fatalf("reload carton: %v", err)
	}
	if row.SyncStatus != carton.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want pending（码垛后必须重新排队）", row.SyncStatus)
	}
	if row.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", row.SyncRetryCount)
	}
}

func TestPalletService_CreatePallet_RejectsInvalidType(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewPalletService(client)
	lotID := seedLot(t, client, "PAL-TYPE-001")

	if _, err := svc.CreatePallet(context.Background(), &service.CreatePalletRequest{
		LotID:      lotID.String(),
		PalletType: "barrel",
	}); err == nil {
		t.Fatal("want error：无效的托盘类型")
	}
}

// seedSealedCartons 在指定 DTY 批次下造 n 个已封箱的纸箱。
func seedSealedCartons(t *testing.T, client *ent.Client, lotID uuid.UUID, n int) []string {
	t.Helper()

	ctx := context.Background()
	cartonSvc := service.NewCartonService(client)
	bobbinSvc := seedPackableBobbins(t, client, lotID, n)

	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		c, err := cartonSvc.CreateCarton(ctx, &service.CreateCartonRequest{LotID: lotID.String()})
		if err != nil {
			t.Fatalf("seed carton #%d: %v", i, err)
		}
		if _, err := cartonSvc.PackBobbins(ctx, c.ID, &service.PackIntoCartonRequest{
			BobbinIDs: []string{bobbinSvc[i]},
		}); err != nil {
			t.Fatalf("pack carton #%d: %v", i, err)
		}
		if _, err := cartonSvc.SealCarton(ctx, c.ID, &service.SealCartonRequest{}); err != nil {
			t.Fatalf("seal carton #%d: %v", i, err)
		}
		out = append(out, c.ID)
	}

	return out
}
