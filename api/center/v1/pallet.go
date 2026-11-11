package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CenterPalletHandler 托盘查询处理器（Center 侧，只读）
//
// 与 Edge 侧的 PalletHandler 分开：那边能创建、码盘、封托，
// Center 只提供查询视图。
type CenterPalletHandler struct {
	palletService *service.PalletService
}

// NewCenterPalletHandler 创建 Center 侧托盘查询处理器
func NewCenterPalletHandler(palletService *service.PalletService) *CenterPalletHandler {
	return &CenterPalletHandler{palletService: palletService}
}

// ListPallets godoc
// @Summary 查询托盘列表
// @Tags pallets
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID"
// @Param status query string false "状态：building / completed / shipped"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.PalletResponse}}
// @Router /v1/pallets [get]
func (h *CenterPalletHandler) ListPallets(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	pallets, total, err := h.palletService.ListPallets(
		c.Request.Context(), page, pageSize,
		c.Query("lot_id"), c.Query("status"),
	)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(pallets, page, pageSize, total))
}

// GetPallet godoc
// @Summary 获取托盘详情
// @Tags pallets
// @Produce json
// @Param id path string true "托盘ID"
// @Success 200 {object} api.Response{data=service.PalletResponse}
// @Failure 404 {object} api.Response
// @Router /v1/pallets/{id} [get]
func (h *CenterPalletHandler) GetPallet(c *gin.Context) {
	resp, err := h.palletService.GetPallet(c.Request.Context(), c.Param("id"))
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
