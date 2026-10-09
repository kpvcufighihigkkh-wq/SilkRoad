package edge_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

func TestResyncFailed_ResetsFailedToPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-1").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusFailed).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncFailed(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncFailed: %v", err)
	}
	if n != 1 {
		t.Errorf("重置条数 = %d, want 1", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}

func TestResyncAll_ResetsSyncedToPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-2").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		SetSyncStatus(lot.SyncStatusSynced).
		Save(ctx)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncAll(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncAll: %v", err)
	}
	if n != 1 {
		t.Errorf("重置条数 = %d, want 1", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if !got.SyncedAt.IsZero() {
		t.Errorf("SyncedAt 应被清空，实际 %v", got.SyncedAt)
	}
}

// collectableSyncCase 描述一张可采集表的播种与读回方式，
// 供「每张可采集表都要能被重置」的表驱动用例复用。
type collectableSyncCase struct {
	table  string
	seed   func(t *testing.T, c *ent.Client, ctx context.Context, status string, retries int, syncedAt time.Time) uuid.UUID
	reload func(t *testing.T, c *ent.Client, ctx context.Context, id uuid.UUID) (string, int, time.Time)
}

// collectableSyncCases 与 collectors() 同集（lots / doffings / bobbins /
// bobbin_grades）。collectors() 是「真正会被采集」的权威定义，重置端点必须
// 与它同集：多一张（如 barrels）会造出永远排不出去的积压，少一张则该表的
// failed 记录仍无回收路径。
func collectableSyncCases() []collectableSyncCase {
	return []collectableSyncCase{
		{
			table: "lots",
			seed: func(t *testing.T, c *ent.Client, ctx context.Context, status string, retries int, syncedAt time.Time) uuid.UUID {
				t.Helper()
				row, err := c.Lot.Create().
					SetLotNumber("LOT-" + uuid.NewString()).
					SetProductType("FDY").
					SetPlannedQuantity(1).
					SetSyncStatus(lot.SyncStatus(status)).
					SetSyncRetryCount(retries).
					SetSyncedAt(syncedAt).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed lot: %v", err)
				}
				return row.ID
			},
			reload: func(t *testing.T, c *ent.Client, ctx context.Context, id uuid.UUID) (string, int, time.Time) {
				t.Helper()
				row, err := c.Lot.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload lot: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount, row.SyncedAt
			},
		},
		{
			table: "doffings",
			seed: func(t *testing.T, c *ent.Client, ctx context.Context, status string, retries int, syncedAt time.Time) uuid.UUID {
				t.Helper()
				parent, err := c.Lot.Create().
					SetLotNumber("LOT-PARENT-" + uuid.NewString()).
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed parent lot: %v", err)
				}
				row, err := c.Doffing.Create().
					SetSpinningLineID(uuid.New()).
					SetSpinningPosition(1).
					SetLotID(parent.ID).
					SetSyncStatus(doffing.SyncStatus(status)).
					SetSyncRetryCount(retries).
					SetSyncedAt(syncedAt).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed doffing: %v", err)
				}
				return row.ID
			},
			reload: func(t *testing.T, c *ent.Client, ctx context.Context, id uuid.UUID) (string, int, time.Time) {
				t.Helper()
				row, err := c.Doffing.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload doffing: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount, row.SyncedAt
			},
		},
		{
			table: "bobbins",
			seed: func(t *testing.T, c *ent.Client, ctx context.Context, status string, retries int, syncedAt time.Time) uuid.UUID {
				t.Helper()
				parent, err := c.Lot.Create().
					SetLotNumber("LOT-PARENT-" + uuid.NewString()).
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed parent lot: %v", err)
				}
				row, err := c.Bobbin.Create().
					SetBobbinNumber("BOBBIN-" + uuid.NewString()).
					SetLotID(parent.ID).
					SetSpinningPosition(1).
					SetGrossWeight(1.5).
					SetNetWeight(1.2).
					SetSyncStatus(bobbin.SyncStatus(status)).
					SetSyncRetryCount(retries).
					SetSyncedAt(syncedAt).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed bobbin: %v", err)
				}
				return row.ID
			},
			reload: func(t *testing.T, c *ent.Client, ctx context.Context, id uuid.UUID) (string, int, time.Time) {
				t.Helper()
				row, err := c.Bobbin.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload bobbin: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount, row.SyncedAt
			},
		},
		{
			// bobbin_grades 没有外键，bobbin_id 直接给一个 UUID。
			// 这正好复现 Ruling V 的场景：父丝饼从未同步的孤儿质检记录
			// 耗尽 5 次重试后落在 failed，本端点要能把它捞回来。
			table: "bobbin_grades",
			seed: func(t *testing.T, c *ent.Client, ctx context.Context, status string, retries int, syncedAt time.Time) uuid.UUID {
				t.Helper()
				row, err := c.BobbinGrade.Create().
					SetBobbinID(uuid.New()).
					SetGrade("A").
					SetSyncStatus(bobbingrade.SyncStatus(status)).
					SetSyncRetryCount(retries).
					SetSyncedAt(syncedAt).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed bobbin grade: %v", err)
				}
				return row.ID
			},
			reload: func(t *testing.T, c *ent.Client, ctx context.Context, id uuid.UUID) (string, int, time.Time) {
				t.Helper()
				row, err := c.BobbinGrade.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload bobbin grade: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount, row.SyncedAt
			},
		},
	}
}

