package edge_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

// Upload 必须无条件把「pending 且计数已达上限」的行翻成 failed。
//
// 这是一条可达的永久卡死路径：采集器的过滤是
// `pending` 且 `retry_count < 上限`，计数已达上限的行被它排除在外；
// 而 markExhausted 此前只在 markRetried 的末尾可达，markRetried 又只在
// 本批**存在重试项**时才被调用 —— 批次为空在标记之前就 return，
// 整批成功则走 markSynced。于是这样一行既不被采集、也不被翻成 failed，
// 调度器每 tick 拿到空批次，记录永久停在 pending 且无人可见。
//
// 可证伪：删掉 Upload 里那次 markExhausted 调用，本用例必红。
func TestUpload_MarksExhaustedPendingRowsOnEmptyBatch(t *testing.T) {
	client := setupEdgeClient(t)
	ctx := context.Background()

	// 直接播种该状态，不走 markRetried：要验证的是 Upload 自身的可达性，
	// 不是计数如何涨到上限。
	row, err := client.Lot.Create().
		SetLotNumber("LOT-EXHAUSTED-PENDING").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncStatus(lot.SyncStatusPending).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed pending-at-cap lot: %v", err)
	}

	// 本批为空：库里没有其它 pending 行，而这一行被计数过滤挡在采集之外。
	u := edge.NewUploader(client, "edge-001", "http://center.invalid", "token")

	if _, err := u.Upload(ctx); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("reload lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusFailed {
		t.Errorf("计数已达上限的 pending 行 SyncStatus = %q, want %q（否则永久卡死且无人可见）",
			got.SyncStatus, lot.SyncStatusFailed)
	}
}
