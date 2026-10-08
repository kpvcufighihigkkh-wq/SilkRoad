package hook_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
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

// 业务字段被改 → synced_at 被清空。
//
// 契约三要素之一的 synced_at 必须单独断言：删掉 markDirty 里的
// ClearSyncedAt() 而测试仍然全绿，是已经发生过的真实缺陷（整个套件只验证了
// sync_status 与 sync_retry_count）。此处种一个非零 synced_at，验证它被清回零值。
func TestDirtyMarker_BusinessWriteClearsSyncedAt(t *testing.T) {
	client := newDirtyClient(t)
	ctx := context.Background()

	syncedAt := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	row, err := client.Lot.Create().
		SetLotNumber("LOT-DIRTY-SYNCEDAT").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusSynced).
		SetSyncedAt(syncedAt).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 前置断言：种子值确实落库，否则后面的"被清空"无从谈起
	seeded, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get seeded: %v", err)
	}
	if !seeded.SyncedAt.Equal(syncedAt) {
		t.Fatalf("前置条件不成立：SyncedAt = %v, want %v", seeded.SyncedAt, syncedAt)
	}

	if err := row.Update().SetStatus(lot.StatusCompleted).Exec(ctx); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !got.SyncedAt.IsZero() {
		t.Errorf("SyncedAt = %v, want 零值（业务写入应清空 synced_at）", got.SyncedAt)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
}