// 每张可采集表都要能被两种模式重置，且计数归零、synced_at 清空。
//
// 只测 lots 时，删掉 bobbin_grades 的 switch 分支不会有任何测试变红 ——
// 而孤儿质检记录正是本端点存在的理由（Ruling V）。
func TestResync_CoversEveryCollectableTable(t *testing.T) {
	now := time.Now()

	for _, tc := range collectableSyncCases() {
		t.Run(tc.table+"/failed", func(t *testing.T) {
			client := setupEdgeClient(t)
			ctx := context.Background()
			id := tc.seed(t, client, ctx, "failed", 5, now)

			u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
			n, err := u.ResyncFailed(ctx, tc.table)
			if err != nil {
				t.Fatalf("ResyncFailed(%q): %v", tc.table, err)
			}
			if n != 1 {
				t.Errorf("重置条数 = %d, want 1", n)
			}

			status, retries, syncedAt := tc.reload(t, client, ctx, id)
			if status != "pending" {
				t.Errorf("SyncStatus = %q, want %q", status, "pending")
			}
			if retries != 0 {
				t.Errorf("SyncRetryCount = %d, want 0", retries)
			}
			if !syncedAt.IsZero() {
				t.Errorf("SyncedAt 应被清空，实际 %v", syncedAt)
			}
		})

		t.Run(tc.table+"/all", func(t *testing.T) {
			client := setupEdgeClient(t)
			ctx := context.Background()
			id := tc.seed(t, client, ctx, "synced", 0, now)

			u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
			n, err := u.ResyncAll(ctx, tc.table)
			if err != nil {
				t.Fatalf("ResyncAll(%q): %v", tc.table, err)
			}
			if n != 1 {
				t.Errorf("重置条数 = %d, want 1", n)
			}

			status, retries, syncedAt := tc.reload(t, client, ctx, id)
			if status != "pending" {
				t.Errorf("SyncStatus = %q, want %q", status, "pending")
			}
			if retries != 0 {
				t.Errorf("SyncRetryCount = %d, want 0", retries)
			}
			if !syncedAt.IsZero() {
				t.Errorf("SyncedAt 应被清空，实际 %v", syncedAt)
			}
		})
	}
}

