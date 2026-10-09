package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
)

// BobbinService 丝锭服务
type BobbinService struct {
	client *ent.Client
}

// NewBobbinService 创建丝锭服务
func NewBobbinService(client *ent.Client) *BobbinService {
	return &BobbinService{
		client: client,
	}
}

// CreateBobbinRequest 创建丝锭请求
type CreateBobbinRequest struct {
	BobbinNumber     string  `json:"bobbin_number" binding:"required"`
	LotID            string  `json:"lot_id" binding:"required"`
	SpinningPosition int     `json:"spinning_position" binding:"required,min=1"`
	GrossWeight      float64 `json:"gross_weight" binding:"required"`
	NetWeight        float64 `json:"net_weight" binding:"required"`
	TareWeight       float64 `json:"tare_weight"`
	Grade            string  `json:"grade"`
}

// BobbinResponse 丝锭响应
type BobbinResponse struct {
	ID               string  `json:"id"`
	BobbinNumber     string  `json:"bobbin_number"`
	LotID            string  `json:"lot_id"`
	SpinningPosition int     `json:"spinning_position"`
	GrossWeight      float64 `json:"gross_weight"`
	NetWeight        float64 `json:"net_weight"`
	TareWeight       float64 `json:"tare_weight"`
	Grade            string  `json:"grade,omitempty"`
	Status           string  `json:"status"`
	LabelPrinted     bool    `json:"label_printed"`
	PrintedAt        *string `json:"printed_at,omitempty"`
	CompletedAt      *string `json:"completed_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

// CreateBobbin 创建丝锭
func (s *BobbinService) CreateBobbin(ctx context.Context, req *CreateBobbinRequest) (*BobbinResponse, error) {
	lotID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, err
	}

	builder := s.client.Bobbin.Create().
		SetBobbinNumber(req.BobbinNumber).
		SetLotID(lotID).
		SetSpinningPosition(req.SpinningPosition).
		SetGrossWeight(req.GrossWeight).
		SetNetWeight(req.NetWeight).
		SetStatus(bobbin.Status("producing")).
		SetLabelPrinted(false)

	if req.TareWeight > 0 {
		builder.SetTareWeight(req.TareWeight)
	}

	if req.Grade != "" {
		builder.SetGrade(req.Grade)
	}

	bobbin, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	return s.toBobbinResponse(bobbin), nil
}

// GetBobbin 获取丝锭详情
func (s *BobbinService) GetBobbin(ctx context.Context, id string) (*BobbinResponse, error) {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	bobbin, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		return nil, err
	}

	return s.toBobbinResponse(bobbin), nil
}

// ListBobbins 查询丝锭列表
func (s *BobbinService) ListBobbins(ctx context.Context, page, pageSize int, lotID, status string) ([]*BobbinResponse, int, error) {
	query := s.client.Bobbin.Query()

	// 过滤条件
	if lotID != "" {
		id, err := uuid.Parse(lotID)
		if err == nil {
			query = query.Where(bobbin.LotIDEQ(id))
		}
	}

	if status != "" {
		query = query.Where(bobbin.StatusEQ(bobbin.Status(status)))
	}

	// 查询总数
	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	bobbins, err := query.
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order(ent.Desc("created_at")).
		All(ctx)

	if err != nil {
		return nil, 0, err
	}

	var result []*BobbinResponse
	for _, b := range bobbins {
		result = append(result, s.toBobbinResponse(b))
	}

	return result, total, nil
}

// UpdateBobbinStatus 更新丝锭状态
func (s *BobbinService) UpdateBobbinStatus(ctx context.Context, id, status string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Bobbin.UpdateOneID(bobbinID).
		SetStatus(bobbin.Status(status)).
		Exec(ctx)
}

// MarkBobbinPrinted 标记丝锭已打印
func (s *BobbinService) MarkBobbinPrinted(ctx context.Context, id string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Bobbin.UpdateOneID(bobbinID).
		SetLabelPrinted(true).
		Exec(ctx)
}

// DeleteBobbin 删除丝锭
func (s *BobbinService) DeleteBobbin(ctx context.Context, id string) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	return s.client.Bobbin.DeleteOneID(bobbinID).Exec(ctx)
}

// toBobbinResponse 转换为响应格式
func (s *BobbinService) toBobbinResponse(b *ent.Bobbin) *BobbinResponse {
	resp := &BobbinResponse{
		ID:               b.ID.String(),
		BobbinNumber:     b.BobbinNumber,
		LotID:            b.LotID.String(),
		SpinningPosition: b.SpinningPosition,
		GrossWeight:      b.GrossWeight,
		NetWeight:        b.NetWeight,
		TareWeight:       b.TareWeight,
		Status:           string(b.Status),
		LabelPrinted:     b.LabelPrinted,
		CreatedAt:        b.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        b.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if b.Grade != "" {
		resp.Grade = b.Grade
	}

	if !b.PrintedAt.IsZero() {
		str := b.PrintedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.PrintedAt = &str
	}

	if !b.CompletedAt.IsZero() {
		str := b.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CompletedAt = &str
	}

	return resp
}

// WeighBobbinRequest 丝锭称重请求
type WeighBobbinRequest struct {
	GrossWeight float64 `json:"gross_weight" binding:"required,gt=0"`
	TareWeight  float64 `json:"tare_weight" binding:"required,gte=0"`
	NetWeight   float64 `json:"net_weight" binding:"required,gt=0"`
}

// InspectBobbinRequest 丝锭质检请求
type InspectBobbinRequest struct {
	Grade       string `json:"grade" binding:"required"`
	InspectorID string `json:"inspector_id"`
	DefectNote  string `json:"defect_note"`
}

// WeighBobbin 丝锭称重
func (s *BobbinService) WeighBobbin(ctx context.Context, id string, req *WeighBobbinRequest) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	// 验证重量逻辑：净重 = 毛重 - 皮重
	expectedNet := req.GrossWeight - req.TareWeight
	if abs(expectedNet-req.NetWeight) > 0.01 { // 允许0.01kg误差
		return &ServiceError{Code: 10001, Message: "重量数据不一致：净重应等于毛重减去皮重"}
	}

	return b.Update().
		SetGrossWeight(req.GrossWeight).
		SetTareWeight(req.TareWeight).
		SetNetWeight(req.NetWeight).
		Exec(ctx)
}

// InspectBobbin 丝锭质检
func (s *BobbinService) InspectBobbin(ctx context.Context, id string, req *InspectBobbinRequest) error {
	bobbinID, err := uuid.Parse(id)
	if err != nil {
		return &ServiceError{Code: 10001, Message: "无效的丝锭ID"}
	}

	b, err := s.client.Bobbin.Get(ctx, bobbinID)
	if err != nil {
		if ent.IsNotFound(err) {
			return &ServiceError{Code: 40001, Message: "丝锭不存在"}
		}
		return err
	}

	// 质检员与缺陷原因是可选字段：空串时传 nil，让列保持 NULL。
	var inspectorID *uuid.UUID
	if req.InspectorID != "" {
		parsed, err := uuid.Parse(req.InspectorID)
		if err != nil {
			return &ServiceError{Code: 10001, Message: "无效的质检员ID"}
		}
		inspectorID = &parsed
	}
	var defectReason *string
	if req.DefectNote != "" {
		defectReason = &req.DefectNote
	}

	if err := b.Update().SetGrade(req.Grade).Exec(ctx); err != nil {
		return err
	}

	// 保留完整质检历史（每次质检新增一条），同时把最新等级同步到 bobbin.grade。
	// 不覆盖历史：缺陷原因、评分等在现场只产生一次，丢失即不可恢复。
	if _, err := s.client.BobbinGrade.Create().
		SetBobbinID(bobbinID).
		SetGrade(req.Grade).
		SetNillableInspectorID(inspectorID).
		SetNillableDefectReason(defectReason).
		Save(ctx); err != nil {
		return err
	}

	return nil
}

// abs 返回浮点数的绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
