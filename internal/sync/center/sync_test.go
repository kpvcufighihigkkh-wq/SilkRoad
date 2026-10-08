package center_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
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

// 全零 UUID 也必须被拒 —— 它「能解析」，但不是有效身份。
//
// uuid.Parse("00000000-0000-0000-0000-000000000000") 返回 err == nil 且值为
// uuid.Nil，因此只检查解析错误拦不住它。放行后会命中 createLot 里的
// `if edgeID != uuid.Nil`，跳过 SetEdgeID，仍然写出 edge_id = NULL 并返回
// applied: N —— 与伪造身份在观测上无法区分。这是非 UUID 用例的另一半。
func TestUploadHandler_RejectsNilUUIDEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()

	ctx := context.Background()
	handler := center.NewUploadHandler(client)

	lotID := uuid.New()
	req := &models.UploadRequest{
		EdgeID: uuid.Nil.String(), // 全零，可被 Parse 接受
		Entries: []models.UploadEntry{
			{
				Table:     "lots",
				Operation: "create",
				ID:        lotID,
				Data: map[string]interface{}{
					"id":               lotID.String(),
					"lot_number":       "LOT-NIL-EDGE",
					"product_type":     "FDY",
					"planned_quantity": 1,
					"status":           "in_progress",
				},
			},
		},
	}

	if _, err := handler.HandleUpload(ctx, req); err == nil {
		t.Fatal("全零 UUID 的 EdgeID 应当返回错误")
	}

	if exists, err := client.Lot.Query().Where(lot.IDEQ(lotID)).Exist(ctx); err != nil {
		t.Fatalf("failed querying lot: %v", err)
	} else if exists {
		t.Error("EdgeID 为空 UUID 时记录仍被写入（edge_id 会是 NULL）")
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
					"lot_number":        "LOT-ORDER-001", // 父行解析依据（本任务起 lot_id 不再被采信）
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

// 重传同一批两次：Center 不得出现重复行，且字段应被更新
func TestUpsertLot_RepublishUpdatesInPlace(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	handler := center.NewUploadHandler(client)
	lotID := uuid.New()

	mk := func(status string) *models.UploadRequest {
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "lots", Operation: "create", ID: lotID,
				Data: map[string]interface{}{
					"id": lotID.String(), "lot_number": "LOT-UPSERT-1",
					"product_type": "FDY", "planned_quantity": 10,
					"actual_quantity": 0, "status": status,
				},
			}},
		}
	}

	if _, err := handler.HandleUpload(ctx, mk("in_progress")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, mk("completed")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	n, err := client.Lot.Query().Where(lot.LotNumberEQ("LOT-UPSERT-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（重传应更新而非新建）", n)
	}

	got, err := client.Lot.Get(ctx, lotID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Status) != "completed" {
		t.Errorf("Status = %q, want %q（重传应更新字段）", got.Status, "completed")
	}
}

// Center 已存在同编号但不同 UUID 的行 → 应认领该行，不新建、不报错
func TestUpsertLot_ClaimsExistingByLotNumber(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	// Center 侧先存在一行（模拟人工/历史创建），ID 与 Edge 本地不同
	existing, err := client.Lot.Create().
		SetLotNumber("LOT-CLAIM-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	edgeLocalID := uuid.New() // Edge 本地是另一个 UUID
	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "lots", Operation: "create", ID: edgeLocalID,
			Data: map[string]interface{}{
				"id": edgeLocalID.String(), "lot_number": "LOT-CLAIM-1",
				"product_type": "FDY", "planned_quantity": 20,
				"actual_quantity": 0, "status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	n, err := client.Lot.Query().Where(lot.LotNumberEQ("LOT-CLAIM-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（应认领既有行，不得新建重复）", n)
	}

	got, err := client.Lot.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if got.PlannedQuantity != 20 {
		t.Errorf("PlannedQuantity = %d, want 20（认领后应更新该行）", got.PlannedQuantity)
	}
}

// 重传同一丝饼两次：Center 不得出现重复行，且字段应被更新。
//
// 与 TestUpsertLot_RepublishUpdatesInPlace 同构，覆盖 upsertBobbin 的
// 「先按 ID 命中再更新」分支 —— Task 5 只测了 bobbin 的插入（解引用）路径，
// 该更新分支否则无测试。
//
// 载荷刻意同时带 lot_id（Center 行的真 ID）与 lot_number：Task 5 会把 lot_id
// 改为按 lot_number 解引用，带上 lot_number 后本用例在改写前后都成立。
func TestUpsertBobbin_RepublishUpdatesInPlace(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-BOBBIN-UPSERT-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	handler := center.NewUploadHandler(client)
	bobbinID := uuid.New()

	mk := func(status string) *models.UploadRequest {
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "bobbins", Operation: "create", ID: bobbinID,
				Data: map[string]interface{}{
					"id":                bobbinID.String(),
					"bobbin_number":     "BOBBIN-UPSERT-1",
					"lot_id":            centerLot.ID.String(),
					"lot_number":        "LOT-BOBBIN-UPSERT-1",
					"spinning_position": 1,
					"gross_weight":      5.2,
					"net_weight":        4.8,
					"status":            status,
				},
			}},
		}
	}

	if _, err := handler.HandleUpload(ctx, mk("producing")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, mk("completed")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	n, err := client.Bobbin.Query().Where(bobbin.BobbinNumberEQ("BOBBIN-UPSERT-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（重传应更新而非新建）", n)
	}

	got, err := client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Status) != "completed" {
		t.Errorf("Status = %q, want %q（重传应更新字段）", got.Status, "completed")
	}
}

