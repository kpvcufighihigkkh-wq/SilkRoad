package edge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/grade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/spinningline"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// newBaseDataStub 起一个假的 Center base-data 端点。
//
// 响应形状与真实端点一致：api.Success(data) 的封装是
// {"code":0,"message":"success","data":{...}}，而 base-data 的 data 本身
// 又是 BaseDataPullResponse（含一个 "data" 字段），故载荷是两层嵌套。
func newBaseDataStub(t *testing.T, payload models.BaseDataPullResponse) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query()["tables"]; len(got) != 2 {
			t.Errorf("tables 参数 = %v, 期望拉取 2 张表", got)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0, "message": "success", "data": payload,
		}); err != nil {
			t.Errorf("stub encode failed: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func spinningLineRow(id uuid.UUID, name, number, status string, capacity int) map[string]interface{} {
	return map[string]interface{}{
		"id":          id.String(),
		"line_name":   name,
		"line_number": number,
		"status":      status,
		"capacity":    float64(capacity), // JSON 数字解码后是 float64
	}
}

// 下载必须沿用 Center 的 UUID。
//
// 这是本链路的全部意义：Center 的报表按 doffing.spinning_line_id 关联线体
// 名称，而该字段是 Edge 原样透传上去的。若本地另生成 ID，两边永远对不上。
func TestSyncClient_DownloadData_PreservesCenterIDs(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	lineID := uuid.New()
	gradeID := uuid.New()

	srv := newBaseDataStub(t, models.BaseDataPullResponse{
		Data: map[string][]map[string]interface{}{
			"spinning_lines": {spinningLineRow(lineID, "A线", "LINE-A", "running", 48)},
			"grades": {{
				"id": gradeID.String(), "grade_type": "sorting",
				"grade_code": "A", "grade_name": "优等品",
				"sort_order": float64(1), "is_active": true,
			}},
		},
	})

	svc := NewSyncClient(client, "edge-001", srv.URL, "test-token")
	result, err := svc.DownloadData(ctx)
	if err != nil {
		t.Fatalf("DownloadData: %v", err)
	}

	if result.Downloaded != 2 || result.Applied != 2 {
		t.Errorf("result = %+v, want downloaded=2 applied=2", result)
	}

	line, err := client.SpinningLine.Get(ctx, lineID)
	if err != nil {
		t.Fatalf("按 Center 的 ID 查不到线体: %v", err)
	}
	if line.LineName != "A线" || line.LineNumber != "LINE-A" {
		t.Errorf("线体字段 = %+v", line)
	}
	if line.Capacity != 48 {
		t.Errorf("capacity = %d, want 48", line.Capacity)
	}
	if line.Status != spinningline.StatusRunning {
		t.Errorf("status = %q, want running", line.Status)
	}

	g, err := client.Grade.Get(ctx, gradeID)
	if err != nil {
		t.Fatalf("按 Center 的 ID 查不到等级: %v", err)
	}
	if g.GradeName != "优等品" || g.GradeCode != "A" {
		t.Errorf("等级字段 = %+v", g)
	}
}

// 重复下载不得产生重复行，也不得新增 ID。
func TestSyncClient_DownloadData_IsIdempotent(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	lineID := uuid.New()
	payload := models.BaseDataPullResponse{
		Data: map[string][]map[string]interface{}{
			"spinning_lines": {spinningLineRow(lineID, "A线", "LINE-A", "running", 48)},
		},
	}
	srv := newBaseDataStub(t, payload)
	svc := NewSyncClient(client, "edge-001", srv.URL, "test-token")

	for i := 0; i < 2; i++ {
		if _, err := svc.DownloadData(ctx); err != nil {
			t.Fatalf("第 %d 次 DownloadData: %v", i+1, err)
		}
	}

	n, err := client.SpinningLine.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("线体行数 = %d, want 1（重复下载不得插入重复行）", n)
	}
}

