package edge_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

// 并发调用 Upload 只能发出一次 POST。
//
// 没有 uploadMu 时，N 个并发调用各自收集同一批 pending 记录并各发一次请求，
// retry_count 被 N 倍累加 —— reviewer 已复现 5 次并发把一条记录直接推到
// markExhausted 的 failed/5。跳过是安全的：记录仍是 pending，下次触发会重新采集。
func TestUploader_Upload_ConcurrentCallsIssueSinglePost(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-CONCURRENT").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		Save(ctx); err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	var posts int64

	// 慢响应制造「上传在飞行」的时间窗口，让后续调用必然撞上锁
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt64(&posts, 1)
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{"applied": 1, "rejected": 0, "errors": []string{}},
		}); err != nil {
			t.Errorf("stub encode failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	u := edge.NewUploader(client, "edge-001", srv.URL, "test-token")

	const callers = 5
	var wg sync.WaitGroup
	results := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := u.Upload(ctx)
			results[idx] = err
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt64(&posts); got != 1 {
		t.Errorf("%d 个并发调用发出了 %d 次 POST，want 1", callers, got)
	}

	var skipped int
	for _, err := range results {
		if errors.Is(err, edge.ErrUploadInProgress) {
			skipped++
		}
	}
	if skipped != callers-1 {
		t.Errorf("被跳过的调用数 = %d, want %d", skipped, callers-1)
	}

	// 被跳过的记录不得被推到 failed
	rows, err := client.Lot.Query().All(ctx)
	if err != nil {
		t.Fatalf("failed querying lots: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("查询到 %d 条 lot，want 1", len(rows))
	}
	if rows[0].SyncStatus == lot.SyncStatusFailed {
		t.Errorf("并发调用把记录推到了 %s", rows[0].SyncStatus)
	}
}

// 非 200 且非凭证类失败：整批计入重试，保持 pending
func TestUploader_Upload_NonOKStatusCountsRetry(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-500").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	srv, _ := newCenterStub(t, http.StatusInternalServerError, map[string]interface{}{})

	u := edge.NewUploader(client, "edge-001", srv.URL, "test-token")

	if _, err := u.Upload(ctx); err == nil {
		t.Fatal("Upload 应当返回错误")
	}

	got, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("500 后 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if got.SyncRetryCount != 1 {
		t.Errorf("500 后 SyncRetryCount = %d, want 1", got.SyncRetryCount)
	}
}

// 凭证类失败（401/403）不得消耗重试次数。
//
// 重发同样的请求必然同样失败，与记录本身无关。若计入 retry_count，
// 默认 5m 间隔下约 25 分钟后所有 pending 记录都会被标记为 failed，
// 而本仓库没有回收路径 —— 等于不可恢复的数据丢失。
func TestUploader_Upload_AuthFailureDoesNotCountRetry(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			client := setupEdgeClient(t)
			ctx := context.Background()

			record, err := client.Lot.Create().
				SetLotNumber(fmt.Sprintf("LOT-AUTH-%d", code)).
				SetProductType("FDY").
				SetPlannedQuantity(1).
				Save(ctx)
			if err != nil {
				t.Fatalf("failed creating lot: %v", err)
			}

			srv, _ := newCenterStub(t, code, map[string]interface{}{})

			u := edge.NewUploader(client, "edge-001", srv.URL, "test-token")

			if _, err := u.Upload(ctx); err == nil {
				t.Fatal("Upload 应当返回错误")
			}

			got, err := client.Lot.Get(ctx, record.ID)
			if err != nil {
				t.Fatalf("failed reloading lot: %v", err)
			}
			if got.SyncStatus != lot.SyncStatusPending {
				t.Errorf("凭证失败后 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
			}
			if got.SyncRetryCount != 0 {
				t.Errorf("凭证失败后 SyncRetryCount = %d, want 0（不得消耗重试次数）", got.SyncRetryCount)
			}
		})
	}
}

// 采集到的表必须被标记，不能有表被采集却永远停在 pending。
//
// 原用例的前提是「存在未被采集的表」（当时是 barrels/cartons），全部表接入后
// 该前提不复存在。这里改为正向断言：Upload 成功后，被采集的纸箱必须进入
// synced —— 若 markSynced 的 switch 缺分支，它会停在 pending 并被反复重传。
//
// 批次与落纱同样被采集，这里只钉纸箱：它是本用例新增的那张表。
func TestUploader_CollectedTablesGetMarked(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-KEEP").
		SetProductType("DTY").
		SetPlannedQuantity(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	cartonRow, err := client.Carton.Create().
		SetCartonNumber("CARTON-KEEP").
		SetLotID(record.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating carton: %v", err)
	}

	// 三条待同步记录（lot + carton + doffing），桩回 applied=3 以匹配批次。
	if _, err := client.Doffing.Create().
		SetSpinningLineID(uuid.New()).
		SetSpinningPosition(1).
		SetLotID(record.ID).
		Save(ctx); err != nil {
		t.Fatalf("failed creating doffing: %v", err)
	}

	srv, _ := newCenterStub(t, http.StatusOK, map[string]interface{}{
		"applied": 3, "rejected": 0, "errors": []string{},
	})

	u := edge.NewUploader(client, "edge-001", srv.URL, "test-token")

	if _, err := u.Upload(ctx); err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	got, err := client.Carton.Get(ctx, cartonRow.ID)
	if err != nil {
		t.Fatalf("failed reloading carton: %v", err)
	}
	if got.SyncStatus != carton.SyncStatusSynced {
		t.Errorf("carton SyncStatus = %q, want %q（被采集的表必须有标记分支）",
			got.SyncStatus, carton.SyncStatusSynced)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("carton SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}