// 载荷**不含** grade/tare_weight 键时，重传不写入这两个字段，故 Center 既有的值保留。
//
// 精确契约：本用例只钉「键缺失 ⇒ 不写」（Task 3 的载荷省略未设字段 + Nillable
// setter 的 `if v != nil` no-op）。它**不**主张「重传永远不会抹掉既有值」——
// 若载荷显式带空值，行为另见 TestUpsertBobbin_EmptyGradeClearsStoredValue。
// 若有人把 Nillable setter 换成非 Nillable（空值即清空）或加上 Clear*，本用例必须失败。
func TestUpsertBobbin_AbsentGradeLeavesStoredValue(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-BOBBIN-GRADE-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	handler := center.NewUploadHandler(client)
	bobbinID := uuid.New()

	withGrade := &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "bobbins", Operation: "create", ID: bobbinID,
			Data: map[string]interface{}{
				"id":                bobbinID.String(),
				"bobbin_number":     "BOBBIN-GRADE-1",
				"lot_id":            centerLot.ID.String(),
				"lot_number":        "LOT-BOBBIN-GRADE-1",
				"spinning_position": 1,
				"gross_weight":      5.2,
				"net_weight":        4.8,
				"tare_weight":       0.4,
				"grade":             "A",
				"status":            "completed",
			},
		}},
	}

	// 第二次上传：载荷不含 grade/tare_weight（模拟 Task 3 省略未设字段）
	withoutGrade := &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "bobbins", Operation: "create", ID: bobbinID,
			Data: map[string]interface{}{
				"id":                bobbinID.String(),
				"bobbin_number":     "BOBBIN-GRADE-1",
				"lot_id":            centerLot.ID.String(),
				"lot_number":        "LOT-BOBBIN-GRADE-1",
				"spinning_position": 1,
				"gross_weight":      5.2,
				"net_weight":        4.8,
				"status":            "completed",
			},
		}},
	}

	if _, err := handler.HandleUpload(ctx, withGrade); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, withoutGrade); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	got, err := client.Bobbin.Query().
		Where(bobbin.IDEQ(bobbinID)).
		Select(bobbin.FieldGrade, bobbin.FieldTareWeight).
		Only(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Grade != "A" {
		t.Errorf("Grade = %q, want \"A\"（载荷缺该键时应保留既有值）", got.Grade)
	}
	if got.TareWeight != 0.4 {
		t.Errorf("TareWeight = %v, want 0.4（载荷缺该键时应保留既有值）", got.TareWeight)
	}
}

// edgeRowID 创建（或复用）一条 Edge 行并返回其 ID。
// 上传路径要求 req.EdgeID 是设备行的 UUID。
func edgeRowID(t *testing.T, client *ent.Client) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	if row, err := client.Edge.Query().First(ctx); err == nil {
		return row.ID
	}
	row, err := client.Edge.Create().
		SetEdgeCode("edge-upsert-test").
		SetEdgeName("测试设备").
		SetIPAddress("192.168.2.84").
		Save(ctx)
	if err != nil {
		t.Fatalf("create edge: %v", err)
	}
	return row.ID
}

