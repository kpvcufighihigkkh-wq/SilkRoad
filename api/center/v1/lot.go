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

// CreateLot godoc
// @Summary 创建批次
// @Tags lots
// @Accept json
// @Produce json
// @Param lot body service.CreateLotRequest true "批次信息"
// @Success 200 {object} api.Response{data=service.LotResponse}
// @Failure 400 {object} api.Response
// @Router /v1/lots [post]
func (h *LotHandler) CreateLot(c *gin.Context) {
	var req service.CreateLotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	lot, err := h.lotService.CreateLot(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(lot))
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
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.LotResponse}}
// @Router /v1/lots [get]
func (h *LotHandler) ListLots(c *gin.Context) {
	page, pageSize := api.ParsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	edgeID := c.Query("edge_id")
	status := c.Query("status")

	lots, total, err := h.lotService.ListLots(c.Request.Context(), page, pageSize, edgeID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(lots, page, pageSize, total))
}

// UpdateLotStatus godoc
// @Summary 更新批次状态
// @Tags lots
// @Accept json
// @Produce json
// @Param id path string true "批次ID"
// @Param status body object{status=string} true "状态"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/lots/{id}/status [put]
func (h *LotHandler) UpdateLotStatus(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	if err := h.lotService.UpdateLotStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// DeleteLot godoc
// @Summary 删除批次
// @Tags lots
// @Produce json
// @Param id path string true "批次ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /v1/lots/{id} [delete]
func (h *LotHandler) DeleteLot(c *gin.Context) {
	id := c.Param("id")

	if err := h.lotService.DeleteLot(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
