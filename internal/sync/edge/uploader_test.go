package edge_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

func setupEdgeClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:edgeuptest%d?mode=memory&cache=shared&_fk=1",
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

// 只收集 pending 状态的记录
func TestUploader_CollectPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	// pending 批次
	pending, err := client.Lot.Create().
		SetLotNumber("LOT-PENDING").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 已同步批次
	synced, err := client.Lot.Create().
		SetLotNumber("LOT-SYNCED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		SetSyncStatus("synced").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating synced lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("收集到 %d 条，want 1（只应收集 pending）", len(entries))
	}
	if entries[0].ID != pending.ID {
		t.Errorf("收集到 %v，want %v", entries[0].ID, pending.ID)
	}
	if entries[0].ID == synced.ID {
		t.Error("已同步记录被重复收集")
	}
	if entries[0].Table != "lots" {
		t.Errorf("Table = %q, want %q", entries[0].Table, "lots")
	}
}

// 超过重试上限的记录不再收集
func TestUploader_CollectPending_SkipsExhaustedRetries(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-EXHAUSTED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		SetSyncRetryCount(5).
		Save(ctx); err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("收集到 %d 条，want 0（retry_count 已达上限）", len(entries))
	}
}

// 收集结果必须按外键依赖顺序排列，
// 且只采集 Center 的 handleCreate 真正接受的表（lots / bobbins）。
func TestUploader_CollectPending_OrderedByDependency(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-ORDER").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	if _, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-ORDER").
		SetLotID(record.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx); err != nil {
		t.Fatalf("failed creating bobbin: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("收集到 %d 条，want 2", len(entries))
	}
	if entries[0].Table != "lots" {
		t.Errorf("首个应为 lots，实际 %q（外键要求被引用者先到）", entries[0].Table)
	}
	if entries[1].Table != "bobbins" {
		t.Errorf("第二个应为 bobbins，实际 %q", entries[1].Table)
	}
}

// 未接入 Center 的表不得被采集。
//
// Center 的 handleCreate 只支持 lots 与 bobbins，其余表一律返回
// "unknown table" 而被拒绝。若把 doffings / barrels 采集进来，它们会被
// 反复拒绝、累加重试次数，最终被 markExhausted 标记为 failed ——
// 而本仓库没有回收路径。不采集则保持 pending/retry_count=0，
// 是「延后」而非「丢失」，等 Center 支持后仍能同步。
func TestUploader_CollectPending_SkipsUnsupportedTables(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-UNSUPPORTED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	if _, err := client.Barrel.Create().
		SetBarrelNumber("BARREL-UNSUPPORTED").
		SetLotID(record.ID).
		Save(ctx); err != nil {
		t.Fatalf("failed creating barrel: %v", err)
	}

	if _, err := client.Doffing.Create().
		SetSpinningLineID(uuid.New()).
		SetSpinningPosition(1).
		SetLotID(record.ID).
		Save(ctx); err != nil {
		t.Fatalf("failed creating doffing: %v", err)
	}

	u := edge.NewUploader(client, "edge-001", "http://center:8080", "token")

	entries, err := u.CollectPending(ctx, 100)
	if err != nil {
		t.Fatalf("CollectPending failed: %v", err)
	}

	// 只有 lot 应被采集
	if len(entries) != 1 {
		t.Fatalf("收集到 %d 条，want 1（barrels/doffings 尚未被 Center 支持）", len(entries))
	}
	if entries[0].Table != "lots" {
		t.Errorf("被采集的表 = %q, want %q", entries[0].Table, "lots")
	}
}

// newCenterStub 造一个假的 Center 上传端点。
//
// 路径必须是 /v1/edges/:code/upload —— 生产 Edge 侧按 :code 寻址，
// 这里断言收到的正是该路径，防止实现退回不带设备宽度的 URL。
//
// 响应体按 api.Success 的封套构造：Edge 侧解析的是 {"code":..,"data":{..}}。
func newCenterStub(t *testing.T, code int, data interface{}) (*httptest.Server, *string) {
	t.Helper()

	return newCenterStubFull(t, code, data, 0)
}

