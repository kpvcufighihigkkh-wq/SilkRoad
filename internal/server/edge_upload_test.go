package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

// handleSyncUpload 必须真正驱动上传器并透传其结果。
//
// 接线前该端点固定返回 {"uploaded":0,"failed":0} 且不触碰 Center ——
// 与 Task 4 修掉的 Center 侧「假成功」是同一类缺陷：调用方拿到 200
// 却没有任何记录被送出去。这里断言三件事：请求确实打到了 Center 的
// 上传路径、响应里是真实的 applied 计数、本地记录被标记为 synced。
func TestHandleSyncUpload_DrivesUploader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := newServerTestClient(t)
	ctx := context.Background()

	var gotPath string
	center := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    0,
			"message": "success",
			"data": map[string]interface{}{
				"applied":    1,
				"rejected":   0,
				"errors":     []string{},
				"new_cursor": 0,
			},
		}); err != nil {
			t.Errorf("center stub encode failed: %v", err)
		}
	}))
	defer center.Close()

	record, err := client.Lot.Create().
		SetLotNumber("LOT-EDGE-UPLOAD").
		SetProductType("FDY").
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating lot: %v", err)
	}

	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:      "test-secret",
		ExpireDuration: 1 * time.Hour,
	})

	svr := &EdgeServer{
		client:   client,
		edgeID:   "edge-001",
		uploader: edge.NewUploader(client, "edge-001", center.URL, "test-token"),
		jwtAuth:  jwtAuth,
	}

	router := gin.New()
	router.POST("/v1/sync/upload", svr.jwtMiddleware(), svr.handleSyncUpload)

	token, err := jwtAuth.GenerateToken("edge-user-id", "edge", []string{"operator"})
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/sync/upload", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	// Edge 侧必须按 :code 寻址，缺段会被 Center 的鉴权第三重拒绝
	if gotPath != "/v1/edges/edge-001/upload" {
		t.Errorf("Center 收到路径 = %q, want %q", gotPath, "/v1/edges/edge-001/upload")
	}

	var body struct {
		Code int `json:"code"`
		Data struct {
			Applied  int `json:"applied"`
			Rejected int `json:"rejected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed unmarshalling response %s: %v", w.Body.String(), err)
	}
	if body.Code != 0 {
		t.Errorf("响应的 code = %d, want 0", body.Code)
	}
	if body.Data.Applied != 1 {
		t.Errorf("响应的 applied = %d, want 1（不得是硬编码的 0）", body.Data.Applied)
	}

	got, err := client.Lot.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("failed reloading lot: %v", err)
	}
	if got.SyncStatus != lot.SyncStatusSynced {
		t.Errorf("上传后 SyncStatus = %q, want %q", got.SyncStatus, lot.SyncStatusSynced)
	}
}
