package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CenterDoffingHandler 落纱记录查询处理器（Center 侧，只读）
//
// 与 Edge 侧的 DoffingHandler 分开：Edge 需要创建/确认/取消落纱，
// Center 只读；混在一个 handler 里会让 Center 的包意外获得写方法。
type CenterDoffingHandler struct {
	doffingService *service.DoffingService
}

// NewCenterDoffingHandler 创建 Center 侧落纱查询处理器
func NewCenterDoffingHandler(doffingService *service.DoffingService) *CenterDoffingHandler {
	return &CenterDoffingHandler{doffingService: doffingService}
}

// ListDoffings godoc
// @Summary 查询落纱记录
// @Tags doffings
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID"
// @Param spinning_line_id query string false "线体ID"
// @Param operator_id query string false "操作员ID"
// @Param status query string false "状态：pending / confirmed / cancelled"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.DoffingResponse}}
// @Router /v1/doffings [get]
func (h *CenterDoffingHandler) ListDoffings(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	filter := service.DoffingListFilter{
		LotID:          c.Query("lot_id"),
		SpinningLineID: c.Query("spinning_line_id"),
		OperatorID:     c.Query("operator_id"),
		Status:         c.Query("status"),
	}

	doffings, total, err := h.doffingService.ListDoffings(c.Request.Context(), page, pageSize, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(doffings, page, pageSize, total))
}
