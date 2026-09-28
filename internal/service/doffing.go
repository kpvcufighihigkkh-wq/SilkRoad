package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
)

// DoffingService 落纱操作服务
type DoffingService struct {
	client *ent.Client
}

// NewDoffingService 创建落纱操作服务
func NewDoffingService(client *ent.Client) *DoffingService {
	return &DoffingService{client: client}
}

// CreateDoffingRequest 创建落纱请求
type CreateDoffingRequest struct {
	SpinningLineID   string `json:"spinning_line_id" binding:"required"`
	SpinningPosition int    `json:"spinning_position" binding:"required"`
	LotID            string `json:"lot_id" binding:"required"`
	OperatorID       string `json:"operator_id"`
}

// ConfirmDoffingRequest 确认落纱请求
type ConfirmDoffingRequest struct {
	ActualWeight float64 `json:"actual_weight" binding:"required,gt=0"`
	BobbinNumber string  `json:"bobbin_number" binding:"required"`
	Grade        string  `json:"grade"`
}

// CancelDoffingRequest 取消落纱请求
type CancelDoffingRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// DoffingResponse 落纱响应
type DoffingResponse struct {
	ID               string    `json:"id"`
	SpinningLineID   string    `json:"spinning_line_id"`
	SpinningPosition int       `json:"spinning_position"`
	LotID            string    `json:"lot_id"`
	OperatorID       string    `json:"operator_id,omitempty"`
	Status           string    `json:"status"`
	BobbinNumber     string    `json:"bobbin_number,omitempty"`
	ActualWeight     float64   `json:"actual_weight,omitempty"`
	Grade            string    `json:"grade,omitempty"`
	CancelReason     string    `json:"cancel_reason,omitempty"`
	DoffingTime      time.Time `json:"doffing_time"`
	ConfirmedAt      time.Time `json:"confirmed_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CreateDoffing 创建落纱操作
func (s *DoffingService) CreateDoffing(ctx context.Context, req *CreateDoffingRequest) (*DoffingResponse, error) {
	spinningLineUUID, err := uuid.Parse(req.SpinningLineID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的线体ID"}
	}

	lotUUID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次ID"}
	}

	var operatorUUID uuid.UUID
	if req.OperatorID != "" {
		operatorUUID, err = uuid.Parse(req.OperatorID)
		if err != nil {
			return nil, &ServiceError{Code: 10001, Message: "无效的操作员ID"}
		}
	}

	builder := s.client.Doffing.Create().
		SetSpinningLineID(spinningLineUUID).
		SetSpinningPosition(req.SpinningPosition).
		SetLotID(lotUUID).
		SetStatus(doffing.StatusPending).
		SetDoffingTime(time.Now())

	if req.OperatorID != "" {
		builder.SetOperatorID(operatorUUID)
	}

	d, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toDoffingResponse(d), nil
}

// GetDoffing 获取落纱详情
func (s *DoffingService) GetDoffing(ctx context.Context, id string) (*DoffingResponse, error) {
	doffingUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的落纱ID"}
	}

	d, err := s.client.Doffing.Get(ctx, doffingUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "落纱记录不存在"}
		}
		return nil, err
	}

	return s.toDoffingResponse(d), nil
}

// ListDoffing 查询落纱记录
func (s *DoffingService) ListDoffing(ctx context.Context, page, pageSize int, spinningLineID, status string) ([]*DoffingResponse, int, error) {
	query := s.client.Doffing.Query()

	// 筛选条件
	if spinningLineID != "" {
		lineUUID, err := uuid.Parse(spinningLineID)
		if err == nil {
			query = query.Where(doffing.SpinningLineIDEQ(lineUUID))
		}
	}

	if status != "" {
		query = query.Where(doffing.StatusEQ(doffing.Status(status)))
	}

	// 总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	doffings, err := query.
		Order(ent.Desc(doffing.FieldDoffingTime)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	responses := make([]*DoffingResponse, len(doffings))
	for i, d := range doffings {
		responses[i] = s.toDoffingResponse(d)
	}

	return responses, total, nil
}

// ConfirmDoffing 确认落纱
func (s *DoffingService) ConfirmDoffing(ctx context.Context, id string, req *ConfirmDoffingRequest) error {
	doffingUUID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的落纱ID"}
	}

	d, err := s.client.Doffing.Get(ctx, doffingUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "落纱记录不存在"}
		}
		return err
	}

	if d.Status != doffing.StatusPending {
		return &ServiceError{Code: 20001, Message: "只能确认待处理状态的落纱"}
	}

	builder := d.Update().
		SetStatus(doffing.StatusConfirmed).
		SetActualWeight(req.ActualWeight).
		SetBobbinNumber(req.BobbinNumber).
		SetConfirmedAt(time.Now())

	if req.Grade != "" {
		builder.SetGrade(req.Grade)
	}

	if err := builder.Exec(ctx); err != nil {
		return err
	}

	return nil
}

// CancelDoffing 取消落纱
func (s *DoffingService) CancelDoffing(ctx context.Context, id string, req *CancelDoffingRequest) error {
	doffingUUID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的落纱ID"}
	}

	d, err := s.client.Doffing.Get(ctx, doffingUUID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "落纱记录不存在"}
		}
		return err
	}

	if d.Status == doffing.StatusConfirmed {
		return &ServiceError{Code: 20001, Message: "已确认的落纱不能取消"}
	}

	if err := d.Update().
		SetStatus(doffing.StatusCancelled).
		SetCancelReason(req.Reason).
		Exec(ctx); err != nil {
		return err
	}

	return nil
}

// toDoffingResponse 转换为响应格式
func (s *DoffingService) toDoffingResponse(d *ent.Doffing) *DoffingResponse {
	resp := &DoffingResponse{
		ID:               d.ID.String(),
		SpinningLineID:   d.SpinningLineID.String(),
		SpinningPosition: d.SpinningPosition,
		LotID:            d.LotID.String(),
		Status:           string(d.Status),
		DoffingTime:      d.DoffingTime,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}

	if d.OperatorID != uuid.Nil {
		resp.OperatorID = d.OperatorID.String()
	}

	if d.BobbinNumber != "" {
		resp.BobbinNumber = d.BobbinNumber
	}

	if d.ActualWeight > 0 {
		resp.ActualWeight = d.ActualWeight
	}

	if d.Grade != "" {
		resp.Grade = d.Grade
	}

	if d.CancelReason != "" {
		resp.CancelReason = d.CancelReason
	}

	if !d.ConfirmedAt.IsZero() {
		resp.ConfirmedAt = d.ConfirmedAt
	}

	return resp
}
