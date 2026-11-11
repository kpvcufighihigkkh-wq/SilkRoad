package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CenterBarrelHandler 桶查询处理器（Center 侧，只读）
type CenterBarrelHandler struct {
	barrelService *service.BarrelService
}

// NewCenterBarrelHandler 创建 Center 侧桶查询处理器
func NewCenterBarrelHandler(barrelService *service.BarrelService) *CenterBarrelHandler {
	return &CenterBarrelHandler{barrelService: barrelService}
}

// ListBarrels godoc
// @Summary 查询桶列表
// @Tags barrels
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID"
// @Param status query string false "状态：active / full / sorted / packed"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.BarrelResponse}}
// @Router /v1/barrels [get]
func (h *CenterBarrelHandler) ListBarrels(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	rows, total, err := h.barrelService.ListBarrels(
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

	c.JSON(http.StatusOK, api.PageSuccess(rows, page, pageSize, total))
}

// GetBarrel godoc
// @Summary 获取桶详情
// @Tags barrels
// @Produce json
// @Param id path string true "桶ID"
// @Success 200 {object} api.Response{data=service.BarrelResponse}
// @Failure 404 {object} api.Response
// @Router /v1/barrels/{id} [get]
func (h *CenterBarrelHandler) GetBarrel(c *gin.Context) {
	resp, err := h.barrelService.GetBarrel(c.Request.Context(), c.Param("id"))
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
