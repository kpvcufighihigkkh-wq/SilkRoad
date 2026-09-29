package center_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/sync/center"
	"github.com/yourusername/igh-silkroad/internal/sync/models"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func setupTestClient(t *testing.T) *ent.Client {
	// 使用内存SQLite，每个测试独立数据库名，避免相互污染且不触碰开发库
	db, err := sql.Open("sqlite", "file:synccentertest?mode=memory&cache=shared&_fk=1")
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

func TestUploadHandler_HandleUpload(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	handler := center.NewUploadHandler(client)

	// 构造上传请求
	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: "edge-001",
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-UPLOAD-001",
					"plc_lot_number":   "PLC-UPLOAD-001",
					"product_type":     "FDY",
					"product_spec":     "150D/48F",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
			},
		},
	}

	// 处理上传
	resp, err := handler.HandleUpload(ctx, req)
	if err != nil {
		t.Fatalf("HandleUpload failed: %v", err)
	}

	if resp.Applied != 1 {
		t.Errorf("Expected applied=1, got %d", resp.Applied)
	}

	if resp.Rejected != 0 {
		t.Errorf("Expected rejected=0, got %d (errors: %v)", resp.Rejected, resp.Errors)
	}

	// 验证数据已插入
	lot, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("Failed to retrieve created lot: %v", err)
	}

	if lot.LotNumber != "LOT-UPLOAD-001" {
		t.Errorf("Expected lot_number=LOT-UPLOAD-001, got %s", lot.LotNumber)
	}

	t.Logf("✅ Upload test passed: created lot %s", lot.LotNumber)
}

func TestBaseDataProvider_HandlePullRequest(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// 创建测试数据
	_, err := client.SpinningLine.Create().
		SetLineNumber("L001").
		SetLineName("Line 1").
		SetCapacity(48).
		SetStatus("running").
		Save(ctx)

	if err != nil {
		t.Fatalf("failed creating test spinning line: %v", err)
	}

	provider := center.NewBaseDataProvider(client)

	// 构造拉取请求
	req := &models.BaseDataPullRequest{
		EdgeID: "edge-001",
		Tables: []string{"spinning_lines"},
	}

	// 处理拉取
	resp, err := provider.HandlePullRequest(ctx, req)
	if err != nil {
		t.Fatalf("HandlePullRequest failed: %v", err)
	}

	if len(resp.Data["spinning_lines"]) == 0 {
		t.Error("Expected spinning_lines data, got empty")
	}

	t.Logf("✅ Pull test passed: got %d spinning lines", len(resp.Data["spinning_lines"]))
}
