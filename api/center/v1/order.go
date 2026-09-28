package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// OrderHandler 订单API处理器
type OrderHandler struct {
	orderService *service.OrderService
}

// NewOrderHandler 创建订单处理器
func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

// CreateOrder 创建订单
// @Summary 创建订单
// @Description 创建新的生产订单
// @Tags 订单管理
// @Accept json
// @Produce json
// @Param order body service.CreateOrderRequest true "订单信息"
// @Success 200 {object} api.Response{data=service.OrderResponse}
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /v1/orders [post]
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误: "+err.Error()))
		return
	}

	order, err := h.orderService.CreateOrder(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeDatabaseError, "创建订单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(order))
}

// GetOrder 获取订单详情
// @Summary 获取订单详情
// @Description 根据ID获取订单详细信息
// @Tags 订单管理
// @Produce json
// @Param id path string true "订单ID"
// @Success 200 {object} api.Response{data=service.OrderResponse}
// @Failure 404 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /v1/orders/{id} [get]
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id := c.Param("id")

	order, err := h.orderService.GetOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeResourceNotFound, "订单不存在"))
		return
	}

	c.JSON(http.StatusOK, api.Success(order))
}

// ListOrders 查询订单列表
// @Summary 查询订单列表
// @Description 分页查询订单列表，支持状态过滤
// @Tags 订单管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "订单状态"
// @Success 200 {object} api.PaginatedResponse{data=[]service.OrderResponse}
// @Failure 500 {object} api.ErrorResponse
// @Router /v1/orders [get]
func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	orders, total, err := h.orderService.ListOrders(c.Request.Context(), page, pageSize, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeDatabaseError, "查询订单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(orders, page, pageSize, total))
}

// DeleteOrder 删除订单
// @Summary 删除订单
// @Description 删除指定订单
// @Tags 订单管理
// @Produce json
// @Param id path string true "订单ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /v1/orders/{id} [delete]
func (h *OrderHandler) DeleteOrder(c *gin.Context) {
	id := c.Param("id")

	err := h.orderService.DeleteOrder(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeDatabaseError, "删除订单失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
