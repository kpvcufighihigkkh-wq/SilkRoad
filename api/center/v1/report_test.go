package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/api"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// TestParseTimeRange 钉住报表端点的区间语义：
// 缺省最近七天、start 闭 end 开、非法输入与过宽区间必须报错而不是被静默修正。
func TestParseTimeRange(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		start     string
		end       string
		wantErr   bool
		wantStart time.Time
		wantEnd   time.Time
	}{
		{"都不传 → 最近七天", "", "", false,
			now.Add(-7 * 24 * time.Hour), now},
		{"只传 start", "2026-11-01T00:00:00Z", "", false,
			time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), now},
		{"只传 end", "", "2026-11-05T00:00:00Z", false,
			time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)},
		{"start 晚于 end", "2026-11-09T00:00:00Z", "2026-11-08T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 等于 end", "2026-11-09T00:00:00Z", "2026-11-09T00:00:00Z", true, time.Time{}, time.Time{}},
		{"start 格式非法", "2026/11/01", "", true, time.Time{}, time.Time{}},
		{"end 格式非法", "", "not-a-time", true, time.Time{}, time.Time{}},
		{"跨度 367 天", "2025-11-08T00:00:00Z", "2026-11-10T00:00:00Z", true, time.Time{}, time.Time{}},
		{"跨度恰好 366 天", "2025-11-09T00:00:00Z", "2026-11-10T00:00:00Z", false,
			time.Date(2025, 11, 9, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)},
		{"带偏移量的时间统一转 UTC", "2026-11-01T08:00:00+08:00", "2026-11-02T08:00:00+08:00", false,
			time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, err := centerv1.ParseTimeRange(tc.start, tc.end, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got start=%v end=%v", start, end)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !start.Equal(tc.wantStart) || !end.Equal(tc.wantEnd) {
				t.Errorf("got [%v, %v), want [%v, %v)", start, end, tc.wantStart, tc.wantEnd)
			}
			if start.Location() != time.UTC || end.Location() != time.UTC {
				t.Errorf("want UTC, got %v and %v", start.Location(), end.Location())
			}
		})
	}
}

// TestParseTopN 钉住 top 参数的缺省值与上限。
// 越界一律报错：静默截断会让前端误以为拿到了自己想要的数量。
func TestParseTopN(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{"不传取缺省", "", 10, false},
		{"下界", "1", 1, false},
		{"上界", "50", 50, false},
		{"零", "0", 0, true},
		{"负数", "-1", 0, true},
		{"超过上限", "51", 0, true},
		{"非数字", "abc", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := centerv1.ParseTopN(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// setupReportTest 装配一个只挂报表路由的 router，后端用内存 SQLite。
//
// 只测 handler 的参数处理与响应包装：聚合口径由 service 层测试覆盖，
// 这里要钉的是「handler 有没有把解析出来的参数真的传下去」以及
// 「错误有没有映射成正确的状态码」。
func setupReportTest(t *testing.T) (*gin.Engine, *ent.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := setupTestClientShared(t)
	h := centerv1.NewReportHandler(service.NewReportService(client))

	router := gin.New()
	router.GET("/v1/reports/quality-distribution", h.QualityDistribution)
	router.GET("/v1/reports/machine-operator", h.MachineOperator)

	return router, client
}

func TestReportHandler_QualityDistribution_InvalidTimeRange(t *testing.T) {
	router, _ := setupReportTest(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/reports/quality-distribution?start=not-a-time", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Code != api.CodeInvalidParams {
		t.Errorf("code = %d, want %d", resp.Code, api.CodeInvalidParams)
	}
}

// top 必须真的被传进 service：种三条不同的缺陷原因，top=1 应只回一行。
// 若 handler 把 c.Query("top") 写成别的键，这里会变红。
func TestReportHandler_QualityDistribution_TopIsPassedThrough(t *testing.T) {
	router, client := setupReportTest(t)
	ctx := context.Background()

	now := time.Now().UTC()
	// 种子全部落在「最近七天」窗口内：缺省 end 取服务端当前时间，
	// 种在 now 或之后的记录会被右开区间排除，导致计数少一条。
	base := now.Add(-time.Hour)
	for i, reason := range []string{"毛丝", "毛丝", "断头", "油污"} {
		if _, err := client.BobbinGrade.Create().
			SetBobbinID(uuid.New()).
			SetGrade("B").
			SetInspectedAt(base.Add(time.Duration(i) * time.Second)).
			SetDefectReason(reason).
			Save(ctx); err != nil {
			t.Fatalf("seed grade: %v", err)
		}
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/reports/quality-distribution?top=1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			TopDefects []struct {
				DefectReason string `json:"defect_reason"`
				Count        int    `json:"count"`
			} `json:"top_defects"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(resp.Data.TopDefects) != 1 {
		t.Fatalf("len(top_defects) = %d, want 1（top=1 未生效）: %+v",
			len(resp.Data.TopDefects), resp.Data.TopDefects)
	}
	if resp.Data.TopDefects[0].DefectReason != "毛丝" || resp.Data.TopDefects[0].Count != 2 {
		t.Errorf("top_defects[0] = %+v, want {毛丝 2}", resp.Data.TopDefects[0])
	}
}

// range 必须回显实际生效的区间，而不是请求里的原始字符串（可能为空）。
func TestReportHandler_QualityDistribution_RangeEchoesEffectiveWindow(t *testing.T) {
	router, _ := setupReportTest(t)

	start := "2026-11-01T00:00:00Z"
	end := "2026-11-02T00:00:00Z"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/v1/reports/quality-distribution?start="+start+"&end="+end, nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Range struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"range"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Data.Range.Start != start {
		t.Errorf("range.start = %q, want %q", resp.Data.Range.Start, start)
	}
	if resp.Data.Range.End != end {
		t.Errorf("range.end = %q, want %q", resp.Data.Range.End, end)
	}
}

// 不传区间时 range 必须回显缺省值（最近七天），而不是空串。
func TestReportHandler_MachineOperator_DefaultRangeIsEchoed(t *testing.T) {
	router, _ := setupReportTest(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/reports/machine-operator", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			Range struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"range"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Data.Range.Start == "" || resp.Data.Range.End == "" {
		t.Fatalf("range 未回显: %+v", resp.Data.Range)
	}
	start, err := time.Parse(time.RFC3339, resp.Data.Range.Start)
	if err != nil {
		t.Fatalf("range.start 不是 RFC3339: %v", err)
	}
	end, err := time.Parse(time.RFC3339, resp.Data.Range.End)
	if err != nil {
		t.Fatalf("range.end 不是 RFC3339: %v", err)
	}
	if got := end.Sub(start); got != 7*24*time.Hour {
		t.Errorf("缺省窗口 = %v, want 168h", got)
	}
}
