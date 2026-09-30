package edge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
	_ "modernc.org/sqlite"
)

// 本文件用内部包（package edge）测试未导出的标记函数。
//
// 上传侧对外的 API 是 edge_test（外部包）的 uploader_test.go；
// 但 markRetried / markSynced / markExhausted 未导出，且 item 4 之后
// barrels/doffings 已不在采集列表内，无法经 Upload 进入标记分支 ——
// 于是这两张表的标记代码没有任何测试覆盖，删掉对应 case 不会变红。
// 这里补上。

// newInternalEdgeClient 造一个内存 SQLite 的 ent.Client。
//
// 与外部包 uploader_test.go 的 setupEdgeClient 等价，但测试包不同
// （package edge vs edge_test），两者无法互相引用，因此各留一份。
// 计数器共用以保证库名不冲突 —— 见下方 internalTestDBCounter。
func newInternalEdgeClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:edgeinittest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&internalTestDBCounter, 1))

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

var internalTestDBCounter int64

// newInternalStub 造一个最小 Center 桩
func newInternalStub(t *testing.T, data interface{}) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0, "message": "success", "data": data,
		}); err != nil {
			t.Errorf("stub encode failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

// 每张可标记表都要能独立完成 synced 与 retried 两种标记。
//
// 表驱动覆盖 markableTables 全表：只测 lots/doffings 时，删掉 barrels 或
// bobbins 的 case 不会有任何测试变红 —— forEachTable 会静默跳过未知表。
func TestUploader_MarksEveryMarkableTable(t *testing.T) {
	cases := []struct {
		table string
		seed  func(t *testing.T, client *ent.Client, ctx context.Context) uuid.UUID
		check func(t *testing.T, client *ent.Client, ctx context.Context, id uuid.UUID) (string, int)
	}{
		{
			table: "lots",
			seed: func(t *testing.T, client *ent.Client, ctx context.Context) uuid.UUID {
				t.Helper()
				row, err := client.Lot.Create().
					SetLotNumber("LOT-MARK").
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed lot: %v", err)
				}
				return row.ID
			},
			check: func(t *testing.T, client *ent.Client, ctx context.Context, id uuid.UUID) (string, int) {
				t.Helper()
				row, err := client.Lot.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload lot: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount
			},
		},
		{
			table: "doffings",
			seed: func(t *testing.T, client *ent.Client, ctx context.Context) uuid.UUID {
				t.Helper()
				// doffing.lot_id 有外键，必须指向真实 lot
				lotRow, err := client.Lot.Create().
					SetLotNumber("LOT-MARK-DOFFING").
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed lot for doffing: %v", err)
				}
				row, err := client.Doffing.Create().
					SetSpinningLineID(uuid.New()).
					SetSpinningPosition(1).
					SetLotID(lotRow.ID).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed doffing: %v", err)
				}
				return row.ID
			},
			check: func(t *testing.T, client *ent.Client, ctx context.Context, id uuid.UUID) (string, int) {
				t.Helper()
				row, err := client.Doffing.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload doffing: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount
			},
		},
		{
			table: "barrels",
			seed: func(t *testing.T, client *ent.Client, ctx context.Context) uuid.UUID {
				t.Helper()
				lotRow, err := client.Lot.Create().
					SetLotNumber("LOT-MARK-BARREL").
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed lot for barrel: %v", err)
				}
				row, err := client.Barrel.Create().
					SetBarrelNumber("BARREL-MARK").
					SetLotID(lotRow.ID).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed barrel: %v", err)
				}
				return row.ID
			},
			check: func(t *testing.T, client *ent.Client, ctx context.Context, id uuid.UUID) (string, int) {
				t.Helper()
				row, err := client.Barrel.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload barrel: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount
			},
		},
		{
			table: "bobbins",
			seed: func(t *testing.T, client *ent.Client, ctx context.Context) uuid.UUID {
				t.Helper()
				lotRow, err := client.Lot.Create().
					SetLotNumber("LOT-MARK-BOBBIN").
					SetProductType("FDY").
					SetPlannedQuantity(1).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed lot for bobbin: %v", err)
				}
				row, err := client.Bobbin.Create().
					SetBobbinNumber("BOBBIN-MARK").
					SetLotID(lotRow.ID).
					SetSpinningPosition(1).
					SetGrossWeight(1.5).
					SetNetWeight(1.2).
					Save(ctx)
				if err != nil {
					t.Fatalf("seed bobbin: %v", err)
				}
				return row.ID
			},
			check: func(t *testing.T, client *ent.Client, ctx context.Context, id uuid.UUID) (string, int) {
				t.Helper()
				row, err := client.Bobbin.Get(ctx, id)
				if err != nil {
					t.Fatalf("reload bobbin: %v", err)
				}
				return string(row.SyncStatus), row.SyncRetryCount
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.table+"/synced", func(t *testing.T) {
			client := newInternalEdgeClient(t)
			ctx := context.Background()
			id := tc.seed(t, client, ctx)

			u := NewUploader(client, "edge-001", "http://center:8080", "test-token")

			entry := models.UploadEntry{Table: tc.table, Operation: "create", ID: id}
			if err := u.applyOutcome(ctx, []models.UploadEntry{entry},
				&models.UploadResponse{Applied: 1}); err != nil {
				t.Fatalf("applyOutcome failed: %v", err)
			}

			status, retries := tc.check(t, client, ctx, id)
			if status != "synced" {
				t.Errorf("%s 标记后 SyncStatus = %q, want %q", tc.table, status, "synced")
			}
			if retries != 0 {
				t.Errorf("%s 标记后 SyncRetryCount = %d, want 0", tc.table, retries)
			}
		})

		t.Run(tc.table+"/retried", func(t *testing.T) {
			client := newInternalEdgeClient(t)
			ctx := context.Background()
			id := tc.seed(t, client, ctx)

			u := NewUploader(client, "edge-001", "http://center:8080", "test-token")

			// 计数不自洽 → 整批计入重试（含该表）。经公开入口的真实响应驱动。
			if err := u.markRetried(ctx, map[string][]uuid.UUID{tc.table: {id}}); err != nil {
				t.Fatalf("markRetried failed: %v", err)
			}

			status, retries := tc.check(t, client, ctx, id)
			if status != "pending" {
				t.Errorf("%s 重试标记后 SyncStatus = %q, want %q", tc.table, status, "pending")
			}
			if retries != 1 {
				t.Errorf("%s 重试标记后 SyncRetryCount = %d, want 1", tc.table, retries)
			}
		})
	}
}

