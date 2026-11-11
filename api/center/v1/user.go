package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

// isSelfOrAdmin 判断调用者是否可以操作路径上的那个用户 ID。
//
// 两侧都按 UUID 解析后比较，而不是比字符串：UUID 是大小写不敏感的十六进制，
// 前端或客户端若以大写形式回传自己的 ID，字符串比较会把「改自己的密码」
// 误判成「改他人密码」并返回 403 —— 而同一个 ID 交给 ChangePassword
// 又能正常解析，用户看到的是「明明是我自己却说我无权」。
//
// 任一侧解析失败都按「不是本人」处理，由 admin 分支兜底；路径参数格式错误
// 最终会在 service 层被拒绝，这里不重复报错。
func isSelfOrAdmin(claims *middleware.JWTClaims, targetID string) bool {
	if primaryRole(claims) == "admin" {
		return true
	}

	callerID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return false
	}
	target, err := uuid.Parse(targetID)
	if err != nil {
		return false
	}

	return callerID == target
}

// respondServiceError 把服务层错误映射为 HTTP 响应。
//
// 20004（无权限）必须返回 403 而不是 400：前端按状态码区分「没权限」与
// 「参数不对」，若越权也回 400，用户会以为是自己填错了。其余业务错误沿用 400。
func respondServiceError(c *gin.Context, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	status := http.StatusBadRequest
	if svcErr.Code == api.CodeForbidden {
		status = http.StatusForbidden
	}
	c.JSON(status, api.Error(svcErr.Code, svcErr.Message))
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
		respondServiceError(c, err)
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
		respondServiceError(c, err)
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
	userClaims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	user, err := h.userService.GetUser(c.Request.Context(), userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, api.Error(api.CodeNotFound, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(user))
}

// ResetPassword godoc
// @Summary 管理员重置他人密码
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "用户ID"
// @Param password body service.ResetPasswordRequest true "新密码"
// @Success 200 {object} api.Response
// @Failure 403 {object} api.Response
// @Router /v1/users/{id}/password/reset [put]
//
// 与 ChangePassword 的分工：那条路由要旧密码、供本人用；这条不要旧密码、
// 挂在 requireRole("admin") 组内。管理员重置自己时同样走这里也没问题 ——
// 他本来就有权限改别人的密码，重置自己只是少一道旧密码校验。
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id := c.Param("id")

	var req service.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	if err := h.userService.ResetPassword(
		c.Request.Context(), id, primaryRole(claims), req.NewPassword,
	); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, api.Success(nil))
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
//
// 允许角色为「admin 或本人」（设计文档权限矩阵）：该路由不在 requireRole("admin")
// 组内，改他人密码必须在这里拦下。ChangePassword 要求提供旧密码，因此 admin
// 实际上改不了他人密码（它不知道旧密码）—— 但「不知道旧密码」是运行时才发现
// 的，不能当作授权依据，越权判定仍须显式做。
func (h *UserHandler) ChangePassword(c *gin.Context) {
	id := c.Param("id")

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	claims, ok := claimsFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
		return
	}

	if !isSelfOrAdmin(claims, id) {
		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权修改他人密码"))
		return
	}

	if err := h.userService.ChangePassword(c.Request.Context(), id, &req); err != nil {
		respondServiceError(c, err)
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
