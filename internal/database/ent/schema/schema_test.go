package schema_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/module"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

func newTestClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:schematest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&testDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

func TestLot_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	if created.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, lot.SyncStatusPending)
	}
	if created.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", created.SyncRetryCount)
	}
	if !created.SyncedAt.IsZero() {
		t.Errorf("SyncedAt = %v, want zero", created.SyncedAt)
	}
}

func TestBarrel_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-002").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-SYNC-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating barrel: %v", err)
	}

	if created.SyncStatus != barrel.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, barrel.SyncStatusPending)
	}
	if created.Capacity != 9 {
		t.Errorf("Capacity = %d, want default 9", created.Capacity)
	}
}

func TestBobbin_SyncDefaults(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-SYNC-003").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Bobbin.Create().
		SetBobbinNumber("BOB-SYNC-001").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(12.5).
		SetNetWeight(11.8).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating bobbin: %v", err)
	}

	if created.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", created.SyncStatus, bobbin.SyncStatusPending)
	}
}

func TestSyncStatusQueryByPending(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-PENDING-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx); err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	pending, err := client.Lot.Query().
		Where(lot.SyncStatusEQ(lot.SyncStatusPending)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if pending != 1 {
		t.Errorf("pending count = %d, want 1", pending)
	}

	synced, err := client.Lot.Query().
		Where(lot.SyncStatusEQ(lot.SyncStatusSynced)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if synced != 0 {
		t.Errorf("synced count = %d, want 0", synced)
	}
}

func TestSyncRetryCountAccumulates(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Lot.Create().
		SetLotNumber("LOT-RETRY-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	updated, err := client.Lot.UpdateOneID(created.ID).
		AddSyncRetryCount(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed updating lot: %v", err)
	}
	if updated.SyncRetryCount != 1 {
		t.Errorf("SyncRetryCount = %d, want 1", updated.SyncRetryCount)
	}
}

func TestBarrelCanBeCreatedWithoutDoffing(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-NODOFF-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 设计文档 §3.1：建桶不需要先有落纱记录
	created, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-NODOFF-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("barrel requires no doffing: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
}

func TestModule_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	// 不带 edge_id 也能创建（Edge未注册场景）
	anonymous, err := client.Module.Create().
		SetModuleNumber("MOD-ANON-001").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating module without edge: %v", err)
	}
	if anonymous.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", anonymous.EdgeID)
	}

	// 带 edge_id 创建
	edge, err := client.Edge.Create().
		SetEdgeCode("edge-mod-01").
		SetEdgeName("模块测试边端").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	tagged, err := client.Module.Create().
		SetModuleNumber("MOD-TAGGED-001").
		SetEdgeID(edge.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating module with edge: %v", err)
	}
	if tagged.EdgeID != edge.ID {
		t.Errorf("EdgeID = %v, want %v", tagged.EdgeID, edge.ID)
	}

	// 按 edge_id 过滤
	count, err := client.Module.Query().
		Where(module.EdgeIDEQ(edge.ID)).
		Count(ctx)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("modules for edge = %d, want 1", count)
	}
}

func TestPallet_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-PALLET-EDGE-001").
		SetProductType(lot.ProductTypeFDY).
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	created, err := client.Pallet.Create().
		SetPalletCode("PALLET-EDGE-001").
		SetLotID(lotRow.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating pallet: %v", err)
	}
	if created.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", created.EdgeID)
	}
}

func TestCarton_EdgeIDOptional(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	created, err := client.Carton.Create().
		SetCartonNumber("CARTON-EDGE-001").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating carton: %v", err)
	}
	if created.EdgeID != uuid.Nil {
		t.Errorf("EdgeID = %v, want nil", created.EdgeID)
	}
}
