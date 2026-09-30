package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// EdgeHandler 边端设备处理器
type EdgeHandler struct {
	edgeService *service.EdgeService
	jwtAuth     *middleware.JWTAuth
}

// NewEdgeHandler 创建边端设备处理器
func NewEdgeHandler(edgeService *service.EdgeService, jwtAuth *middleware.JWTAuth) *EdgeHandler {
	return &EdgeHandler{
		edgeService: edgeService,
		jwtAuth:     jwtAuth,
	}
}

// CreateEdge godoc
// @Summary 注册边端设备
// @Tags edges
// @Router /v1/edges [post]
func (h *EdgeHandler) CreateEdge(c *gin.Context) {
	var req service.CreateEdgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	resp, err := h.edgeService.CreateEdge(c.Request.Context(), &req)
	if err != nil {
		// edge_code 有唯一约束；重复注册是客户端错误，不是服务端故障。
		// 不区分会让 POST /v1/edges 对重复 code 返回 500，运维无法判断
		// 是自己传错了还是 Center 坏了。
		if ent.IsConstraintError(err) {
			c.JSON(http.StatusConflict, api.Error(api.CodeResourceExists, "设备编码已存在"))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// ListEdges godoc
// @Summary 查询边端设备列表
// @Tags edges
// @Router /v1/edges [get]
func (h *EdgeHandler) ListEdges(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)
	status := c.Query("status")

	list, total, err := h.edgeService.ListEdges(c.Request.Context(), page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(list, page, pageSize, total))
}

// GetEdge godoc
// @Summary 获取边端设备详情
// @Tags edges
// @Router /v1/edges/{code} [get]
func (h *EdgeHandler) GetEdge(c *gin.Context) {
	code := c.Param("code")

	resp, err := h.edgeService.GetEdgeByCode(c.Request.Context(), code)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, "设备未注册"))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// IssueToken godoc
// @Summary 为已注册设备签发同步凭证
// @Tags edges
// @Router /v1/edges/{code}/token [post]
func (h *EdgeHandler) IssueToken(c *gin.Context) {
	code := c.Param("code")

	// 必须先确认已注册，否则任何字符串都能换到 token
	if _, err := h.edgeService.GetEdgeByCode(c.Request.Context(), code); err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, "设备未注册"))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	token, err := h.jwtAuth.GenerateEdgeToken(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, api.Success(gin.H{
		"token":     token,
		"edge_code": code,
	}))
}
