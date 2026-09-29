package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// BobbinHandler 丝锭处理器
type BobbinHandler struct {
	bobbinService *service.BobbinService
}

// NewBobbinHandler 创建丝锭处理器
func NewBobbinHandler(bobbinService *service.BobbinService) *BobbinHandler {
	return &BobbinHandler{
		bobbinService: bobbinService,
	}
}

// CreateBobbin godoc
// @Summary 创建丝锭
// @Tags bobbins
// @Accept json
// @Produce json
// @Param bobbin body service.CreateBobbinRequest true "丝锭信息"
// @Success 200 {object} api.Response{data=service.BobbinResponse}
// @Failure 400 {object} api.Response
// @Router /v1/bobbins [post]
func (h *BobbinHandler) CreateBobbin(c *gin.Context) {
	var req service.CreateBobbinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	bobbin, err := h.bobbinService.CreateBobbin(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(bobbin))
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
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
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
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.BobbinResponse}}
// @Router /v1/bobbins [get]
func (h *BobbinHandler) ListBobbins(c *gin.Context) {
	page, pageSize := api.ParsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	lotID := c.Query("lot_id")
	status := c.Query("status")

	bobbins, total, err := h.bobbinService.ListBobbins(c.Request.Context(), page, pageSize, lotID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(bobbins, page, pageSize, total))
}

// UpdateBobbinStatus godoc
// @Summary 更新丝锭状态
// @Tags bobbins
// @Accept json
// @Produce json
// @Param id path string true "丝锭ID"
// @Param status body object{status=string} true "状态"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/bobbins/{id}/status [put]
func (h *BobbinHandler) UpdateBobbinStatus(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	if err := h.bobbinService.UpdateBobbinStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// MarkBobbinPrinted godoc
// @Summary 标记丝锭已打印
// @Tags bobbins
// @Produce json
// @Param id path string true "丝锭ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/bobbins/{id}/print [post]
func (h *BobbinHandler) MarkBobbinPrinted(c *gin.Context) {
	id := c.Param("id")

	if err := h.bobbinService.MarkBobbinPrinted(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// DeleteBobbin godoc
// @Summary 删除丝锭
// @Tags bobbins
// @Produce json
// @Param id path string true "丝锭ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /v1/bobbins/{id} [delete]
func (h *BobbinHandler) DeleteBobbin(c *gin.Context) {
	id := c.Param("id")

	if err := h.bobbinService.DeleteBobbin(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