// 同名线体已存在但 ID 不同：保留本地 ID，不建重复行。
//
// 这覆盖「Edge 曾自行建过线体」的配置漂移场景。此时报表关联会失效，
// 代码打告警而不静默处理 —— 但要钉住它不产生重复行。
func TestSyncClient_DownloadData_KeepsLocalIDOnNameCollision(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	localID := uuid.New()
	if _, err := client.SpinningLine.Create().
		SetID(localID).
		SetLineName("A线").
		Save(ctx); err != nil {
		t.Fatalf("seed local line: %v", err)
	}

	srv := newBaseDataStub(t, models.BaseDataPullResponse{
		Data: map[string][]map[string]interface{}{
			"spinning_lines": {spinningLineRow(uuid.New(), "A线", "LINE-A", "running", 48)},
		},
	})
	svc := NewSyncClient(client, "edge-001", srv.URL, "test-token")

	if _, err := svc.DownloadData(ctx); err != nil {
		t.Fatalf("DownloadData: %v", err)
	}

	n, err := client.SpinningLine.Query().Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("线体行数 = %d, want 1（同名不得插入重复行）", n)
	}

	line, err := client.SpinningLine.Get(ctx, localID)
	if err != nil {
		t.Fatalf("本地行被替换了 ID: %v", err)
	}
	// 其余字段应被 Center 的值更新。
	if line.LineNumber != "LINE-A" {
		t.Errorf("line_number = %q, want LINE-A", line.LineNumber)
	}
}

// 等级按 (grade_type, grade_code) 定位：同一编码在不同体系里是两条记录。
//
// 单用 grade_code 会让「分拣等级 A」与「外观等级 A」互相覆盖。
func TestSyncClient_DownloadData_GradeKeyedByTypeAndCode(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	sortingID := uuid.New()
	visionID := uuid.New()

	srv := newBaseDataStub(t, models.BaseDataPullResponse{
		Data: map[string][]map[string]interface{}{
			"grades": {
				{"id": sortingID.String(), "grade_type": "sorting", "grade_code": "A",
					"grade_name": "分拣优等", "sort_order": float64(1), "is_active": true},
				{"id": visionID.String(), "grade_type": "vision", "grade_code": "A",
					"grade_name": "外观优等", "sort_order": float64(1), "is_active": true},
			},
		},
	})
	svc := NewSyncClient(client, "edge-001", srv.URL, "test-token")

	if _, err := svc.DownloadData(ctx); err != nil {
		t.Fatalf("DownloadData: %v", err)
	}

	rows, err := client.Grade.Query().Where(grade.GradeCodeEQ("A")).All(ctx)
	if err != nil {
		t.Fatalf("query grades: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("同码不同类别的等级行数 = %d, want 2（不得互相覆盖）", len(rows))
	}
}

// 凭证失败要作为错误返回，而不是静默返回 0 条。
//
// 静默成功会让运维以为「配置本来就为空」，而实际上只有令牌配错了。
func TestSyncClient_DownloadData_RejectsUnauthorized(t *testing.T) {
	client := newInternalEdgeClient(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	svc := NewSyncClient(client, "edge-001", srv.URL, "bad-token")
	if _, err := svc.DownloadData(context.Background()); err == nil {
		t.Fatal("401 应返回错误，got nil")
	}
}

// 未知表要报错：两端对 baseDataTables 的理解不一致时不能静默跳过。
func TestSyncClient_ApplyTable_RejectsUnknownTable(t *testing.T) {
	client := newInternalEdgeClient(t)
	svc := NewSyncClient(client, "edge-001", "http://example.invalid", "t")

	if _, err := svc.applyTable(context.Background(), "unknown", nil); err == nil {
		t.Fatal("未知表应返回错误，got nil")
	}
}

// 落地后的本地线体必须能被 doffing 的 spinning_line_id 关联到 ——
// 这是报表显示机台名称的前提，用一次真实的关联查询钉住。
func TestSyncClient_DownloadData_EnablesDoffingLineJoin(t *testing.T) {
	client := newInternalEdgeClient(t)
	ctx := context.Background()

	lineID := uuid.New()
	srv := newBaseDataStub(t, models.BaseDataPullResponse{
		Data: map[string][]map[string]interface{}{
			"spinning_lines": {spinningLineRow(lineID, "A线", "LINE-A", "running", 48)},
		},
	})

	svc := NewSyncClient(client, "edge-001", srv.URL, "test-token")
	if _, err := svc.DownloadData(ctx); err != nil {
		t.Fatalf("DownloadData: %v", err)
	}

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-JOIN").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed lot: %v", err)
	}
	if _, err := client.Doffing.Create().
		SetLotID(lotRow.ID).
		SetSpinningLineID(lineID).
		SetSpinningPosition(1).
		Save(ctx); err != nil {
		t.Fatalf("seed doffing: %v", err)
	}

	line, err := client.SpinningLine.Get(ctx, lineID)
	if err != nil {
		t.Fatalf("落纱引用的线体查不到: %v", err)
	}
	if line.LineName != "A线" {
		t.Errorf("line_name = %q, want A线", line.LineName)
	}
}

// 编译期确认：测试辅助构造的 ent 客户端可被 NewSyncClient 接受。
var _ = func(c *ent.Client) { NewSyncClient(c, "e", "http://x", "t") }
