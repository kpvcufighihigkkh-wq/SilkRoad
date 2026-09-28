package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// DoffingHandler 落纱操作处理器
type DoffingHandler struct {
	doffingService *service.DoffingService
}

// NewDoffingHandler 创建落纱操作处理器
func NewDoffingHandler(doffingService *service.DoffingService) *DoffingHandler {
	return &DoffingHandler{
		doffingService: doffingService,
	}
}

// CreateDoffing godoc
// @Summary 创建落纱操作
// @Tags doffing
// @Accept json
// @Produce json
// @Param doffing body service.CreateDoffingRequest true "落纱信息"
// @Success 200 {object} api.Response{data=service.DoffingResponse}
// @Failure 400 {object} api.Response
// @Router /v1/doffing [post]
func (h *DoffingHandler) CreateDoffing(c *gin.Context) {
	var req service.CreateDoffingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	doffing, err := h.doffingService.CreateDoffing(c.Request.Context(), &req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(doffing))
}

// GetDoffing godoc
// @Summary 获取落纱详情
// @Tags doffing
// @Produce json
// @Param id path string true "落纱ID"
// @Success 200 {object} api.Response{data=service.DoffingResponse}
// @Failure 404 {object} api.Response
// @Router /v1/doffing/{id} [get]
func (h *DoffingHandler) GetDoffing(c *gin.Context) {
	id := c.Param("id")

	doffing, err := h.doffingService.GetDoffing(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeResourceNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(doffing))
}

// ListDoffing godoc
// @Summary 查询落纱记录
// @Tags doffing
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param spinning_line_id query string false "线体ID"
// @Param status query string false "状态"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.DoffingResponse}}
// @Router /v1/doffing [get]
func (h *DoffingHandler) ListDoffing(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	spinningLineID := c.Query("spinning_line_id")
	status := c.Query("status")

	doffings, total, err := h.doffingService.ListDoffing(c.Request.Context(), page, pageSize, spinningLineID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(doffings, total, page, pageSize))
}

// ConfirmDoffing godoc
// @Summary 确认落纱
// @Tags doffing
// @Accept json
// @Produce json
// @Param id path string true "落纱ID"
// @Param confirm body service.ConfirmDoffingRequest true "确认信息"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/doffing/{id}/confirm [put]
func (h *DoffingHandler) ConfirmDoffing(c *gin.Context) {
	id := c.Param("id")

	var req service.ConfirmDoffingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	if err := h.doffingService.ConfirmDoffing(c.Request.Context(), id, &req); err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// CancelDoffing godoc
// @Summary 取消落纱
// @Tags doffing
// @Accept json
// @Produce json
// @Param id path string true "落纱ID"
// @Param cancel body service.CancelDoffingRequest true "取消原因"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/doffing/{id}/cancel [put]
func (h *DoffingHandler) CancelDoffing(c *gin.Context) {
	id := c.Param("id")

	var req service.CancelDoffingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	if err := h.doffingService.CancelDoffing(c.Request.Context(), id, &req); err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
