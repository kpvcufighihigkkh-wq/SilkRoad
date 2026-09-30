package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/edge"
)

// ErrEdgeNotRegistered 表示该 edge_code 未在 Center 注册。
// 上传接口据此拒绝请求 —— 未注册的设备不得被自动接纳。
var ErrEdgeNotRegistered = errors.New("edge not registered")

// EdgeService 边端设备服务
type EdgeService struct {
	client *ent.Client
}

// NewEdgeService 创建边端设备服务
func NewEdgeService(client *ent.Client) *EdgeService {
	return &EdgeService{client: client}
}

// CreateEdgeRequest 注册边端设备请求
type CreateEdgeRequest struct {
	EdgeCode  string `json:"edge_code"  binding:"required,max=50"`
	EdgeName  string `json:"edge_name"  binding:"required,max=100"`
	IPAddress string `json:"ip_address" binding:"required,max=50"`
	Version   string `json:"version"    binding:"omitempty,max=50"`
	Notes     string `json:"notes"      binding:"omitempty"`
}

// EdgeResponse 边端设备响应
type EdgeResponse struct {
	ID        string `json:"id"`
	EdgeCode  string `json:"edge_code"`
	EdgeName  string `json:"edge_name"`
	IPAddress string `json:"ip_address,omitempty"`
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	LastSeen  string `json:"last_seen,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// CreateEdge 注册边端设备
func (s *EdgeService) CreateEdge(ctx context.Context, req *CreateEdgeRequest) (*EdgeResponse, error) {
	builder := s.client.Edge.Create().
		SetEdgeCode(req.EdgeCode).
		SetEdgeName(req.EdgeName).
		SetIPAddress(req.IPAddress).
		SetStatus(edge.StatusOffline)

	if req.Version != "" {
		builder.SetVersion(req.Version)
	}
	if req.Notes != "" {
		builder.SetNotes(req.Notes)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create edge: %w", err)
	}

	return s.toEdgeResponse(created), nil
}

// GetEdgeByCode 按 edge_code 查询设备。
// 未注册时返回 ErrEdgeNotRegistered —— 上传鉴权依赖这个区分。
func (s *EdgeService) GetEdgeByCode(ctx context.Context, code string) (*EdgeResponse, error) {
	found, err := s.client.Edge.Query().
		Where(edge.EdgeCodeEQ(code)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, ErrEdgeNotRegistered
		}
		return nil, fmt.Errorf("query edge: %w", err)
	}

	return s.toEdgeResponse(found), nil
}

// ListEdges 查询设备列表
func (s *EdgeService) ListEdges(ctx context.Context, page, pageSize int, status string) ([]*EdgeResponse, int, error) {
	query := s.client.Edge.Query()

	if status != "" {
		query = query.Where(edge.StatusEQ(edge.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc(edge.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*EdgeResponse, 0, len(rows))
	for _, r := range rows {
		result = append(result, s.toEdgeResponse(r))
	}

	return result, total, nil
}

// UpdateHeartbeat 刷新心跳时间并标记在线
func (s *EdgeService) UpdateHeartbeat(ctx context.Context, code string) error {
	affected, err := s.client.Edge.Update().
		Where(edge.EdgeCodeEQ(code)).
		SetLastSeen(time.Now()).
		SetStatus(edge.StatusOnline).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}
	if affected == 0 {
		return ErrEdgeNotRegistered
	}
	return nil
}

// toEdgeResponse 转换为响应格式
func (s *EdgeService) toEdgeResponse(e *ent.Edge) *EdgeResponse {
	resp := &EdgeResponse{
		ID:        e.ID.String(),
		EdgeCode:  e.EdgeCode,
		EdgeName:  e.EdgeName,
		IPAddress: e.IPAddress,
		Status:    string(e.Status),
		Version:   e.Version,
		CreatedAt: e.CreatedAt.Format(time.RFC3339),
		UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
	}

	if !e.LastSeen.IsZero() {
		resp.LastSeen = e.LastSeen.Format(time.RFC3339)
	}

	return resp
}
