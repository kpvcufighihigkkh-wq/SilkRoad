package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// /health 的语义是「能否提供服务」，不是「进程活着」。
//
// 双机热备下备机的进程同样活着，但它连不上数据库（副本只读或主备未切换）
// 时必须返回非 2xx，否则负载均衡会把流量继续送进来。本用例钉住两个方向：
// 库可用 → 200 ok；库不可用（此处以 nil client 表示）→ 503 unavailable。
func TestCenter_HealthReflectsDatabaseReadiness(t *testing.T) {
	cases := []struct {
		name       string
		withClient bool
		wantStatus int
		wantBody   string
	}{
		{name: "库可用", withClient: true, wantStatus: http.StatusOK, wantBody: "ok"},
		{name: "库不可用", withClient: false, wantStatus: http.StatusServiceUnavailable, wantBody: "unavailable"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var client = newServerTestClient(t)
			if !tc.withClient {
				// 装配一个未接库的服务实例：与生产里「数据库连接失败」等价。
				srv := newCenterServerForTest(t, nil)
				assertHealth(t, srv.router, tc.wantStatus, tc.wantBody)
				return
			}

			srv := newCenterServerForTest(t, client)
			assertHealth(t, srv.router, tc.wantStatus, tc.wantBody)
		})
	}
}

func assertHealth(t *testing.T, router http.Handler, wantStatus int, wantBody string) {
	t.Helper()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	router.ServeHTTP(w, req)

	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, wantStatus, w.Body.String())
	}

	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败：%v; body=%s", err, w.Body.String())
	}
	if body.Data.Status != wantBody {
		t.Errorf("data.status = %q, want %q", body.Data.Status, wantBody)
	}
}
