package center_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
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

	// 构造上传请求。
	//
	// EdgeID 必须是**已注册设备行的 UUID**，不是 edge_code：HandleUpload 用它
	// 作为 lot.edge_id 的外键，且解析失败会让整批被拒（此前是静默退化为
	// uuid.Nil 并照常写入 edge_id = NULL，与身份被伪造无法区分）。
	// 用真实设备行，顺带验证 edge_id 被真正写上而不是 NULL。
	edgeRow, err := client.Edge.Create().
		SetEdgeCode("edge-upload-test").
		SetEdgeName("上传测试设备").
		SetIPAddress("192.168.9.9").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge row: %v", err)
	}

	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: edgeRow.ID.String(),
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
	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("Failed to retrieve created lot: %v", err)
	}

	if got.LotNumber != "LOT-UPLOAD-001" {
		t.Errorf("Expected lot_number=LOT-UPLOAD-001, got %s", got.LotNumber)
	}

	// 权威身份必须真正落到 edge_id 上，而不是 NULL
	if got.EdgeID != edgeRow.ID {
		t.Errorf("lot.edge_id = %v, want %v（权威设备 UUID）", got.EdgeID, edgeRow.ID)
	}

	t.Logf("✅ Upload test passed: created lot %s", got.LotNumber)
}

// 非 UUID 的 EdgeID 必须让整批被拒，而不是静默写入 edge_id = NULL。
//
// 此前 handleUpload 在 Parse 失败时退化为 uuid.Nil 并继续处理，返回 applied: N，
// 记录虽写入了却无法归属到任何设备 —— 与「身份被伪造」在观测上完全一致。
func TestUploadHandler_RejectsNonUUIDEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: "edge-001", // edge_code，不是设备行 UUID
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-BAD-EDGE",
					"product_type":     "FDY",
					"planned_quantity": 1,
					"status":           "in_progress",
				},
			},
		},
	}

	if _, err := handler.HandleUpload(ctx, req); err == nil {
		t.Fatal("非 UUID 的 EdgeID 应当返回错误")
	}

	// 整批拒绝意味着一条都不应落库
	if exists, err := client.Lot.Query().Where(lot.IDEQ(lotID)).Exist(ctx); err != nil {
		t.Fatalf("failed querying lot: %v", err)
	} else if exists {
		t.Error("EdgeID 非法时记录仍被写入")
	}
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

func TestSortEntriesByDependency(t *testing.T) {
	lotID := uuid.New()
	barrelID := uuid.New()
	bobbinID := uuid.New()

	// 故意乱序：bobbin 先于 barrel，barrel 先于 lot
	entries := []models.UploadEntry{
		{Table: "bobbins", Operation: "create", ID: bobbinID},
		{Table: "barrels", Operation: "create", ID: barrelID},
		{Table: "lots", Operation: "create", ID: lotID},
	}

	sorted := center.SortEntriesByDependency(entries)

	want := []string{"lots", "barrels", "bobbins"}
	for i, e := range sorted {
		if e.Table != want[i] {
			t.Errorf("位置 %d = %q, want %q（必须按依赖顺序）", i, e.Table, want[i])
		}
	}
}

func TestSortEntriesByDependency_StableForUnknownTables(t *testing.T) {
	entries := []models.UploadEntry{
		{Table: "unknown_b", Operation: "create", ID: uuid.New()},
		{Table: "lots", Operation: "create", ID: uuid.New()},
		{Table: "unknown_a", Operation: "create", ID: uuid.New()},
	}

	sorted := center.SortEntriesByDependency(entries)

	if sorted[0].Table != "lots" {
		t.Errorf("首个应为 lots，实际 %q", sorted[0].Table)
	}
	// 未知表保持相对顺序
	if sorted[1].Table != "unknown_b" || sorted[2].Table != "unknown_a" {
		t.Errorf("未知表相对顺序应保持，实际 %q, %q", sorted[1].Table, sorted[2].Table)
	}
}

// 载荷中的 edge_id 必须被忽略，使用请求级 EdgeID
func TestCreateLot_IgnoresPayloadEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	authoritativeEdge, err := client.Edge.Create().
		SetEdgeCode("edge-authoritative").
		SetEdgeName("权威设备").
		SetIPAddress("192.168.2.84").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	forgedEdge, err := client.Edge.Create().
		SetEdgeCode("edge-forged").
		SetEdgeName("伪造设备").
		SetIPAddress("10.0.0.1").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating forged edge: %v", err)
	}

	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: authoritativeEdge.ID.String(), // Center 推导出的权威身份
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-FORGERY-001",
					"edge_id":          forgedEdge.ID.String(), // 载荷试图伪造
					"product_type":     "FDY",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
			},
		},
	}

	resp, err := handler.HandleUpload(ctx, req)
	if err != nil {
		t.Fatalf("HandleUpload failed: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("failed retrieving lot: %v", err)
	}

	if got.EdgeID != authoritativeEdge.ID {
		t.Errorf("EdgeID = %v, want %v（必须用权威身份，忽略载荷）",
			got.EdgeID, authoritativeEdge.ID)
	}
	if got.EdgeID == forgedEdge.ID {
		t.Error("载荷中的 edge_id 被采信了 —— 这是安全缺陷")
	}
}

