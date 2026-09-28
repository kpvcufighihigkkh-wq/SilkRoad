package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// BobbinHandler 丝锭处理器（Edge端）
type BobbinHandler struct {
	bobbinService *service.BobbinService
}

// NewBobbinHandler 创建丝锭处理器
func NewBobbinHandler(bobbinService *service.BobbinService) *BobbinHandler {
	return &BobbinHandler{
		bobbinService: bobbinService,
	}
}

// GetBobbin godoc
// @Summary 获取丝锭详情
// @Tags bobbins
// @Produce json
// @Param id path string true "丝锭ID"
// @Success 200 {object} api.Response{data=service.BobbinResponse}
// @Failure 404 {object} api.Response
// @Router /v1/bobbins/{id} [get]
func (h *BobbinHandler) GetBobbin(c *gin.Context) {
	id := c.Param("id")

	bobbin, err := h.bobbinService.GetBobbin(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeResourceNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(bobbin))
}

// ListBobbins godoc
// @Summary 查询丝锭列表
// @Tags bobbins
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID"
// @Param status query string false "状态"
// @Param spinning_position query int false "纺丝位号"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.BobbinResponse}}
// @Router /v1/bobbins [get]
func (h *BobbinHandler) ListBobbins(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	lotID := c.Query("lot_id")
	status := c.Query("status")

	bobbins, total, err := h.bobbinService.ListBobbins(c.Request.Context(), page, pageSize, lotID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(bobbins, total, page, pageSize))
}

// WeighBobbin godoc
// @Summary 丝锭称重
// @Tags bobbins
// @Accept json
// @Produce json
// @Param id path string true "丝锭ID"
// @Param weigh body service.WeighBobbinRequest true "称重数据"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/bobbins/{id}/weigh [put]
func (h *BobbinHandler) WeighBobbin(c *gin.Context) {
	id := c.Param("id")

	var req service.WeighBobbinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	if err := h.bobbinService.WeighBobbin(c.Request.Context(), id, &req); err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// InspectBobbin godoc
// @Summary 丝锭质检
// @Tags bobbins
// @Accept json
// @Produce json
// @Param id path string true "丝锭ID"
// @Param inspect body service.InspectBobbinRequest true "质检数据"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/bobbins/{id}/inspect [put]
func (h *BobbinHandler) InspectBobbin(c *gin.Context) {
	id := c.Param("id")

	var req service.InspectBobbinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	if err := h.bobbinService.InspectBobbin(c.Request.Context(), id, &req); err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
