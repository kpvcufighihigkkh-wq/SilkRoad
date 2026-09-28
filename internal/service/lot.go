package service

import (
	"context"

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
	LotNumber       string `json:"lot_number" binding:"required"`
	OrderID         string `json:"order_id" binding:"required"`
	ProductType     string `json:"product_type" binding:"required"`
	ProductSpec     string `json:"product_spec" binding:"required"`
	PlannedQuantity int    `json:"planned_quantity" binding:"required,min=1"`
}

// LotResponse 批次响应
type LotResponse struct {
	ID              string  `json:"id"`
	LotNumber       string  `json:"lot_number"`
	OrderID         string  `json:"order_id"`
	ProductType     string  `json:"product_type"`
	ProductSpec     string  `json:"product_spec"`
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
	orderID, err := uuid.Parse(req.OrderID)
	if err != nil {
		return nil, err
	}

	lot, err := s.client.Lot.Create().
		SetLotNumber(req.LotNumber).
		SetOrderID(orderID).
		SetProductType(lot.ProductType(req.ProductType)).
		SetProductSpec(req.ProductSpec).
		SetPlannedQuantity(req.PlannedQuantity).
		SetActualQuantity(0).
		SetStatus(lot.Status("in_progress")).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	return s.toLotResponse(lot), nil
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
func (s *LotService) ListLots(ctx context.Context, page, pageSize int, orderID, status string) ([]*LotResponse, int, error) {
	query := s.client.Lot.Query()

	// 过滤条件
	if orderID != "" {
		id, err := uuid.Parse(orderID)
		if err == nil {
			query = query.Where(lot.OrderIDEQ(id))
		}
	}

	if status != "" {
		query = query.Where(lot.StatusEQ(lot.Status(status)))
	}

	// 查询总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	lots, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc("created_at")).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	var result []*LotResponse
	for _, lot := range lots {
		result = append(result, s.toLotResponse(lot))
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
func (s *LotService) toLotResponse(lot *ent.Lot) *LotResponse {
	var progress float64
	if lot.PlannedQuantity > 0 {
		progress = float64(lot.ActualQuantity) / float64(lot.PlannedQuantity) * 100
	}

	resp := &LotResponse{
		ID:              lot.ID.String(),
		LotNumber:       lot.LotNumber,
		OrderID:         lot.OrderID.String(),
		ProductType:     string(lot.ProductType),
		ProductSpec:     lot.ProductSpec,
		Status:          string(lot.Status),
		PlannedQuantity: lot.PlannedQuantity,
		ActualQuantity:  lot.ActualQuantity,
		Progress:        progress,
		CreatedAt:       lot.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       lot.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if !lot.StartTime.IsZero() {
		str := lot.StartTime.Format("2006-01-02T15:04:05Z07:00")
		resp.StartTime = &str
	}

	if !lot.EndTime.IsZero() {
		str := lot.EndTime.Format("2006-01-02T15:04:05Z07:00")
		resp.EndTime = &str
	}

	return resp
}