// 认领既有行时不得覆盖其来源设备（edge_id 是历史事实，不是当前归属）。
//
// 场景：Center 已有一行 LOT-CLAIM-EDGE-1，来源为设备 A（cmd/seed 或人工插入
// 会 SetEdgeID）。设备 B 上线后上传同编号的批次 —— resolveLot 按 ID 未命中、
// 按编号命中并认领该行。若 updateLotRow 无条件写 SetEdgeID(edgeID)，
// edge_id 会被改写成 B，而 LotService.ListLots 与 Center 批次列表的
// ?edge_id= 过滤直接依赖它，运维按设备筛选批次即会看到 A 的记录挂在 B 名下。
func TestUpsertLot_ClaimKeepsExistingEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	deviceA, err := client.Edge.Create().
		SetEdgeCode("edge-claim-origin").
		SetEdgeName("原产设备").
		SetIPAddress("192.168.2.90").
		Save(ctx)
	if err != nil {
		t.Fatalf("create device A: %v", err)
	}
	deviceB, err := client.Edge.Create().
		SetEdgeCode("edge-claim-newcomer").
		SetEdgeName("新上线设备").
		SetIPAddress("192.168.2.91").
		Save(ctx)
	if err != nil {
		t.Fatalf("create device B: %v", err)
	}

	// Center 侧既有行：同编号、来源设备 A、ID 与 Edge 本地不同
	existing, err := client.Lot.Create().
		SetLotNumber("LOT-CLAIM-EDGE-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		SetEdgeID(deviceA.ID).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	edgeLocalID := uuid.New()
	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: deviceB.ID.String(), // 上传者是设备 B
		Entries: []models.UploadEntry{{
			Table: "lots", Operation: "create", ID: edgeLocalID,
			Data: map[string]interface{}{
				"id": edgeLocalID.String(), "lot_number": "LOT-CLAIM-EDGE-1",
				"product_type": "FDY", "planned_quantity": 20,
				"actual_quantity": 0, "status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Lot.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if got.EdgeID != deviceA.ID {
		t.Errorf("EdgeID = %v, want %v（认领不得覆盖既有来源设备）", got.EdgeID, deviceA.ID)
	}
	if got.EdgeID == deviceB.ID {
		t.Error("EdgeID 被改写成上传者 B —— edge_id 是历史来源，不得覆盖")
	}
	// 认领本身仍须生效（字段被更新）
	if got.PlannedQuantity != 20 {
		t.Errorf("PlannedQuantity = %d, want 20（认领后应更新该行）", got.PlannedQuantity)
	}
}

// 认领的行若原本没有来源设备，应补上权威身份 —— 与上一个用例互补。
//
// 历史/人工创建的行（无 edge_id）在设备接管后应被归属到该设备，
// 但仅限「原本为空」的情形。
func TestUpsertLot_ClaimStampsNilEdgeID(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	// Center 侧既有行：同编号、**无** edge_id、ID 与 Edge 本地不同
	existing, err := client.Lot.Create().
		SetLotNumber("LOT-CLAIM-NOEDGE-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if existing.EdgeID != uuid.Nil {
		t.Fatalf("前提不成立：seed 行的 EdgeID 应为 uuid.Nil，实际 %v", existing.EdgeID)
	}

	device := edgeRowID(t, client)
	edgeLocalID := uuid.New()
	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: device.String(),
		Entries: []models.UploadEntry{{
			Table: "lots", Operation: "create", ID: edgeLocalID,
			Data: map[string]interface{}{
				"id": edgeLocalID.String(), "lot_number": "LOT-CLAIM-NOEDGE-1",
				"product_type": "FDY", "planned_quantity": 20,
				"actual_quantity": 0, "status": "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Lot.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if got.EdgeID != device {
		t.Errorf("EdgeID = %v, want %v（认领无来源的行时应补上权威身份）", got.EdgeID, device)
	}
}

// 丝饼按业务编号认领：Center 已有同编号但不同 UUID 的行 → 认领该行、不新建。
//
// 与 lot 侧的 TestUpsertLot_ClaimsExistingByLotNumber 同构，覆盖
// resolveBobbin 的按编号命中分支（handler.go 的 by-number 查询）——
// TestUpsertBobbin_RepublishUpdatesInPlace 复用同一个 ID，只走到 ID 命中。
func TestUpsertBobbin_ClaimsExistingByBobbinNumber(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-BOBBIN-CLAIM-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	// Center 侧已有丝饼：同编号、不同 UUID、毛重 5.0
	existing, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-CLAIM-1").
		SetLotID(centerLot.ID).
		SetSpinningPosition(1).
		SetGrossWeight(5.0).
		SetNetWeight(4.6).
		SetStatus(bobbin.StatusProducing).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed bobbin: %v", err)
	}

	edgeLocalID := uuid.New() // Edge 本地是另一个 UUID
	handler := center.NewUploadHandler(client)
	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "bobbins", Operation: "create", ID: edgeLocalID,
			Data: map[string]interface{}{
				"id":                edgeLocalID.String(),
				"bobbin_number":     "BOBBIN-CLAIM-1",
				"lot_id":            centerLot.ID.String(),
				"lot_number":        "LOT-BOBBIN-CLAIM-1",
				"spinning_position": 1,
				"gross_weight":      5.5,
				"net_weight":        5.1,
				"status":            "completed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	n, err := client.Bobbin.Query().Where(bobbin.BobbinNumberEQ("BOBBIN-CLAIM-1")).Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("行数 = %d, want 1（应认领既有行，不得新建重复）", n)
	}

	// 认领的是 Center 既有行（保留其 ID），且字段被更新
	got, err := client.Bobbin.Get(ctx, existing.ID)
	if err != nil {
		t.Fatalf("get existing: %v", err)
	}
	if got.GrossWeight != 5.5 {
		t.Errorf("GrossWeight = %v, want 5.5（认领后应更新该行）", got.GrossWeight)
	}
	// 未命中 Edge 本地 ID —— 证明走的是按编号认领而非按 ID
	if _, err := client.Bobbin.Get(ctx, edgeLocalID); err == nil {
		t.Error("以 Edge 本地 UUID 新建了行 —— 应按编号认领既有行")
	}
}

// 载荷**显式带** grade:""（键存在、值为空串）时，覆盖函数把它解析为
// 非 nil 的 *string("")，故 SetNillableGrade 会写入并清空既有值。
//
// 这是对 TestUpsertBobbin_AbsentGradeLeavesStoredValue 的补充：那个用例只钉
// 「键缺失 ⇒ 不写」；本用例钉「键存在且为空 ⇒ 视为清空」。二者共同界定
// Nillable setter 的实际语义，避免「重传不会抹掉既有值」被误解为无条件成立。
func TestUpsertBobbin_EmptyGradeClearsStoredValue(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-BOBBIN-GRADE-CLEAR-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	handler := center.NewUploadHandler(client)
	bobbinID := uuid.New()

	mk := func(grade interface{}) *models.UploadRequest {
		data := map[string]interface{}{
			"id":                bobbinID.String(),
			"bobbin_number":     "BOBBIN-GRADE-CLEAR-1",
			"lot_id":            centerLot.ID.String(),
			"lot_number":        "LOT-BOBBIN-GRADE-CLEAR-1",
			"spinning_position": 1,
			"gross_weight":      5.2,
			"net_weight":        4.8,
			"status":            "completed",
		}
		if grade != nil {
			data["grade"] = grade
		}
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "bobbins", Operation: "create", ID: bobbinID, Data: data,
			}},
		}
	}

	// 第一次：带 grade "A"
	if _, err := handler.HandleUpload(ctx, mk("A")); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	// 第二次：显式带空串（键存在、值为 ""）
	if _, err := handler.HandleUpload(ctx, mk("")); err != nil {
		t.Fatalf("second upload: %v", err)
	}

	got, err := client.Bobbin.Query().
		Where(bobbin.IDEQ(bobbinID)).
		Select(bobbin.FieldGrade).
		Only(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Grade != "" {
		t.Errorf("Grade = %q, want \"\"（载荷显式传空串应清空既有值）", got.Grade)
	}
}

// 载荷里的 lot_id 是 Edge 本地 UUID，Center 必须按 lot_number 解引用，
// 把子记录挂到 Center 自己的批次行上。
func TestUploadBobbin_DereferencesParentByLotNumber(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	// Center 侧已有的批次（UUID 与 Edge 本地不同）
	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-DEREF-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	edgeLocalLotID := uuid.New() // Edge 本地的批次 UUID，Center 不认识
	bobbinID := uuid.New()

	handler := center.NewUploadHandler(client)
	upload := func() *models.UploadResponse {
		resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "bobbins", Operation: "create", ID: bobbinID,
				Data: map[string]interface{}{
					"id":                bobbinID.String(),
					"bobbin_number":     "BOBBIN-DEREF-1",
					"lot_id":            edgeLocalLotID.String(), // 陷阱：会被忽略
					"lot_number":        "LOT-DEREF-1",           // 真依据
					"spinning_position": 1,
					"gross_weight":      1.5,
					"net_weight":        1.2,
					"status":            "producing",
				},
			}},
		})
		if err != nil {
			t.Fatalf("HandleUpload: %v", err)
		}
		return resp
	}

	resp := upload()
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		t.Fatalf("get bobbin: %v", err)
	}
	if got.LotID != centerLot.ID {
		t.Errorf("LotID = %v, want %v（必须按编号解引用到 Center 的批次，而非采信载荷的 lot_id）",
			got.LotID, centerLot.ID)
	}
	if got.LotID == edgeLocalLotID {
		t.Error("采信了载荷中的 lot_id —— 这是安全缺陷")
	}

	// 第二遍：同一载荷重传。此时 Center 已存在该 bobbin_number 的行，
	// upsertBobbin 走认领路径转 updateBobbinRow —— 该路径也必须按编号解引用，
	// 否则会把子记录挂回不存在的 edgeLocalLotID（外键失败）或挂错父行。
	resp2 := upload()
	if resp2.Applied != 1 {
		t.Fatalf("second pass applied = %d, want 1; errors=%v", resp2.Applied, resp2.Errors)
	}
	got2, err := client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		t.Fatalf("get bobbin after second pass: %v", err)
	}
	if got2.LotID != centerLot.ID {
		t.Errorf("第二遍后 LotID = %v, want %v（认领路径 updateBobbinRow 也必须按编号解引用）",
			got2.LotID, centerLot.ID)
	}
}