// bobbin 与 doffing 分支同样必须置脏。
//
// 表驱动覆盖 markDirty 的另两条 case：曾经出现过"整张表的置脏代码被删掉而
// 全套件仍绿"的缺陷，所以每条分支都要有业务写入与"仅同步字段写入"两侧断言。
// 各实体的字段名、触发器 API 不同，故 per-case 提供闭包。
func TestDirtyMarker_CoversBobbinAndDoffing(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name string
		// seed 建一条 synced + retry=3 + 非空 synced_at 的记录
		seed func(t *testing.T, client *ent.Client)
		// businessWrite 制造一次业务字段写入
		businessWrite func(t *testing.T, client *ent.Client)
		// syncOnlyWrite 制造一次"只改同步字段"的写入
		syncOnlyWrite func(t *testing.T, client *ent.Client)
		// read 读回 (当前状态, retry, synced_at)
		read func(t *testing.T, client *ent.Client) (status string, retry int, syncedAt time.Time)
	}{
		{
			name: "bobbin",
			seed: func(t *testing.T, client *ent.Client) {
				t.Helper()
				if _, err := client.Bobbin.Create().
					SetBobbinNumber("BOB-DIRTY-1").
					SetLotID(lotUUIDForDirtyTest(t, client)).
					SetSpinningPosition(1).
					SetGrossWeight(1.5).
					SetNetWeight(1.2).
					SetSyncStatus(bobbin.SyncStatusSynced).
					SetSyncRetryCount(3).
					SetSyncedAt(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)).
					Save(ctx); err != nil {
					t.Fatalf("seed bobbin: %v", err)
				}
			},
			businessWrite: func(t *testing.T, client *ent.Client) {
				t.Helper()
				// 业务字段：质检等级
				if err := client.Bobbin.Update().
					Where(bobbin.BobbinNumberEQ("BOB-DIRTY-1")).
					SetGrade("A").
					Exec(ctx); err != nil {
					t.Fatalf("bobbin business write: %v", err)
				}
			},
			syncOnlyWrite: func(t *testing.T, client *ent.Client) {
				t.Helper()
				if err := client.Bobbin.Update().
					Where(bobbin.BobbinNumberEQ("BOB-DIRTY-1")).
					SetSyncStatus(bobbin.SyncStatusSynced).
					SetSyncRetryCount(3).
					Exec(ctx); err != nil {
					t.Fatalf("bobbin sync-only write: %v", err)
				}
			},
			read: func(t *testing.T, client *ent.Client) (string, int, time.Time) {
				t.Helper()
				got, err := client.Bobbin.Query().
					Where(bobbin.BobbinNumberEQ("BOB-DIRTY-1")).
					Only(ctx)
				if err != nil {
					t.Fatalf("read bobbin: %v", err)
				}
				return string(got.SyncStatus), got.SyncRetryCount, got.SyncedAt
			},
		},
		{
			name: "doffing",
			seed: func(t *testing.T, client *ent.Client) {
				t.Helper()
				if _, err := client.Doffing.Create().
					SetSpinningLineID(uuidForDirtyTest()).
					SetSpinningPosition(1).
					SetLotID(lotUUIDForDirtyTest(t, client)).
					SetSyncStatus(doffing.SyncStatusSynced).
					SetSyncRetryCount(3).
					SetSyncedAt(time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)).
					Save(ctx); err != nil {
					t.Fatalf("seed doffing: %v", err)
				}
			},
			businessWrite: func(t *testing.T, client *ent.Client) {
				t.Helper()
				// 业务字段：落纱状态确认
				if err := client.Doffing.Update().
					SetStatus(doffing.StatusConfirmed).
					Exec(ctx); err != nil {
					t.Fatalf("doffing business write: %v", err)
				}
			},
			syncOnlyWrite: func(t *testing.T, client *ent.Client) {
				t.Helper()
				if err := client.Doffing.Update().
					SetSyncStatus(doffing.SyncStatusSynced).
					SetSyncRetryCount(3).
					Exec(ctx); err != nil {
					t.Fatalf("doffing sync-only write: %v", err)
				}
			},
			read: func(t *testing.T, client *ent.Client) (string, int, time.Time) {
				t.Helper()
				got, err := client.Doffing.Query().Only(ctx)
				if err != nil {
					t.Fatalf("read doffing: %v", err)
				}
				return string(got.SyncStatus), got.SyncRetryCount, got.SyncedAt
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/业务写入置脏", func(t *testing.T) {
			client := newDirtyClient(t)
			tc.seed(t, client)

			tc.businessWrite(t, client)

			status, retry, syncedAt := tc.read(t, client)
			if status != "pending" {
				t.Errorf("SyncStatus = %q, want %q（业务字段被改应重新置脏）", status, "pending")
			}
			if retry != 0 {
				t.Errorf("SyncRetryCount = %d, want 0（置脏应重置重试计数）", retry)
			}
			if !syncedAt.IsZero() {
				t.Errorf("SyncedAt = %v, want 零值（置脏应清空 synced_at）", syncedAt)
			}
		})

		t.Run(tc.name+"/仅同步字段写入不置脏", func(t *testing.T) {
			client := newDirtyClient(t)
			tc.seed(t, client)

			tc.syncOnlyWrite(t, client)

			status, retry, _ := tc.read(t, client)
			if status != "synced" {
				t.Errorf("SyncStatus = %q, want %q（只改同步字段不得被拦回 pending）", status, "synced")
			}
			if retry != 3 {
				t.Errorf("SyncRetryCount = %d, want 3（仅同步字段写入不得由 hook 改动）", retry)
			}
		})
	}
}

// lotUUIDForDirtyTest 建一个最小 Lot，返回其 ID，用于满足 bobbin/doffing 的
// 非空 lot_id 外键（_fk=1 下缺它会直接约束失败）。
func lotUUIDForDirtyTest(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	row, err := client.Lot.Create().
		SetLotNumber(fmt.Sprintf("LOT-FK-%d", atomic.AddInt64(&dirtyTestCounter, 1))).
		SetProductType("FDY").
		SetPlannedQuantity(1).
		Save(context.Background())
	if err != nil {
		t.Fatalf("seed fk lot: %v", err)
	}
	return row.ID
}

// uuidForDirtyTest 返回一个随机 UUID，用于 doffing 的 spinning_line_id
// （该字段没有外键约束，无需真实行）。
func uuidForDirtyTest() uuid.UUID {
	return uuid.New()
}
