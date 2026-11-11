package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// CartonHandler 纸箱处理器（Edge 侧）
//
// 纸箱是 DTY 专属环节：装箱 → 封箱 → 8 箱/托盘码垛。Center 只读。
type CartonHandler struct {
	cartonService *service.CartonService
}

// NewCartonHandler 创建纸箱处理器
func NewCartonHandler(cartonService *service.CartonService) *CartonHandler {
	return &CartonHandler{cartonService: cartonService}
}

// CreateCarton godoc
// @Summary 创建纸箱
// @Tags cartons
// @Accept json
// @Produce json
// @Param carton body service.CreateCartonRequest true "纸箱信息"
// @Success 200 {object} api.Response{data=service.CartonResponse}
// @Router /v1/cartons [post]
func (h *CartonHandler) CreateCarton(c *gin.Context) {
	var req service.CreateCartonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.cartonService.CreateCarton(c.Request.Context(), &req)
	if err != nil {
		respondCartonError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// PackBobbins godoc
// @Summary 丝锭装箱
// @Tags cartons
// @Accept json
// @Produce json
// @Param id path string true "纸箱ID"
// @Param body body service.PackIntoCartonRequest true "丝锭列表"
// @Success 200 {object} api.Response{data=service.CartonResponse}
// @Router /v1/cartons/{id}/pack [post]
func (h *CartonHandler) PackBobbins(c *gin.Context) {
	var req service.PackIntoCartonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.cartonService.PackBobbins(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondCartonError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// SealCarton godoc
// @Summary 封箱
// @Tags cartons
// @Accept json
// @Produce json
// @Param id path string true "纸箱ID"
// @Param body body service.SealCartonRequest false "总重量（省略时按内容物净重累加）"
// @Success 200 {object} api.Response{data=service.CartonResponse}
// @Router /v1/cartons/{id}/seal [post]
func (h *CartonHandler) SealCarton(c *gin.Context) {
	var req service.SealCartonRequest
	// body 可以完全省略（按净重累加），因此绑定失败不作错误处理 —— 只有
	// 显式传了非法 JSON 才交给服务层判断，这里保持宽容。
	_ = c.ShouldBindJSON(&req)

	resp, err := h.cartonService.SealCarton(c.Request.Context(), c.Param("id"), &req)
	if err != nil {
		respondCartonError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// respondCartonError 把服务层错误映射为 HTTP 响应（规则同托盘/桶）。
func respondCartonError(c *gin.Context, err error) {
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
