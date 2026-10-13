package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// LotHandler 批次处理器
type LotHandler struct {
	lotService *service.LotService
}

// NewLotHandler 创建批次处理器
func NewLotHandler(lotService *service.LotService) *LotHandler {
	return &LotHandler{
		lotService: lotService,
	}
}

// GetLot godoc
// @Summary 获取批次详情
// @Tags lots
// @Produce json
// @Param id path string true "批次ID"
// @Success 200 {object} api.Response{data=service.LotResponse}
// @Failure 404 {object} api.Response
// @Router /v1/lots/{id} [get]
func (h *LotHandler) GetLot(c *gin.Context) {
	id := c.Param("id")

	lot, err := h.lotService.GetLot(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(lot))
}

// ListLots godoc
// @Summary 查询批次列表
// @Tags lots
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param edge_id query string false "边端设备ID"
// @Param status query string false "状态"
// @Param order_code query string false "订单号（精确匹配）"
// @Param product_type query string false "产品类型（FDY/POY/DTY）"
// @Param keyword query string false "批次号模糊搜索"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.LotResponse}}
// @Router /v1/lots [get]
func (h *LotHandler) ListLots(c *gin.Context) {
	page, pageSize := api.ParsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	edgeID := c.Query("edge_id")
	status := c.Query("status")
	orderCode := c.Query("order_code")
	productType := c.Query("product_type")
	keyword := c.Query("keyword")

	lots, total, err := h.lotService.ListLots(
		c.Request.Context(), page, pageSize,
		edgeID, status, orderCode, productType, keyword,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(lots, page, pageSize, total))
}
