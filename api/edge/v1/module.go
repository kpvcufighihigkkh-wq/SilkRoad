package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// ModuleHandler 吊车处理器（Edge 侧）
//
// 吊车是物流载具：装两个落纱桶、推进状态。Center 只读。
type ModuleHandler struct {
	moduleService *service.ModuleService
}

// NewModuleHandler 创建吊车处理器
func NewModuleHandler(moduleService *service.ModuleService) *ModuleHandler {
	return &ModuleHandler{moduleService: moduleService}
}

// CreateModule godoc
// @Summary 创建吊车
// @Tags modules
// @Accept json
// @Produce json
// @Param module body service.CreateModuleRequest true "吊车信息"
// @Success 200 {object} api.Response{data=service.ModuleResponse}
// @Router /v1/modules [post]
func (h *ModuleHandler) CreateModule(c *gin.Context) {
	var req service.CreateModuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.moduleService.CreateModule(c.Request.Context(), &req)
	if err != nil {
		respondModuleError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// LoadBarrels godoc
// @Summary 装桶
// @Tags modules
// @Accept json
// @Produce json
// @Param id path string true "吊车ID"
// @Param body body service.LoadBarrelsRequest true "桶列表（1-2 个）"
// @Success 200 {object} api.Response{data=service.ModuleResponse}
// @Router /v1/modules/{id}/load [post]
func (h *ModuleHandler) LoadBarrels(c *gin.Context) {
	var req service.LoadBarrelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.moduleService.LoadBarrels(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondModuleError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// UpdateStatus godoc
// @Summary 更新吊车状态
// @Tags modules
// @Accept json
// @Produce json
// @Param id path string true "吊车ID"
// @Param body body service.UpdateModuleStatusRequest true "状态与位置"
// @Success 200 {object} api.Response{data=service.ModuleResponse}
// @Router /v1/modules/{id}/status [post]
func (h *ModuleHandler) UpdateStatus(c *gin.Context) {
	var req service.UpdateModuleStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.moduleService.UpdateStatus(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondModuleError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// respondModuleError 把服务层错误映射为 HTTP 响应（规则同托盘/桶/纸箱）。
func respondModuleError(c *gin.Context, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, err.Error()))
		return
	}

	switch svcErr.Code {
	case 40001:
		c.JSON(http.StatusNotFound, api.Error(svcErr.Code, svcErr.Message))
	case 30001:
		c.JSON(http.StatusConflict, api.Error(svcErr.Code, svcErr.Message))
	default:
		c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
	}
}
