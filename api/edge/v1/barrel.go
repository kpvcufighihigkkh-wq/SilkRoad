package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// BarrelHandler 落纱桶处理器（Edge 侧）
//
// 桶由落纱工位产生：建桶、按锭位装锭、封桶。Center 只读。
type BarrelHandler struct {
	barrelService *service.BarrelService
}

// NewBarrelHandler 创建落纱桶处理器
func NewBarrelHandler(barrelService *service.BarrelService) *BarrelHandler {
	return &BarrelHandler{barrelService: barrelService}
}

// CreateBarrel godoc
// @Summary 创建桶
// @Tags barrels
// @Accept json
// @Produce json
// @Param barrel body service.CreateBarrelRequest true "桶信息"
// @Success 200 {object} api.Response{data=service.BarrelResponse}
// @Router /v1/barrels [post]
func (h *BarrelHandler) CreateBarrel(c *gin.Context) {
	var req service.CreateBarrelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.barrelService.CreateBarrel(c.Request.Context(), &req)
	if err != nil {
		respondBarrelError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// LoadBobbins godoc
// @Summary 装锭
// @Tags barrels
// @Accept json
// @Produce json
// @Param id path string true "桶ID"
// @Param body body service.LoadBobbinsRequest true "丝锭与锭位"
// @Success 200 {object} api.Response{data=service.BarrelResponse}
// @Router /v1/barrels/{id}/load [post]
func (h *BarrelHandler) LoadBobbins(c *gin.Context) {
	var req service.LoadBobbinsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.barrelService.LoadBobbins(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondBarrelError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// SealBarrel godoc
// @Summary 封桶
// @Tags barrels
// @Produce json
// @Param id path string true "桶ID"
// @Success 200 {object} api.Response{data=service.BarrelResponse}
// @Router /v1/barrels/{id}/seal [post]
func (h *BarrelHandler) SealBarrel(c *gin.Context) {
	resp, err := h.barrelService.SealBarrel(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondBarrelError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// respondBarrelError 把服务层错误映射为 HTTP 响应。
// 映射规则与托盘一致：不存在→404，状态冲突→409，其余→400。
func respondBarrelError(c *gin.Context, err error) {
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
