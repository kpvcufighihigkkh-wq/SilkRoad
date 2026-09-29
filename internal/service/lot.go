package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/google/uuid"
)

// LotService 批次服务
type LotService struct {
	client *ent.Client
}

// NewLotService 创建批次服务
func NewLotService(client *ent.Client) *LotService {
	return &LotService{
		client: client,
	}
}

// CreateLotRequest 创建批次请求
type CreateLotRequest struct {
	LotNumber       string `json:"lot_number" binding:"required,max=50"`
	EdgeID          string `json:"edge_id" binding:"omitempty,uuid"`
	PLCLotNumber    string `json:"plc_lot_number" binding:"omitempty,max=50"`
	OrderCode       string `json:"order_code" binding:"omitempty,max=50"`
	ProductType     string `json:"product_type" binding:"required,oneof=FDY POY DTY"`
	ProductSpec     string `json:"product_spec" binding:"omitempty,max=100"`
	PlannedQuantity int    `json:"planned_quantity" binding:"required,min=1"`
}

// UpdateLotRequest 更新批次请求
type UpdateLotRequest struct {
	ProductSpec     *string `json:"product_spec" binding:"omitempty,max=100"`
	PlannedQuantity *int    `json:"planned_quantity" binding:"omitempty,min=1"`
}

// LotResponse 批次响应
type LotResponse struct {
	ID              string  `json:"id"`
	LotNumber       string  `json:"lot_number"`
	EdgeID          string  `json:"edge_id,omitempty"`
	PLCLotNumber    string  `json:"plc_lot_number,omitempty"`
	OrderCode       string  `json:"order_code,omitempty"`
	ProductType     string  `json:"product_type"`
	ProductSpec     string  `json:"product_spec,omitempty"`
	Status          string  `json:"status"`
	PlannedQuantity int     `json:"planned_quantity"`
	ActualQuantity  int     `json:"actual_quantity"`
	Progress        float64 `json:"progress"`
	StartTime       *string `json:"start_time,omitempty"`
	EndTime         *string `json:"end_time,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// CreateLot 创建批次
func (s *LotService) CreateLot(ctx context.Context, req *CreateLotRequest) (*LotResponse, error) {
	builder := s.client.Lot.Create().
		SetLotNumber(req.LotNumber).
		SetProductType(lot.ProductType(req.ProductType)).
		SetPlannedQuantity(req.PlannedQuantity).
		SetActualQuantity(0).
		SetStatus(lot.Status("in_progress"))

	if req.EdgeID != "" {
		edgeID, err := uuid.Parse(req.EdgeID)
		if err != nil {
			return nil, fmt.Errorf("invalid edge_id: %w", err)
		}
		builder.SetEdgeID(edgeID)
	}

	if req.PLCLotNumber != "" {
		builder.SetPlcLotNumber(req.PLCLotNumber)
	}
	if req.OrderCode != "" {
		builder.SetOrderCode(req.OrderCode)
	}
	if req.ProductSpec != "" {
		builder.SetProductSpec(req.ProductSpec)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toLotResponse(created), nil
}

// GetLot 获取批次详情
func (s *LotService) GetLot(ctx context.Context, id string) (*LotResponse, error) {
	lotID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	lot, err := s.client.Lot.Get(ctx, lotID)
	if err != nil {
		return nil, err
	}

	return s.toLotResponse(lot), nil
}

// ListLots 查询批次列表
func (s *LotService) ListLots(ctx context.Context, page, pageSize int, edgeID, status string) ([]*LotResponse, int, error) {
	query := s.client.Lot.Query()

	if edgeID != "" {
		id, err := uuid.Parse(edgeID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid edge_id: %w", err)
		}
		query = query.Where(lot.EdgeIDEQ(id))
	}

	if status != "" {
		query = query.Where(lot.StatusEQ(lot.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	lots, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc(lot.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*LotResponse, 0, len(lots))
	for _, l := range lots {
		result = append(result, s.toLotResponse(l))
	}

	return result, total, nil
}

// UpdateLotStatus 更新批次状态
func (s *LotService) UpdateLotStatus(ctx context.Context, id, status string) error {
	lotID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Lot.UpdateOneID(lotID).
		SetStatus(lot.Status(status)).
		Exec(ctx)
}

// DeleteLot 删除批次
func (s *LotService) DeleteLot(ctx context.Context, id string) error {
	lotID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Lot.DeleteOneID(lotID).Exec(ctx)
}

// toLotResponse 转换为响应格式
func (s *LotService) toLotResponse(l *ent.Lot) *LotResponse {
	var progress float64
	if l.PlannedQuantity > 0 {
		progress = float64(l.ActualQuantity) / float64(l.PlannedQuantity) * 100
	}

	resp := &LotResponse{
		ID:              l.ID.String(),
		LotNumber:       l.LotNumber,
		PLCLotNumber:    l.PlcLotNumber,
		OrderCode:       l.OrderCode,
		ProductType:     string(l.ProductType),
		ProductSpec:     l.ProductSpec,
		Status:          string(l.Status),
		PlannedQuantity: l.PlannedQuantity,
		ActualQuantity:  l.ActualQuantity,
		Progress:        progress,
		CreatedAt:       l.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       l.UpdatedAt.Format(time.RFC3339),
	}

	if l.EdgeID != uuid.Nil {
		resp.EdgeID = l.EdgeID.String()
	}
	if !l.StartTime.IsZero() {
		str := l.StartTime.Format(time.RFC3339)
		resp.StartTime = &str
	}
	if !l.EndTime.IsZero() {
		str := l.EndTime.Format(time.RFC3339)
		resp.EndTime = &str
	}

	return resp
}
