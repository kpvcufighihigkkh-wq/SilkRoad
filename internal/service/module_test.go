package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/module"
	"github.com/yourusername/igh-silkroad/internal/service"
)

func seedBarrel(t *testing.T, client *ent.Client, lotID uuid.UUID, number string) string {
	t.Helper()

	row, err := client.Barrel.Create().
		SetBarrelNumber(number).
		SetLotID(lotID).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed barrel %s: %v", number, err)
	}
	return row.ID.String()
}

func TestModuleService_CreateModule_DefaultsToIdle(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)

	got, err := svc.CreateModule(context.Background(), &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	if got.Status != string(module.StatusIdle) {
		t.Errorf("Status = %q, want idle", got.Status)
	}
	if got.ModuleNumber == "" {
		t.Error("ModuleNumber 为空：未生成吊车号")
	}
	if got.Barrel1ID != nil || got.Barrel2ID != nil {
		t.Errorf("初始挂位应为 null，得到 %v / %v", got.Barrel1ID, got.Barrel2ID)
	}
}

func TestModuleService_CreateModule_RejectsDuplicateNumber(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	if _, err := svc.CreateModule(ctx, &service.CreateModuleRequest{ModuleNumber: "MDL-DUP"}); err != nil {
		t.Fatalf("first CreateModule: %v", err)
	}
	if _, err := svc.CreateModule(ctx, &service.CreateModuleRequest{ModuleNumber: "MDL-DUP"}); err == nil {
		t.Fatal("want error：吊车编号重复")
	}
}

func TestModuleService_LoadBarrels_SetsBothSlots(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "MDL-LOT-001")
	b1 := seedBarrel(t, client, lotID, "BRL-MDL-1")
	b2 := seedBarrel(t, client, lotID, "BRL-MDL-2")

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	got, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{BarrelIDs: []string{b1, b2}})
	if err != nil {
		t.Fatalf("LoadBarrels: %v", err)
	}
	if got.Barrel1ID == nil || *got.Barrel1ID != b1 {
		t.Errorf("Barrel1ID = %v, want %s", got.Barrel1ID, b1)
	}
	if got.Barrel2ID == nil || *got.Barrel2ID != b2 {
		t.Errorf("Barrel2ID = %v, want %s", got.Barrel2ID, b2)
	}
	if got.Status != string(module.StatusLoading) {
		t.Errorf("Status = %q, want loading", got.Status)
	}
}

// 只装一个桶时，第二个挂位必须清空 —— 否则残留的旧桶会被当成还在车上。
func TestModuleService_LoadBarrels_ClearsSecondSlot(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "MDL-LOT-002")
	b1 := seedBarrel(t, client, lotID, "BRL-MDL-3")
	b2 := seedBarrel(t, client, lotID, "BRL-MDL-4")

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}
	if _, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{BarrelIDs: []string{b1, b2}}); err != nil {
		t.Fatalf("first load: %v", err)
	}

	got, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{BarrelIDs: []string{b1}})
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if got.Barrel2ID != nil {
		t.Errorf("Barrel2ID = %v, want null（只装一个桶时第二挂位应清空）", got.Barrel2ID)
	}
}

func TestModuleService_LoadBarrels_RejectsSameBarrelTwice(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "MDL-LOT-003")
	b1 := seedBarrel(t, client, lotID, "BRL-MDL-5")

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	if _, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{
		BarrelIDs: []string{b1, b1},
	}); err == nil {
		t.Fatal("want error：两个挂位不能是同一个桶")
	}
}

func TestModuleService_LoadBarrels_RejectsMoreThanTwo(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "MDL-LOT-004")
	b1 := seedBarrel(t, client, lotID, "BRL-MDL-6")
	b2 := seedBarrel(t, client, lotID, "BRL-MDL-7")
	b3 := seedBarrel(t, client, lotID, "BRL-MDL-8")

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	if _, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{
		BarrelIDs: []string{b1, b2, b3},
	}); err == nil {
		t.Fatal("want error：吊车只有两个挂位")
	}
}

// 一个桶同时只能在一台吊车上。
func TestModuleService_LoadBarrels_RejectsBarrelOnAnotherModule(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "MDL-LOT-005")
	b1 := seedBarrel(t, client, lotID, "BRL-MDL-9")

	first, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule first: %v", err)
	}
	if _, err := svc.LoadBarrels(ctx, first.ID, &service.LoadBarrelsRequest{BarrelIDs: []string{b1}}); err != nil {
		t.Fatalf("load to first: %v", err)
	}

	second, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule second: %v", err)
	}
	if _, err := svc.LoadBarrels(ctx, second.ID, &service.LoadBarrelsRequest{BarrelIDs: []string{b1}}); err == nil {
		t.Fatal("want error：桶已挂在其它吊车上")
	}
}

func TestModuleService_LoadBarrels_RejectsUnknownBarrel(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	if _, err := svc.LoadBarrels(ctx, m.ID, &service.LoadBarrelsRequest{
		BarrelIDs: []string{uuid.NewString()},
	}); err == nil {
		t.Fatal("want error：桶不存在")
	}
}

func TestModuleService_UpdateStatus_PersistsStatusAndLocation(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	got, err := svc.UpdateStatus(ctx, m.ID, &service.UpdateModuleStatusRequest{
		Status:          "transporting",
		CurrentLocation: "A 区 3 号货道",
	})
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if got.Status != string(module.StatusTransporting) {
		t.Errorf("Status = %q, want transporting", got.Status)
	}
	if got.CurrentLocation != "A 区 3 号货道" {
		t.Errorf("CurrentLocation = %q", got.CurrentLocation)
	}
}

func TestModuleService_UpdateStatus_RejectsUnknownStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	m, err := svc.CreateModule(ctx, &service.CreateModuleRequest{})
	if err != nil {
		t.Fatalf("CreateModule: %v", err)
	}

	if _, err := svc.UpdateStatus(ctx, m.ID, &service.UpdateModuleStatusRequest{
		Status: "flying",
	}); err == nil {
		t.Fatal("want error：未知吊车状态")
	}
}

func TestModuleService_ListModules_FiltersByStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewModuleService(client)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := svc.CreateModule(ctx, &service.CreateModuleRequest{}); err != nil {
			t.Fatalf("CreateModule: %v", err)
		}
	}

	_, idle, err := svc.ListModules(ctx, 1, 20, string(module.StatusIdle))
	if err != nil {
		t.Fatalf("ListModules: %v", err)
	}
	if idle != 2 {
		t.Errorf("status=idle total=%d, want 2", idle)
	}

	_, sorting, err := svc.ListModules(ctx, 1, 20, string(module.StatusSorting))
	if err != nil {
		t.Fatalf("ListModules: %v", err)
	}
	if sorting != 0 {
		t.Errorf("status=sorting total=%d, want 0", sorting)
	}
}
