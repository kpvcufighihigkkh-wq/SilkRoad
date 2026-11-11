package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// PalletHandler 托盘处理器（Edge 侧）
//
// 托盘由 Edge 产生：包装线创建托盘、码入丝锭、称重封托。
// Center 只读，不挂这几个写接口。
type PalletHandler struct {
	palletService *service.PalletService
}

// NewPalletHandler 创建托盘处理器
func NewPalletHandler(palletService *service.PalletService) *PalletHandler {
	return &PalletHandler{palletService: palletService}
}

// CreatePallet godoc
// @Summary 创建托盘
// @Tags pallets
// @Accept json
// @Produce json
// @Param pallet body service.CreatePalletRequest true "托盘信息"
// @Success 200 {object} api.Response{data=service.PalletResponse}
// @Failure 400 {object} api.Response
// @Router /v1/pallets [post]
func (h *PalletHandler) CreatePallet(c *gin.Context) {
	var req service.CreatePalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.palletService.CreatePallet(c.Request.Context(), &req)
	if err != nil {
		respondPalletError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// PackBobbins godoc
// @Summary 丝锭码盘
// @Tags pallets
// @Accept json
// @Produce json
// @Param id path string true "托盘ID"
// @Param body body service.PackBobbinsRequest true "丝锭列表"
// @Success 200 {object} api.Response{data=service.PalletResponse}
// @Router /v1/pallets/{id}/pack [post]
func (h *PalletHandler) PackBobbins(c *gin.Context) {
	id := c.Param("id")

	var req service.PackBobbinsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.palletService.PackBobbins(c.Request.Context(), id, &req)
	if err != nil {
		respondPalletError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// PackCartons godoc
// @Summary DTY 纸箱码垛
// @Tags pallets
// @Accept json
// @Produce json
// @Param id path string true "托盘ID"
// @Param body body service.PackCartonsRequest true "纸箱列表"
// @Success 200 {object} api.Response{data=service.PalletResponse}
// @Router /v1/pallets/{id}/pack-cartons [post]
func (h *PalletHandler) PackCartons(c *gin.Context) {
	id := c.Param("id")

	var req service.PackCartonsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.palletService.PackCartons(c.Request.Context(), id, &req)
	if err != nil {
		respondPalletError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// SealPallet godoc
// @Summary 封托
// @Tags pallets
// @Accept json
// @Produce json
// @Param id path string true "托盘ID"
// @Param body body service.SealPalletRequest true "重量信息"
// @Success 200 {object} api.Response{data=service.PalletResponse}
// @Router /v1/pallets/{id}/seal [post]
func (h *PalletHandler) SealPallet(c *gin.Context) {
	id := c.Param("id")

	var req service.SealPalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, err.Error()))
		return
	}

	resp, err := h.palletService.SealPallet(c.Request.Context(), id, &req)
	if err != nil {
		respondPalletError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// respondPalletError 把服务层错误映射为 HTTP 响应。
//
// 40001（不存在）返回 404，30001（状态不允许）返回 409 —— 后者表示
// 「请求与当前资源状态冲突」，与参数错误（400）是两回事。
func respondPalletError(c *gin.Context, err error) {
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
