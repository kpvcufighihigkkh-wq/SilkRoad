package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// claimsFromContext 从 gin 上下文取出 JWT claims。
// 与 GetCurrentUser 中既有的读取方式一致（c.Get + middleware.ClaimsKey），
// 不改成 c.Request.Context() 的 Value —— 二者不互通（gin 的 Value()
// 只对 string 类型的 key 回退查 Keys，而 ClaimsKey 是自定义类型）。
func claimsFromContext(c *gin.Context) (*middleware.JWTClaims, bool) {
	raw, exists := c.Get(string(middleware.ClaimsKey))
	if !exists {
		return nil, false
	}
	claims, ok := raw.(*middleware.JWTClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// primaryRole 取调用者的最高角色，用于越权比较。
//
// JWT 的 Roles 是数组（设备凭证为 ["edge"]），管理接口的用户凭证实际只含
// 一个角色。取最高权重的那个：多角色时不能因为数组里有一个低权角色
// 就把调用者降级，也不能只取第一个。
func primaryRole(claims *middleware.JWTClaims) string {
	best := ""
	bestRank := 0
	for _, r := range claims.Roles {
		if rank := service.RoleRank(r); rank > bestRank {
			best, bestRank = r, rank
		}
	}
	return best
}

// UserHandler 用户处理器
type UserHandler struct {
	userService *service.UserService
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// CreateUser godoc
// @Summary 创建用户
// @Tags users
// @Accept json
// @Produce json
// @Param user body service.CreateUserRequest true "用户信息"
// @Success 200 {object} api.Response{data=service.UserResponse}
// @Failure 400 {object} api.Response
// @Router /v1/users [post]
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	user, err := h.userService.CreateUser(c.Request.Context(), primaryRole(claims), &req)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(user))
}

// GetUser godoc
// @Summary 获取用户详情
// @Tags users
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {object} api.Response{data=service.UserResponse}
// @Failure 404 {object} api.Response
// @Router /v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	user, err := h.userService.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(user))
}

// ListUsers godoc
// @Summary 查询用户列表
// @Tags users
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param role query string false "角色"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.UserResponse}}
// @Router /v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	page, pageSize := api.ParsePagination(c.DefaultQuery("page", "1"), c.DefaultQuery("page_size", "20"))
	role := c.Query("role")

	users, total, err := h.userService.ListUsers(c.Request.Context(), page, pageSize, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(users, page, pageSize, total))
}

// UpdateUser godoc
// @Summary 更新用户信息
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param user body service.UpdateUserRequest true "更新信息"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	if err := h.userService.UpdateUser(c.Request.Context(), id, primaryRole(claims), &req); err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// GetCurrentUser godoc
// @Summary 获取当前登录用户信息
// @Tags users
// @Produce json
// @Success 200 {object} api.Response{data=service.UserResponse}
// @Failure 401 {object} api.Response
// @Router /v1/users/me [get]
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	claims, exists := c.Get(string(middleware.ClaimsKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	userClaims := claims.(*middleware.JWTClaims)
	user, err := h.userService.GetUser(c.Request.Context(), userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(user))
}

// ChangePassword godoc
// @Summary 修改密码
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param password body service.ChangePasswordRequest true "密码信息"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Router /v1/users/{id}/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	id := c.Param("id")

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	if err := h.userService.ChangePassword(c.Request.Context(), id, &req); err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}

// DeleteUser godoc
// @Summary 删除用户
// @Tags users
// @Produce json
// @Param id path string true "用户ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	if err := h.userService.DeleteUser(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
}
