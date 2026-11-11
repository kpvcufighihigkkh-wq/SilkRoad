package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CenterModuleHandler 吊车查询处理器（Center 侧，只读）
type CenterModuleHandler struct {
	moduleService *service.ModuleService
}

// NewCenterModuleHandler 创建 Center 侧吊车查询处理器
func NewCenterModuleHandler(moduleService *service.ModuleService) *CenterModuleHandler {
	return &CenterModuleHandler{moduleService: moduleService}
}

// ListModules godoc
// @Summary 查询吊车列表
// @Tags modules
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "状态：loading / transporting / sorting / warehouse / idle"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.ModuleResponse}}
// @Router /v1/modules [get]
func (h *CenterModuleHandler) ListModules(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	rows, total, err := h.moduleService.ListModules(
		c.Request.Context(), page, pageSize, c.Query("status"),
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

// GetModule godoc
// @Summary 获取吊车详情
// @Tags modules
// @Produce json
// @Param id path string true "吊车ID"
// @Success 200 {object} api.Response{data=service.ModuleResponse}
// @Failure 404 {object} api.Response
// @Router /v1/modules/{id} [get]
func (h *CenterModuleHandler) GetModule(c *gin.Context) {
	resp, err := h.moduleService.GetModule(c.Request.Context(), c.Param("id"))
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