// newCenterStubFull 与 newCenterStub 相同，另可延迟响应并断言请求头。
//
// delay > 0 时响应前先等待，用于制造「上传在飞行」的时间窗口。
func newCenterStubFull(t *testing.T, code int, data interface{}, delay time.Duration) (*httptest.Server, *string) {
	t.Helper()

	gotPath := new(string)
	block := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotPath = r.URL.Path

		// 缺少 Bearer 凭证的上传会被 Center 的 authenticateEdge 拒绝，
		// 删掉请求头设置应当让测试变红。只断言形状与存在性，不绑定具体
		// 字面量，避免每个调用点都要跟着改。
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")) == "" {
			t.Errorf("Authorization = %q, want 形如 %q 的非空凭证", auth, "Bearer <token>")
		}

		if delay > 0 {
			close(block)
			time.Sleep(delay)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    data,
		}); err != nil {
			t.Errorf("stub encode failed: %v", err)
		}
	}))

	// 延迟模式下必须等真实请求到达再关服务器，否则 handler 内的
	// t.Errorf 会在测试结束后调用，panic 成 "Log in goroutine after test
	// has completed"。非延迟模式没有请求阻塞，直接关即可。
	if delay > 0 {
		t.Cleanup(func() {
			select {
			case <-block:
			case <-time.After(5 * time.Second):
			}
			srv.Close()
		})
	} else {
		t.Cleanup(srv.Close)
	}

	return srv, gotPath
}

// 部分拒绝：被拒记录必须留在 pending 并累加重试次数，只有真正应用的记录才标记 synced。
//
// 这是本任务的核心回归测试。Center 的 HandleUpload 逐条处理、失败只累加 rejected
// 并继续（handler.go:87-95），因此 applied=1, rejected=1 是可达状态。
// 若按「整批成功」标记，被拒记录会被静默标记 synced —— 从队列消失、永不重试，
// 与同步层存在的意义（消除假成功）背道而驰。
func TestUploader_Upload_PartialRejection_KeepsRejectedPending(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-PARTIAL").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 一条 Center 会拒绝的记录。这里用 bobbins（在采集列表内）并让桩返回
	// 拒绝，因为 item 4 之后只有 lots/bobbins 会被采集 —— 这样批次里
	// 确实同时存在「被应用」与「被拒」两条记录，才是本测试要验证的场景。
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-PARTIAL").
		SetLotID(record.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating bobbin: %v", err)
	}

	// 复刻 Center 的错误串格式：fmt.Sprintf("%s/%s: %v", entry.Table, entry.ID, err)
	rejectedMsg := fmt.Sprintf("bobbins/%s: unknown table: bobbins", bobbinRow.ID)

	srv, gotPath := newCenterStub(t, http.StatusOK, map[string]interface{}{
		"applied":    1,
		"rejected":   1,
		"errors":     []string{rejectedMsg},
		"new_cursor": 0,
	})

	u := edge.NewUploader(client, "edge-001", srv.URL, "token")

	resp, err := u.Upload(ctx)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if *gotPath != "/v1/edges/edge-001/upload" {
		t.Errorf("请求路径 = %q, want %q（:code 缺失会被鉴权第三重拒绝）",
			*gotPath, "/v1/edges/edge-001/upload")
	}
	if resp.Applied != 1 || resp.Rejected != 1 {
		t.Errorf("applied/rejected = %d/%d, want 1/1", resp.Applied, resp.Rejected)
	}

	// 被应用的记录：synced 且带 synced_at
	appliedLot, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if appliedLot.SyncStatus != lot.SyncStatusSynced {
		t.Errorf("已应用记录 SyncStatus = %q, want %q", appliedLot.SyncStatus, lot.SyncStatusSynced)
	}
	if appliedLot.SyncedAt.IsZero() {
		t.Error("已应用记录 SyncedAt 未写入")
	}
	if appliedLot.SyncRetryCount != 0 {
		t.Errorf("已应用记录 SyncRetryCount = %d, want 0", appliedLot.SyncRetryCount)
	}

	// 被拒绝的记录：必须仍是 pending，retry_count 累加到 1 —— 下轮还会被收集重试
	rejectedRow, err := client.Bobbin.Get(ctx, bobbinRow.ID)
	if err != nil {
		t.Fatalf("failed reloading bobbin: %v", err)
	}
	if rejectedRow.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("被拒记录 SyncStatus = %q, want %q（不得被静默标记 synced）",
			rejectedRow.SyncStatus, bobbin.SyncStatusPending)
	}
	if rejectedRow.SyncRetryCount != 1 {
		t.Errorf("被拒记录 SyncRetryCount = %d, want 1", rejectedRow.SyncRetryCount)
	}
}

