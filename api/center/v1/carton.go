package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CenterCartonHandler 纸箱查询处理器（Center 侧，只读）
type CenterCartonHandler struct {
	cartonService *service.CartonService
}

// NewCenterCartonHandler 创建 Center 侧纸箱查询处理器
func NewCenterCartonHandler(cartonService *service.CartonService) *CenterCartonHandler {
	return &CenterCartonHandler{cartonService: cartonService}
}

// ListCartons godoc
// @Summary 查询纸箱列表
// @Tags cartons
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID"
// @Param pallet_id query string false "托盘ID（列盘内纸箱）"
// @Param status query string false "状态：packing / packed / shipped"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.CartonResponse}}
// @Router /v1/cartons [get]
func (h *CenterCartonHandler) ListCartons(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	rows, total, err := h.cartonService.ListCartons(
		c.Request.Context(), page, pageSize,
		c.Query("lot_id"), c.Query("status"), c.Query("pallet_id"),
	)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(rows, page, pageSize, total))
}

// GetCarton godoc
// @Summary 获取纸箱详情
// @Tags cartons
// @Produce json
// @Param id path string true "纸箱ID"
// @Success 200 {object} api.Response{data=service.CartonResponse}
// @Failure 404 {object} api.Response
// @Router /v1/cartons/{id} [get]
func (h *CenterCartonHandler) GetCarton(c *gin.Context) {
	resp, err := h.cartonService.GetCarton(c.Request.Context(), c.Param("id"))
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusNotFound, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}