// 载荷完全不含 edge_id 时，仍必须写入鉴权层推导的权威设备 UUID。
//
// 这是 Task 4 审查中暂缓的断言：与 TestCreateLot_IgnoresPayloadEdgeID
// 覆盖的「伪造」场景互补，本用例覆盖「缺失」场景 —— 载荷里根本没有
// edge_id，因此落库的值只可能来自 req.EdgeID。
func TestHandleEdgeUpload_StampsAuthoritativeEdgeUUID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	authoritativeEdge, err := client.Edge.Create().
		SetEdgeCode("edge-absent-payload").
		SetEdgeName("权威设备-载荷缺失").
		SetIPAddress("192.168.2.85").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	data := map[string]interface{}{
		"id":               lotID.String(),
		"lot_number":       "LOT-ABSENT-EDGE-001",
		"product_type":     "FDY",
		"planned_quantity": 100,
		"actual_quantity":  0,
		"status":           "in_progress",
	}

	// 前提断言：载荷必须完全不含 edge_id，否则本用例无法证明
	// 身份来自 req.EdgeID 而非载荷。
	if _, present := data["edge_id"]; present {
		t.Fatal("测试构造错误：data 不得包含 edge_id")
	}

	req := &models.UploadRequest{
		EdgeID: authoritativeEdge.ID.String(), // 鉴权层推导的权威身份
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data:      data,
			},
		},
	}

	resp, err := handler.HandleUpload(ctx, req)
	if err != nil {
		t.Fatalf("HandleUpload failed: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("failed retrieving lot: %v", err)
	}

	if got.EdgeID != authoritativeEdge.ID {
		t.Errorf("EdgeID = %v, want %v（载荷无 edge_id 时仍应落权威身份）",
			got.EdgeID, authoritativeEdge.ID)
	}
	if got.EdgeID == uuid.Nil {
		t.Error("EdgeID 为 uuid.Nil：权威身份未能落到外键上")
	}
}

// 上传批次内若丝锭（bobbins）先于其所属批次（lots）到达，HandleUpload 必须
// 先处理 lots，否则 bobbin 的 lot_id 外键指向一条尚不存在的记录，插入被
// SQLite 外键约束拒绝。
//
// 这是 SortEntriesByDependency 唯一能被端到端观测到的接缝：排序只有在批次
// 含 >= 2 条记录时才可能改变行为，而 handler 目前只支持 lots/bobbins 两张表，
// 故 lots-before-bobbins 是排序当前唯一影响的顺序，也正是边端队列冲刷的
// 真实形态：先落纱出桶（bobbin），再补批次头（lot）。
//
// 若 handler.go 中 HandleUpload 的排序调用被移除，本用例必须失败：bobbin
// 插入会撞上真实外键（setupTestClient 以 _fk=1 打开连接并显式执行
// PRAGMA foreign_keys = ON），applied 退化为 1、rejected 变为 1。
func TestHandleUpload_SortsBobbinsAfterLots(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	authoritativeEdge, err := client.Edge.Create().
		SetEdgeCode("edge-sort-order").
		SetEdgeName("排序验证设备").
		SetIPAddress("192.168.2.86").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	bobbinID := uuid.New()

	// 刻意乱序：bobbin 先于其所属 lot 到达
	req := &models.UploadRequest{
		EdgeID: authoritativeEdge.ID.String(),
		Entries: []models.UploadEntry{
			{
				Table:     "bobbins",
				Operation: "create",
				ID:        bobbinID,
				Data: map[string]interface{}{
					"id":                bobbinID.String(),
					"bobbin_number":     "BOBBIN-ORDER-001",
					"lot_id":            lotID.String(),
					"spinning_position": 1,
					"gross_weight":      5.2,
					"net_weight":        4.8,
					"status":            "producing",
				},
			},
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-ORDER-001",
					"product_type":     "FDY",
					"planned_quantity": 100,
					"actual_quantity":  0,
					"status":           "in_progress",
				},
			},
		},
	}

	resp, err := handler.HandleUpload(ctx, req)
	if err != nil {
		t.Fatalf("HandleUpload failed: %v", err)
	}

	if resp.Applied != 2 {
		t.Errorf("applied = %d, want 2（未排序时 bobbin 先撞 lot_id 外键）; errors=%v",
			resp.Applied, resp.Errors)
	}
	if resp.Rejected != 0 {
		t.Errorf("rejected = %d, want 0; errors=%v", resp.Rejected, resp.Errors)
	}
	if len(resp.Errors) != 0 {
		t.Errorf("Errors 应为空，实际 %v", resp.Errors)
	}

	// 顺带确认外键确实指向了正确的批次
	got, err := client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		t.Fatalf("failed retrieving bobbin: %v", err)
	}
	if got.LotID != lotID {
		t.Errorf("bobbin.LotID = %v, want %v", got.LotID, lotID)
	}
}
