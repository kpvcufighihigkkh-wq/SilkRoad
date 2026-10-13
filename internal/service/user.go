package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/user"
	"github.com/google/uuid"
)

// UserService 用户服务
type UserService struct {
	client *ent.Client
}

// NewUserService 创建用户服务
func NewUserService(client *ent.Client) *UserService {
	return &UserService{
		client: client,
	}
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
	RealName string `json:"real_name" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// UpdateUserRequest 更新用户请求
//
// Role 用指针以区分「未提供」与「显式空串」：
//
//	nil      → 不改角色
//	&"admin" → 改角色为 admin
//	&""      → 非法，返回参数错误
//
// 用值类型时二者都会被 encoding/json 解析成空串，无法区分，
// 前端只改姓名的请求会意外清空角色。
type UpdateUserRequest struct {
	RealName string  `json:"real_name"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone"`
	Role     *string `json:"role"`
	IsActive bool    `json:"is_active"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// UserResponse 用户响应
type UserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	Role      string `json:"role"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateUser 创建用户。
//
// callerRole 是发起请求者的角色。校验「目标角色不得高于调用者」——
// 缺少这道校验时，任何已登录用户都能创建一个 admin 账号完成提权。
func (s *UserService) CreateUser(ctx context.Context, callerRole string, req *CreateUserRequest) (*UserResponse, error) {
	if !IsRoleHigherOrEqual(callerRole, req.Role) {
		return nil, ErrRoleTooHigh
	}

	// 检查用户名是否已存在
	exists, err := s.client.User.Query().
		Where(user.UsernameEQ(req.Username)).
		Exist(ctx)

	if err != nil {
		return nil, err
	}

	if exists {
		return nil, ErrUserExists
	}

	// 加密密码
	passwordHash := hashPassword(req.Password)

	builder := s.client.User.Create().
		SetUsername(req.Username).
		SetPasswordHash(passwordHash).
		SetFullName(req.RealName).
		SetRole(user.Role(req.Role)).
		SetIsActive(true)

	if req.Email != "" {
		builder.SetEmail(req.Email)
	}

	if req.Phone != "" {
		builder.SetPhone(req.Phone)
	}

	u, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toUserResponse(u), nil
}

// GetUser 获取用户详情
func (s *UserService) GetUser(ctx context.Context, id string) (*UserResponse, error) {
	userID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.toUserResponse(u), nil
}

// GetUserByUsername 根据用户名获取用户
func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*ent.User, error) {
	return s.client.User.Query().
		Where(user.UsernameEQ(username)).
		Only(ctx)
}

// ListUsers 查询用户列表
func (s *UserService) ListUsers(ctx context.Context, page, pageSize int, role string) ([]*UserResponse, int, error) {
	query := s.client.User.Query()

	// 过滤条件
	if role != "" {
		query = query.Where(user.RoleEQ(user.Role(role)))
	}

	// 查询总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	users, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc("created_at")).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	var result []*UserResponse
	for _, u := range users {
		result = append(result, s.toUserResponse(u))
	}

	return result, total, nil
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(ctx context.Context, id, callerRole string, req *UpdateUserRequest) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	builder := s.client.User.UpdateOneID(userID)

	if req.RealName != "" {
		builder.SetFullName(req.RealName)
	}

	if req.Email != "" {
		builder.SetEmail(req.Email)
	}

	if req.Phone != "" {
		builder.SetPhone(req.Phone)
	}

	builder.SetIsActive(req.IsActive)

	// Role 为 nil 表示不改角色；非 nil 时必须校验。
	if req.Role != nil {
		if !IsRoleHigherOrEqual(callerRole, *req.Role) {
			return ErrRoleTooHigh
		}
		builder.SetRole(user.Role(*req.Role))
	}

	return builder.Exec(ctx)
}

// ChangePassword 修改密码
func (s *UserService) ChangePassword(ctx context.Context, id string, req *ChangePasswordRequest) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	// 获取用户
	u, err := s.client.User.Get(ctx, userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if !verifyPassword(req.OldPassword, u.PasswordHash) {
		return ErrInvalidPassword
	}

	// 更新密码
	newPasswordHash := hashPassword(req.NewPassword)
	return s.client.User.UpdateOneID(userID).
		SetPasswordHash(newPasswordHash).
		Exec(ctx)
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	userID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.User.DeleteOneID(userID).Exec(ctx)
}

// ValidateLogin 验证登录
func (s *UserService) ValidateLogin(ctx context.Context, username, password string) (*ent.User, error) {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !u.IsActive {
		return nil, ErrUserInactive
	}

	if !verifyPassword(password, u.PasswordHash) {
		return nil, ErrInvalidPassword
	}

	return u, nil
}

// toUserResponse 转换为响应格式
func (s *UserService) toUserResponse(u *ent.User) *UserResponse {
	resp := &UserResponse{
		ID:        u.ID.String(),
		Username:  u.Username,
		RealName:  u.FullName,
		Role:      string(u.Role),
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if u.Email != "" {
		resp.Email = u.Email
	}

	if u.Phone != "" {
		resp.Phone = u.Phone
	}

	return resp
}

// hashPassword 哈希密码
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// verifyPassword 验证密码
func verifyPassword(password, hash string) bool {
	return hashPassword(password) == hash
}

// 错误定义
var (
	ErrUserExists      = &ServiceError{Code: 40002, Message: "用户名已存在"}
	ErrUserNotFound    = &ServiceError{Code: 40001, Message: "用户不存在"}
	ErrUserInactive    = &ServiceError{Code: 20004, Message: "用户已禁用"}
	ErrInvalidPassword = &ServiceError{Code: 20001, Message: "密码错误"}

	// ErrRoleTooHigh 目标角色高于调用者，或角色值非法。
	ErrRoleTooHigh = &ServiceError{
		Code:    30001, // api.CodeBusinessRuleError
		Message: "无权创建或授予高于自身级别的角色",
	}
)

// ServiceError 服务错误
type ServiceError struct {
	Code    int
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}
