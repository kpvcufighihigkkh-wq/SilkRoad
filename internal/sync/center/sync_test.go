package center_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/sync/center"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
	"github.com/google/uuid"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

func setupTestClient(t *testing.T) *ent.Client {
	// 使用测试PostgreSQL数据库
	dsn := "postgres://igh:igh@localhost:5432/igh_test?sslmode=disable"
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		t.Skip("PostgreSQL not available, skipping test")
	}

	// 创建schema
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

func TestUploadHandler_HandleUpload(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// 先创建依赖的Project
	project, err := client.Project.Create().
		SetProjectNumber("PROJ-TEST-001").
		SetProjectName("Test Project").
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetStatus("in_progress").
		SetPlannedQuantity(5000).
		Save(ctx)

	if err != nil {
		t.Fatalf("failed creating test project: %v", err)
	}

	// 创建依赖的Order
	order, err := client.Order.Create().
		SetOrderNumber("ORD-TEST-001").
		SetProjectID(project.ID).
		SetCustomerName("Test Customer").
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetOrderQuantity(1000).
		SetStatus("pending").
		Save(ctx)

	if err != nil {
		t.Fatalf("failed creating test order: %v", err)
	}

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
					"order_id":         order.ID.String(),
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
		SetPositionCount(48).
		SetStatus("active").
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