// 只重置目标状态：ResyncFailed 不得碰 synced 记录，ResyncAll 不得碰 failed 记录。
//
// 可证伪：WHERE 里漏掉状态过滤时，两种模式都会把全部记录重置，
// 计数 n 与「另一行保持原状」的断言会同时失败。
func TestResync_OnlyTargetStatusIsReset(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	seed := func(t *testing.T, status string, retries int) uuid.UUID {
		t.Helper()
		row, err := client.Lot.Create().
			SetLotNumber("LOT-SCOPE-" + uuid.NewString()).
			SetProductType("FDY").
			SetPlannedQuantity(1).
			SetSyncStatus(lot.SyncStatus(status)).
			SetSyncRetryCount(retries).
			Save(ctx)
		if err != nil {
			t.Fatalf("seed %s lot: %v", status, err)
		}
		return row.ID
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	load := func(t *testing.T, id uuid.UUID) *ent.Lot {
		t.Helper()
		row, err := client.Lot.Get(ctx, id)
		if err != nil {
			t.Fatalf("reload lot %s: %v", id, err)
		}
		return row
	}

	failedID := seed(t, "failed", 0)
	syncedID := seed(t, "synced", 0)
	// 计数已达上限但状态不是 pending：不该被 ResyncFailed 命中。
	// 它锁住谓词里 And(pending, retryGTE) 的那一半 —— 若把「且 pending」
	// 丢掉只留计数判断，任何恰好超限的非 pending 行都会被误重置。
	syncedAtCapID := seed(t, "synced", 5)
	// 计数 3 < 上限：在途的正常 pending 行，不该被命中。
	pendingUnderCapID := seed(t, "pending", 3)

	n, err := u.ResyncFailed(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncFailed: %v", err)
	}
	if n != 1 {
		t.Errorf("ResyncFailed 重置条数 = %d, want 1（只有 failed 记录）", n)
	}

	if got := load(t, failedID); got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("failed 记录 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got := load(t, pendingUnderCapID); got.SyncStatus != lot.SyncStatusPending || got.SyncRetryCount != 3 {
		t.Errorf("在途 pending 记录被误改：status=%q retry=%d, want pending/3", got.SyncStatus, got.SyncRetryCount)
	}
	if got := load(t, syncedAtCapID); got.SyncStatus != lot.SyncStatusSynced || got.SyncRetryCount != 5 {
		t.Errorf("非 pending 的超限记录被误改：status=%q retry=%d, want synced/5", got.SyncStatus, got.SyncRetryCount)
	}
	if got := load(t, syncedID); got.SyncStatus != lot.SyncStatusSynced {
		t.Errorf("synced 记录被误改：status=%q, want synced", got.SyncStatus)
	}

	n, err = u.ResyncAll(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncAll: %v", err)
	}
	// 两条 synced 行（syncedID 与 syncedAtCapID）都属「已同步存量」，
	// 全量重传的谓词只看状态、不看计数。
	if n != 2 {
		t.Errorf("ResyncAll 重置条数 = %d, want 2（两条 synced 记录）", n)
	}
	if got := load(t, pendingUnderCapID); got.SyncStatus != lot.SyncStatusPending || got.SyncRetryCount != 3 {
		t.Errorf("pending 记录被 ResyncAll 误改：status=%q retry=%d, want pending/3", got.SyncStatus, got.SyncRetryCount)
	}
	if got := load(t, syncedID); got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("synced 记录 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
}

// table 为空表示全部可采集表：每张表都要被重置到。
func TestResync_EmptyTableMeansAll(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	cases := collectableSyncCases()
	ids := make(map[string]uuid.UUID, len(cases))
	for _, tc := range cases {
		ids[tc.table] = tc.seed(t, client, ctx, "failed", 5, time.Time{})
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncFailed(ctx, "")
	if err != nil {
		t.Fatalf("ResyncFailed(\"\"): %v", err)
	}
	if n != len(cases) {
		t.Errorf("重置条数 = %d, want %d（全部可采集表）", n, len(cases))
	}

	for _, tc := range cases {
		status, retries, _ := tc.reload(t, client, ctx, ids[tc.table])
		if status != "pending" {
			t.Errorf("%s 未被重置：SyncStatus = %q, want pending", tc.table, status)
		}
		if retries != 0 {
			t.Errorf("%s 重试计数未归零：%d", tc.table, retries)
		}
	}
}

// 未知表名是错误，而非静默 no-op。
//
// 尤其 barrels：它在 markableTables 里但**不被采集**，重置它只会造出
// 永远排不出去的积压（采集器不取、计数不增、markExhausted 也碰不到）。
func TestResync_UnknownTableIsError(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	for _, table := range []string{"barrels", "no_such_table", "Lots"} {
		if _, err := u.ResyncFailed(ctx, table); err == nil {
			t.Errorf("ResyncFailed(%q) 应返回错误，实际为 nil", table)
		}
		if _, err := u.ResyncAll(ctx, table); err == nil {
			t.Errorf("ResyncAll(%q) 应返回错误，实际为 nil", table)
		}
	}
}

// ResyncFailed 的契约是「重试超限的记录」，这不等于「状态为 failed」。
//
// 一行 pending 且 retry_count 已达上限的记录同样超限：采集器按
// `retry_count < 上限` 过滤把它挡在门外，它又没走到 failed。
// 若端点只匹配 failed，这行会被漏掉、永久卡在 pending ——
// 正是端点注释里承诺要回收的那一类。
//
// 可证伪：把 includeRetryExhaustedPending 置回 false，本用例必红。
func TestResyncFailed_RecoversPendingAtRetryCap(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-CAPPED-PENDING").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncStatus(lot.SyncStatusPending).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed pending-at-cap lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncFailed(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncFailed: %v", err)
	}
	if n != 1 {
		t.Errorf("重置条数 = %d, want 1（pending 且计数已达上限也属重试超限）", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("reload lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0（须归还完整重试预算）", got.SyncRetryCount)
	}
}

// 放宽后的谓词不得误伤计数未达上限的 pending 行。
//
// 它们是正常的在途记录：采集器照常采集，重试预算尚未耗尽。
// 若把条件写成 GT 或干脆去掉计数判断，本用例必红。
func TestResyncFailed_LeavesPendingUnderCapAlone(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	// 4 < 上限 5：仍可被采集，不属于超限。
	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-UNDER-CAP").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncStatus(lot.SyncStatusPending).
		SetSyncRetryCount(4).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed under-cap lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncFailed(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncFailed: %v", err)
	}
	if n != 0 {
		t.Errorf("重置条数 = %d, want 0（计数 4 < 上限 5，不属重试超限）", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("reload lot: %v", err)
	}
	if got.SyncRetryCount != 4 {
		t.Errorf("在途记录的重试计数被重置：%d, want 4", got.SyncRetryCount)
	}
}

// ResyncAll 只重置 synced：它是「重传已同步的存量」，
// 不承担回收超限记录的职责（那是 ResyncFailed 的契约）。
func TestResyncAll_LeavesPendingAtCapAlone(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-ALL-CAP").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncStatus(lot.SyncStatusPending).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed pending-at-cap lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")
	n, err := u.ResyncAll(ctx, "lots")
	if err != nil {
		t.Fatalf("ResyncAll: %v", err)
	}
	if n != 0 {
		t.Errorf("重置条数 = %d, want 0（ResyncAll 只匹配 synced）", n)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("reload lot: %v", err)
	}
	if got.SyncRetryCount != 5 {
		t.Errorf("SyncedAt/SyncRetryCount 被 ResyncAll 误改：retry=%d, want 5", got.SyncRetryCount)
	}
}