// 计数对不上时必须保守处理：整批不标记 synced，只累加重试次数。
//
// Center 返回 applied=5 而本批只有 1 条 —— 响应与请求不自洽，说明我们对响应的
// 理解有误。此时宁可贵一次往返（Center 的 create* 是幂等的，重发不会产生重复），
// 也不能猜错哪些记录成功。
func TestUploader_Upload_UnreconciledCounts_MarksNothingSynced(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-UNRECONCILED").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// applied 大于本批条数，且 errors 为空 —— 无法定位任何被拒记录
	srv, _ := newCenterStub(t, http.StatusOK, map[string]interface{}{
		"applied":  5,
		"rejected": 0,
		"errors":   []string{},
	})

	u := edge.NewUploader(client, "edge-001", srv.URL, "token")

	if _, err := u.Upload(ctx); err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	got, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("计数不自洽时 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 1 {
		t.Errorf("计数不自洽时 SyncRetryCount = %d, want 1", got.SyncRetryCount)
	}
}

// 错误串无法解析时同样退化为整批重试。
//
// 计数对得上（applied=0 + rejected=1 = 本批 1 条），但错误串里没有
// "<table>/<uuid>" 前缀，无法定位是哪条被拒 —— 不得因此把整批当成功。
func TestUploader_Upload_UnparseableError_MarksNothingSynced(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-UNPARSEABLE").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	srv, _ := newCenterStub(t, http.StatusOK, map[string]interface{}{
		"applied":  0,
		"rejected": 1,
		"errors":   []string{"未知错误，没有表名与 ID 前缀"},
	})

	u := edge.NewUploader(client, "edge-001", srv.URL, "token")

	if _, err := u.Upload(ctx); err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	got, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("错误串不可解析时 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 1 {
		t.Errorf("错误串不可解析时 SyncRetryCount = %d, want 1", got.SyncRetryCount)
	}
}

// 错误串指向本批之外的记录时退化为整批重试。
//
// 计数对得上（applied=1 + rejected=1 = 本批 2 条），但被拒的 ID 不在本批中，
// 说明我们对响应的理解有误 —— 此时一条都不标记 synced，两条都计重试。
func TestUploader_Upload_RejectedIDOutsideBatch_MarksNothingSynced(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-FOREIGN-ID").
		SetProductType("FDY").
		SetPlannedQuantity(100).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	// 第二条必须来自采集列表内（lots/bobbins），否则批次只有 1 条，
	// 计数会对不上而走另一条分支，测不到「越界 ID」这条路径。
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-FOREIGN-ID").
		SetLotID(record.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating bobbin: %v", err)
	}

	// 错误串指向一个既不在本批中的陌生 ID
	foreignMsg := fmt.Sprintf("lots/%s: boom", uuid.New())

	srv, _ := newCenterStub(t, http.StatusOK, map[string]interface{}{
		"applied":  1,
		"rejected": 1,
		"errors":   []string{foreignMsg},
	})

	u := edge.NewUploader(client, "edge-001", srv.URL, "token")

	if _, err := u.Upload(ctx); err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	gotLot, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if gotLot.SyncStatus != lot.SyncStatusPending {
		t.Errorf("错误串越界时 lot SyncStatus = %q, want %q", gotLot.SyncStatus, lot.SyncStatusPending)
	}
	if gotLot.SyncRetryCount != 1 {
		t.Errorf("错误串越界时 lot SyncRetryCount = %d, want 1", gotLot.SyncRetryCount)
	}

	gotBobbin, err := client.Bobbin.Get(ctx, bobbinRow.ID)
	if err != nil {
		t.Fatalf("failed reloading bobbin: %v", err)
	}
	if gotBobbin.SyncStatus != bobbin.SyncStatusPending {
		t.Errorf("错误串越界时 bobbin SyncStatus = %q, want %q",
			gotBobbin.SyncStatus, bobbin.SyncStatusPending)
	}
	if gotBobbin.SyncRetryCount != 1 {
		t.Errorf("错误串越界时 bobbin SyncRetryCount = %d, want 1", gotBobbin.SyncRetryCount)
	}
}
