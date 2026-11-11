package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// LotHandler 批次处理器（Edge 侧）
//
// 批次由 Edge 产生：投产时建批次并写开工时刻，完工时推进状态写结束时刻。
// Center 只读。
type LotHandler struct {
	lotService *service.LotService
}

// NewLotHandler 创建批次处理器
func NewLotHandler(lotService *service.LotService) *LotHandler {
	return &LotHandler{lotService: lotService}
}

// CreateLot godoc
// @Summary 创建批次
// @Tags lots
// @Accept json
// @Produce json
// @Param lot body service.CreateLotRequest true "批次信息"
// @Success 200 {object} api.Response{data=service.LotResponse}
// @Router /v1/lots [post]
func (h *LotHandler) CreateLot(c *gin.Context) {
	var req service.CreateLotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.lotService.CreateLot(c.Request.Context(), &req)
	if err != nil {
		respondLotError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// UpdateStatus godoc
// @Summary 批次状态流转
// @Tags lots
// @Accept json
// @Produce json
// @Param id path string true "批次ID"
// @Param body body service.UpdateLotStatusRequest true "状态与实产量"
// @Success 200 {object} api.Response{data=service.LotResponse}
// @Router /v1/lots/{id}/status [put]
func (h *LotHandler) UpdateStatus(c *gin.Context) {
	var req service.UpdateLotStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.lotService.UpdateLotStatus(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondLotError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// respondLotError 把服务层错误映射为 HTTP 响应（规则与其它 Edge 写接口一致）。
func respondLotError(c *gin.Context, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	switch svcErr.Code {
	case 40001:
		c.JSON(http.StatusNotFound, api.Error(svcErr.Code, svcErr.Message))
	case 40002:
		c.JSON(http.StatusConflict, api.Error(svcErr.Code, svcErr.Message))
	case 30001:
		c.JSON(http.StatusConflict, api.Error(svcErr.Code, svcErr.Message))
	default:
		c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
	}
}