// markExhausted 必须覆盖全部四张表：重试超限的 pending 行要翻成 failed，
// 否则会永远停留在 pending 且不被采集，成为不可见的积压。
func TestUploader_MarkExhausted_CoversEveryMarkableTable(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	// 四张表各造一条已达上限的 pending 记录
	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-EXHAUST-MARK").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncRetryCount(maxRetryCount).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	doffingRow, err := client.Doffing.Create().
		SetSpinningLineID(uuid.New()).
		SetSpinningPosition(1).
		SetLotID(lotRow.ID).
		SetSyncRetryCount(maxRetryCount).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed doffing: %v", err)
	}

	barrelRow, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-EXHAUST-MARK").
		SetLotID(lotRow.ID).
		SetSyncRetryCount(maxRetryCount).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed barrel: %v", err)
	}

	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-EXHAUST-MARK").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		SetSyncRetryCount(maxRetryCount).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed bobbin: %v", err)
	}

	u := NewUploader(client, "edge-001", "http://center:8080", "test-token")
	if err := u.markExhausted(ctx); err != nil {
		t.Fatalf("markExhausted failed: %v", err)
	}

	if row, err := client.Lot.Get(ctx, lotRow.ID); err != nil {
		t.Fatalf("reload lot: %v", err)
	} else if row.SyncStatus != "failed" {
		t.Errorf("lots SyncStatus = %q, want failed", row.SyncStatus)
	}

	if row, err := client.Doffing.Get(ctx, doffingRow.ID); err != nil {
		t.Fatalf("reload doffing: %v", err)
	} else if row.SyncStatus != "failed" {
		t.Errorf("doffings SyncStatus = %q, want failed", row.SyncStatus)
	}

	if row, err := client.Barrel.Get(ctx, barrelRow.ID); err != nil {
		t.Fatalf("reload barrel: %v", err)
	} else if row.SyncStatus != "failed" {
		t.Errorf("barrels SyncStatus = %q, want failed", row.SyncStatus)
	}

	if row, err := client.Bobbin.Get(ctx, bobbinRow.ID); err != nil {
		t.Fatalf("reload bobbin: %v", err)
	} else if row.SyncStatus != "failed" {
		t.Errorf("bobbins SyncStatus = %q, want failed", row.SyncStatus)
	}
}

// Stop 可重复调用而不 panic（spec 要求）
func TestScheduler_StopIsIdempotent(t *testing.T) {
	client := newInternalEdgeClient(t)
	u := NewUploader(client, "edge-001", "http://center:8080", "test-token")

	s := NewScheduler(u, time.Hour)
	s.Start(context.Background())

	s.Stop()
	s.Stop() // 重复调用不得 panic（close of closed channel）
}
