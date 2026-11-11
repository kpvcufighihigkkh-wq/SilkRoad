package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
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

// CreateLotRequest 创建批次请求（Edge 侧）。
type CreateLotRequest struct {
	LotNumber       string `json:"lot_number" binding:"required,max=50"`
	ProductType     string `json:"product_type" binding:"required"`
	ProductSpec     string `json:"product_spec"`
	OrderCode       string `json:"order_code"`
	PlannedQuantity int    `json:"planned_quantity" binding:"required,min=1"`
}

// UpdateLotStatusRequest 批次状态流转请求。
//
// 只允许状态与实产量变化：批次号、计划量是投产时定下的，改它等于换一个批次。
type UpdateLotStatusRequest struct {
	Status         string `json:"status" binding:"required"`
	ActualQuantity *int   `json:"actual_quantity"`
}

// CreateLot 在 Edge 侧创建批次，并把开工时刻写定。
//
// start_time 是看板一（生产概览）的区间过滤字段。此前全仓库没有任何代码
// 写它，导致该看板无法交付；批次一旦创建即视为投产，故在这里落时间，
// 而不是等第一次落纱 —— 落纱可能晚于开工很久，也可能一次都没有。
func (s *LotService) CreateLot(ctx context.Context, req *CreateLotRequest) (*LotResponse, error) {
	productType := lot.ProductType(req.ProductType)
	if err := productTypeValidator(productType); err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的产品类型：" + req.ProductType}
	}

	exists, err := s.client.Lot.Query().Where(lot.LotNumberEQ(req.LotNumber)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, &ServiceError{Code: 40002, Message: "批次号已存在：" + req.LotNumber}
	}

	builder := s.client.Lot.Create().
		SetLotNumber(req.LotNumber).
		SetProductType(productType).
		SetPlannedQuantity(req.PlannedQuantity).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		SetStartTime(time.Now())

	if req.ProductSpec != "" {
		builder.SetProductSpec(req.ProductSpec)
	}
	if req.OrderCode != "" {
		builder.SetOrderCode(req.OrderCode)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, &ServiceError{Code: 40002, Message: "批次号已存在：" + req.LotNumber}
		}
		return nil, err
	}

	return s.toLotResponse(row), nil
}

// UpdateLotStatus 推进批次状态，并在状态切换时写定开始/结束时刻。
//
// 时间戳只在**首次**进入对应状态时写，不覆盖：
//   - 进入 in_progress 且 start_time 为空 → 写 start_time；
//   - 进入 completed / cancelled 且 end_time 为空 → 写 end_time。
//
// 重复提交同一状态是幂等的，不会把已记录的时刻推后。
func (s *LotService) UpdateLotStatus(ctx context.Context, id string, req *UpdateLotStatusRequest) (*LotResponse, error) {
	lotID, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次ID"}
	}

	status := lot.Status(req.Status)
	if err := lot.StatusValidator(status); err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次状态：" + req.Status}
	}

	current, err := s.client.Lot.Get(ctx, lotID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "批次不存在"}
		}
		return nil, err
	}

	builder := s.client.Lot.UpdateOneID(lotID).SetStatus(status)

	if status == lot.StatusInProgress && current.StartTime.IsZero() {
		builder.SetStartTime(time.Now())
	}
	if (status == lot.StatusCompleted || status == lot.StatusCancelled) && current.EndTime.IsZero() {
		builder.SetEndTime(time.Now())
	}
	if req.ActualQuantity != nil {
		builder.SetActualQuantity(*req.ActualQuantity)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toLotResponse(row), nil
}

// productTypeValidator 校验产品类型枚举。
func productTypeValidator(t lot.ProductType) error {
	return lot.ProductTypeValidator(t)
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

// ListLots 查询批次列表。
//
// 筛选参数全部可选，空串表示不过滤。keyword 模糊匹配 lot_number。
func (s *LotService) ListLots(
	ctx context.Context,
	page, pageSize int,
	edgeID, status, orderCode, productType, keyword string,
) ([]*LotResponse, int, error) {
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

	if orderCode != "" {
		query = query.Where(lot.OrderCodeEQ(orderCode))
	}

	if productType != "" {
		query = query.Where(lot.ProductTypeEQ(lot.ProductType(productType)))
	}

	if keyword != "" {
		// ent 的 ContainsFold 内部已转义 LIKE 元字符，此处直接传原值。
		query = query.Where(lot.LotNumberContainsFold(keyword))
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
