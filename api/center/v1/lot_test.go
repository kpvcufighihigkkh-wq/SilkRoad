package v1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)

func setupLotListTest(t *testing.T) (*gin.Engine, *ent.Client) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	client := setupTestClientShared(t)

	h := centerv1.NewLotHandler(service.NewLotService(client))

	router := gin.New()
	router.GET("/v1/lots", h.ListLots)

	return router, client
}

// seedLotsForHandlerFilter 造三条批次，用于验证 handler 的查询参数到筛选条件的映射。
//
// 三条的 order_code 与 product_type 两两不同，因此每个参数都能唯一命中一条；
// 更重要的是三条的 product_spec 完全相同（都不含任何 lot_number 的子串），
// 这样 keyword 一旦命中，来源只可能是 lot_number 列，不可能是 product_spec。
func seedLotsForHandlerFilter(t *testing.T, client *ent.Client) {
	t.Helper()
	ctx := context.Background()

	seeds := []struct {
		number  string
		order   string
		product string
	}{
		{"LOT-AAA-001", "PO-1001", "FDY"},
		{"LOT-BBB-002", "PO-1002", "POY"},
		{"LOT-CCC-003", "PO-1003", "DTY"},
	}

	for _, s := range seeds {
		if _, err := client.Lot.Create().
			SetLotNumber(s.number).
			SetOrderCode(s.order).
			SetProductType(lot.ProductType(s.product)).
			SetProductSpec("150D/48F").
			SetPlannedQuantity(1000).
			SetActualQuantity(0).
			SetStatus(lot.Status("in_progress")).
			Save(ctx); err != nil {
			t.Fatalf("failed seeding lot %s: %v", s.number, err)
		}
	}
}

// TestLotHandler_ListLots_FiltersMapToDistinctColumns 钉住查询参数到筛选条件的映射。
//
// 三个新增参数在 ListLots 里是相邻的 string，写错顺序照样能编译、service 层测试也照样通过
// （service 测试直接调 svc.ListLots，不经过 handler），因此必须在 handler 这一层验证。
func TestLotHandler_ListLots_FiltersMapToDistinctColumns(t *testing.T) {
	router, client := setupLotListTest(t)
	seedLotsForHandlerFilter(t, client)

	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"order_code 精确匹配订单号", "?order_code=PO-1002", "LOT-BBB-002"},
		{"product_type 精确匹配产品类型", "?product_type=DTY", "LOT-CCC-003"},
		{"keyword 模糊匹配批次号", "?keyword=AAA", "LOT-AAA-001"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/lots"+tc.query, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
			}

			var resp struct {
				Data []struct {
					LotNumber string `json:"lot_number"`
				} `json:"data"`
				Pagination struct {
					Total int `json:"total"`
				} `json:"pagination"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("解析响应失败: %v; body=%s", err, w.Body.String())
			}

			if resp.Pagination.Total != 1 {
				t.Errorf("total = %d, want 1", resp.Pagination.Total)
			}

			// 断言返回的批次号恰好是期望的那一条：
			// 返回 0 条、返回别的批次、或未过滤返回全部 3 条，都会在此失败。
			got := make([]string, 0, len(resp.Data))
			for _, l := range resp.Data {
				got = append(got, l.LotNumber)
			}
			if len(got) != 1 || got[0] != tc.want {
				t.Errorf("筛选结果 = %v, want 恰好 [%s]", got, tc.want)
			}
		})
	}
}
