package hook_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/hook"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	_ "modernc.org/sqlite"
)

var dirtyTestCounter int64

func newDirtyClient(t *testing.T) *ent.Client {
	t.Helper()
	name := fmt.Sprintf("file:dirtytest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&dirtyTestCounter, 1))
	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("enable fk: %v", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })
	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("schema: %v", err)
	}
	client.Use(hook.NewDirtyMarker())
	return client
}

// 业务字段被改 → 记录重新变 pending
func TestDirtyMarker_BusinessWriteMarksPending(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-1").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusSynced).
		SetSyncRetryCount(3).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := row.Update().SetStatus(lot.StatusCompleted).Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q（业务字段被改应重新置脏）", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0（置脏应重置重试计数，否则会被 markExhausted 立刻打回 failed）", got.SyncRetryCount)
	}
}

// 只改同步字段 → 不得再次置脏（否则 markSynced 会被拦回 pending，无限重传）
func TestDirtyMarker_SyncOnlyWriteDoesNotReDirty(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-2").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 模拟 markSynced：只改同步字段
	if err := client.Lot.Update().
		Where(lot.IDEQ(row.ID)).
		SetSyncStatus(lot.SyncStatusSynced).
		SetSyncRetryCount(0).
		Exec(ctx); err != nil {
		t.Fatalf("sync-only update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusSynced {
		t.Errorf("SyncStatus = %q, want %q（只改同步字段不得被拦回 pending）", got.SyncStatus, lot.SyncStatusSynced)
	}
}

// failed 记录被编辑 → 复活为 pending 并重置计数
func TestDirtyMarker_RevivesFailedRecord(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-3").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusFailed).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := row.Update().SetStatus(lot.StatusPaused).Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q（编辑应复活 failed 记录）", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}