// 落纱上传：按 lot_number 挂到 Center 的批次，且状态与重量落库
func TestUpsertDoffing_PersistsWithDereferencedLot(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	centerLot, err := client.Lot.Create().
		SetLotNumber("LOT-DOFF-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	lineID := uuid.New()
	doffingID := uuid.New()
	handler := center.NewUploadHandler(client)

	resp, err := handler.HandleUpload(ctx, &models.UploadRequest{
		EdgeID: edgeRowID(t, client).String(),
		Entries: []models.UploadEntry{{
			Table: "doffings", Operation: "create", ID: doffingID,
			Data: map[string]interface{}{
				"id":                doffingID.String(),
				"lot_id":            uuid.New().String(), // 陷阱：忽略
				"lot_number":        "LOT-DOFF-1",
				"spinning_line_id":  lineID.String(),
				"spinning_position": 3,
				"bobbin_number":     "BOBBIN-DOFF-1",
				"actual_weight":     1.75,
				"grade":             "B",
				"status":            "confirmed",
			},
		}},
	})
	if err != nil {
		t.Fatalf("HandleUpload: %v", err)
	}
	if resp.Applied != 1 {
		t.Fatalf("applied = %d, want 1; errors=%v", resp.Applied, resp.Errors)
	}

	got, err := client.Doffing.Get(ctx, doffingID)
	if err != nil {
		t.Fatalf("get doffing: %v", err)
	}
	if got.LotID != centerLot.ID {
		t.Errorf("LotID = %v, want %v", got.LotID, centerLot.ID)
	}
	if string(got.Status) != "confirmed" {
		t.Errorf("Status = %q, want %q", got.Status, "confirmed")
	}
	if got.ActualWeight != 1.75 {
		t.Errorf("ActualWeight = %v, want 1.75", got.ActualWeight)
	}
	if got.Grade != "B" {
		t.Errorf("Grade = %q, want %q", got.Grade, "B")
	}
}

// 落纱状态流转：同一落纱重传两次，第二次应更新状态而非新建
func TestUpsertDoffing_SecondUploadUpdatesStatus(t *testing.T) {
	client := setupTestClient(t)
	defer client.Close()
	ctx := context.Background()

	if _, err := client.Lot.Create().
		SetLotNumber("LOT-DOFF-2").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx); err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	doffingID := uuid.New()
	handler := center.NewUploadHandler(client)
	mk := func(status string) *models.UploadRequest {
		return &models.UploadRequest{
			EdgeID: edgeRowID(t, client).String(),
			Entries: []models.UploadEntry{{
				Table: "doffings", Operation: "create", ID: doffingID,
				Data: map[string]interface{}{
					"id": doffingID.String(), "lot_number": "LOT-DOFF-2",
					"spinning_line_id": uuid.New().String(), "spinning_position": 1,
					"status": status,
				},
			}},
		}
	}

	if _, err := handler.HandleUpload(ctx, mk("pending")); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := handler.HandleUpload(ctx, mk("cancelled")); err != nil {
		t.Fatalf("second: %v", err)
	}

	n, err := client.Doffing.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("落纱行数 = %d, want 1（状态流转应更新同一行）", n)
	}
	got, err := client.Doffing.Get(ctx, doffingID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if string(got.Status) != "cancelled" {
		t.Errorf("Status = %q, want %q", got.Status, "cancelled")
	}
}
