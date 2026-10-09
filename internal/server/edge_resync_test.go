package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

// newResyncTestServer 造一个只挂了 /v1/sync/resync 的最小 Edge 服务器。
func newResyncTestServer(t *testing.T, client *ent.Client) (*gin.Engine, string) {
	t.Helper()

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})

	svr := &EdgeServer{
		client:   client,
		edgeID:   "edge-001",
		uploader: edge.NewUploader(client, "edge-001", "http://center.invalid", "test-token"),
		jwtAuth:  jwtAuth,
	}

	router := gin.New()
	router.POST("/v1/sync/resync", svr.jwtMiddleware(), svr.handleSyncResync)

	token, err := jwtAuth.GenerateToken("edge-user-id", "edge", []string{"operator"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	return router, token
}

func postResync(t *testing.T, router *gin.Engine, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/sync/resync", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// mode=failed 必须把 failed 记录翻回 pending 并归还重试预算。
//
// 这正是本端点存在的理由：failed 此前是数据终点，孤儿 bobbin_grades
// （父丝饼从未同步）耗尽 5 次重试后即永久滞留，只能人工改库。
func TestHandleSyncResync_FailedModeRecoversOrphanGrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()
	router, token := newResyncTestServer(t, client)

	// 孤儿质检记录：bobbin_id 指向一个不存在的丝饼，这正是它同步失败的原因
	grade, err := client.BobbinGrade.Create().
		SetBobbinID(uuid.New()).
		SetGrade("A").
		SetSyncStatus(bobbingrade.SyncStatusFailed).
		SetSyncRetryCount(5).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed failed bobbin grade: %v", err)
	}

	w := postResync(t, router, token, `{"mode":"failed","table":"bobbin_grades"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Reset int    `json:"reset"`
			Mode  string `json:"mode"`
			Table string `json:"table"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response %s: %v", w.Body.String(), err)
	}
	if body.Code != 0 {
		t.Errorf("响应的 code = %d, want 0", body.Code)
	}
	if body.Data.Reset != 1 {
		t.Errorf("响应的 reset = %d, want 1（不得是硬编码值）", body.Data.Reset)
	}

	got, err := client.BobbinGrade.Get(ctx, grade.ID)
	if err != nil {
		t.Fatalf("reload bobbin grade: %v", err)
	}
	if got.SyncStatus != bobbingrade.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, bobbingrade.SyncStatusPending)
	}
	if got.SyncRetryCount != 0 {
		t.Errorf("SyncRetryCount = %d, want 0", got.SyncRetryCount)
	}
}

// mode=all 重置 synced 记录（载荷新增字段后的存量回填）。
// table 省略时作用于全部可采集表：这里的 lots 与 bobbin_grades 都应被重置。
func TestHandleSyncResync_AllModeResetsEveryCollectableTable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()
	router, token := newResyncTestServer(t, client)

	row, err := client.Lot.Create().
		SetLotNumber("LOT-RESYNC-ENDPOINT").
		SetProductType("FDY").
		SetPlannedQuantity(1).
		SetSyncStatus(lot.SyncStatusSynced).
		Save(ctx)
	if err != nil {
		t.Fatalf("seed synced lot: %v", err)
	}

	// table 省略 → 全部可采集表
	w := postResync(t, router, token, `{"mode":"all"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	got, err := client.Lot.Get(ctx, row.ID)
	if err != nil {
		t.Fatalf("reload lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusPending {
		t.Errorf("SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusPending)
	}
	if !got.SyncedAt.IsZero() {
		t.Errorf("SyncedAt 应被清空，实际 %v", got.SyncedAt)
	}
}

// 非法 mode 与未知表名都必须被拒绝，而不是静默成功。
//
// gin 的 oneof 校验会把非法 mode 拦成 400；未知表名由 uploader 抛出
// ErrUnknownTable，handler 转成 400（客户端错误，不是 500）——
// 两条路径都不能回 200。
func TestHandleSyncResync_RejectsBadInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	router, token := newResyncTestServer(t, client)

	cases := []struct {
		name string
		body string
		want int
	}{
		// 表名是调用方给的，非法值属客户端错误。返 500 会污染错误率看板。
		{"未知表名", `{"mode":"failed","table":"barrels"}`, http.StatusBadRequest},
		{"非法 mode", `{"mode":"everything"}`, http.StatusBadRequest},
		{"缺少 mode", `{"table":"lots"}`, http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := postResync(t, router, token, tc.body)
			if w.Code != tc.want {
				t.Errorf("status = %d, want %d; body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

// 未知表名的错误消息必须列出合法表名 —— 对已鉴权的运维这是「告诉我合法值」。
//
// 可证伪：从错误消息里删掉 collectableTables(u) 那段，本用例必红。
func TestHandleSyncResync_UnknownTableListsValidTables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	router, token := newResyncTestServer(t, client)

	w := postResync(t, router, token, `{"mode":"failed","table":"barrels"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, table := range []string{"lots", "doffings", "bobbins", "bobbin_grades"} {
		if !strings.Contains(body, table) {
			t.Errorf("错误消息未列出合法表名 %q；body=%s", table, body)
		}
	}
}

// 端点必须受 JWT 保护：无 token 不得放行。
func TestHandleSyncResync_RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	router, _ := newResyncTestServer(t, client)

	req := httptest.NewRequest(http.MethodPost, "/v1/sync/resync", strings.NewReader(`{"mode":"failed"}`))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}
