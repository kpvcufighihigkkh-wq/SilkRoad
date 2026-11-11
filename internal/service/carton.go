package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
)

// CartonService 纸箱服务。
//
// **纸箱是 DTY 的专属包装环节**：POY/FDY 丝锭直接码到托盘上，DTY 则是
// 「装箱 → 按 8 箱/托盘码垛」。因此本服务拒绝非 DTY 批次，避免把一个
// 不存在的工艺硬套到产线上 —— 那会造出 Center 里谁也解释不了的包装数据。
//
// 一个丝锭的上级容器只能有一个：已装桶或已码盘的丝锭不得再装箱。
// 这条互斥在托盘与桶两侧各写了一次，三处合起来才是完整的约束。
type CartonService struct {
	client *ent.Client
}

// NewCartonService 创建纸箱服务
func NewCartonService(client *ent.Client) *CartonService {
	return &CartonService{client: client}
}

// CreateCartonRequest 创建纸箱请求。
//
// CartonNumber 可省略：产线边装箱边生成编号。
type CreateCartonRequest struct {
	LotID        string `json:"lot_id" binding:"required"`
	CartonNumber string `json:"carton_number"`
	Capacity     int    `json:"capacity"`
}

// PackIntoCartonRequest 把丝锭装进纸箱。
type PackIntoCartonRequest struct {
	BobbinIDs []string `json:"bobbin_ids" binding:"required,min=1"`
}

// SealCartonRequest 封箱。
//
// 总重量可省略：由现场称重设备给出，省略时按箱内丝锭净重累加。
type SealCartonRequest struct {
	TotalWeight *float64 `json:"total_weight"`
}

// CartonResponse 纸箱响应
type CartonResponse struct {
	ID           string    `json:"id"`
	CartonNumber string    `json:"carton_number"`
	LotID        *string   `json:"lot_id"`
	PalletID     *string   `json:"pallet_id"`
	BobbinCount  int       `json:"bobbin_count"`
	Capacity     int       `json:"capacity"`
	TotalWeight  *float64  `json:"total_weight"`
	Status       string    `json:"status"`
	LabelPrinted bool      `json:"label_printed"`
	PrintedAt    *string   `json:"printed_at"`
	PackedAt     *string   `json:"packed_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateCarton 创建纸箱，初始状态 packing。
func (s *CartonService) CreateCarton(ctx context.Context, req *CreateCartonRequest) (*CartonResponse, error) {
	lotID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次ID"}
	}

	parent, err := s.client.Lot.Get(ctx, lotID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "批次不存在"}
		}
		return nil, err
	}
	// DTY 专属：POY/FDY 走直接码锭，不该出现纸箱。
	if parent.ProductType != lot.ProductTypeDTY {
		return nil, &ServiceError{Code: 30001,
			Message: "纸箱只适用于 DTY 批次，当前批次产品类型为 " + string(parent.ProductType)}
	}

	capacity := req.Capacity
	if capacity == 0 {
		capacity = 12
	}
	if capacity < 1 || capacity > 99 {
		return nil, &ServiceError{Code: 10001, Message: "容量必须在 1-99 之间"}
	}

	number := req.CartonNumber
	if number == "" {
		number = generateCartonNumber()
	}

	row, err := s.client.Carton.Create().
		SetCartonNumber(number).
		SetLotID(lotID).
		SetBobbinCount(0).
		SetCapacity(capacity).
		SetStatus(carton.StatusPacking).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, &ServiceError{Code: 40002, Message: "纸箱编号已存在：" + number}
		}
		return nil, err
	}

	return toCartonResponse(row), nil
}

// PackBobbins 把丝锭装进纸箱。
//
// 校验：
//   - 纸箱必须是 packing，装载后不超过容量；
//   - 丝锭与纸箱同批次；
//   - 丝锭不得已码盘、不得已装桶、不得已进别的箱 —— 一个丝锭只能有一个上级容器；
//   - 已在本箱内的丝锭重复提交是幂等的。
func (s *CartonService) PackBobbins(ctx context.Context, cartonID string, req *PackIntoCartonRequest) (*CartonResponse, error) {
	cid, err := uuid.Parse(cartonID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的纸箱ID"}
	}
	if len(req.BobbinIDs) == 0 {
		return nil, &ServiceError{Code: 10001, Message: "至少需要一个丝锭"}
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	target, err := tx.Carton.Get(ctx, cid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "纸箱不存在"}
		}
		return nil, err
	}
	if target.Status != carton.StatusPacking {
		return nil, &ServiceError{Code: 30001, Message: "只有 packing 状态的纸箱可以装箱"}
	}

	ids := make([]uuid.UUID, 0, len(req.BobbinIDs))
	for _, raw := range req.BobbinIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, &ServiceError{Code: 10001, Message: "无效的丝锭ID：" + raw}
		}
		ids = append(ids, id)
	}

	rows, err := tx.Bobbin.Query().Where(bobbin.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, &ServiceError{Code: 40001, Message: "存在无效的丝锭ID"}
	}

	added := 0
	for _, b := range rows {
		if b.LotID != target.LotID {
			return nil, &ServiceError{Code: 30001, Message: "丝锭与纸箱不属于同一批次"}
		}
		if b.PalletID != uuid.Nil {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已码在托盘上，不能装箱"}
		}
		if b.BarrelID != uuid.Nil {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已装在桶上，不能装箱"}
		}
		if b.CartonID != uuid.Nil && b.CartonID != cid {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已装在其它纸箱上"}
		}
		if b.CartonID == cid {
			continue // 幂等
		}
		added++
	}

	if target.BobbinCount+added > target.Capacity {
		return nil, &ServiceError{Code: 30001,
			Message: fmt.Sprintf("超出纸箱容量：当前 %d + 新增 %d > 上限 %d",
				target.BobbinCount, added, target.Capacity)}
	}

	for _, b := range rows {
		if b.CartonID == cid {
			continue
		}
		if _, err := tx.Bobbin.UpdateOneID(b.ID).
			SetCartonID(cid).
			// 归属变更必须重新排队同步。
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx); err != nil {
			return nil, err
		}
	}

	updated, err := tx.Carton.UpdateOneID(cid).
		SetBobbinCount(target.BobbinCount + added).
		SetSyncStatus(carton.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return toCartonResponse(updated), nil
}

// SealCarton 封箱：写入总重量并推进到 packed。
//
// 总重量省略时按箱内丝锭净重累加 —— 与托盘「毛重-皮重」不同，
// 纸箱的净重就是内容物净重，不需要皮重。
func (s *CartonService) SealCarton(ctx context.Context, cartonID string, req *SealCartonRequest) (*CartonResponse, error) {
	cid, err := uuid.Parse(cartonID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的纸箱ID"}
	}

	target, err := s.client.Carton.Get(ctx, cid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "纸箱不存在"}
		}
		return nil, err
	}
	if target.Status != carton.StatusPacking {
		return nil, &ServiceError{Code: 30001, Message: "只有 packing 状态的纸箱可以封箱"}
	}
	if target.BobbinCount == 0 {
		return nil, &ServiceError{Code: 30001, Message: "空纸箱不能封箱"}
	}

	weight := 0.0
	if req.TotalWeight != nil {
		if *req.TotalWeight < 0 {
			return nil, &ServiceError{Code: 10001, Message: "总重量不能为负"}
		}
		weight = *req.TotalWeight
	} else {
		// 省略时按内容物净重累加：箱子本身不称，与托盘的毛皮重口径不同。
		sum, err := s.client.Bobbin.Query().
			Where(bobbin.CartonIDEQ(cid)).
			Aggregate(ent.Sum(bobbin.FieldNetWeight)).
			Float64(ctx)
		if err != nil {
			return nil, err
		}
		weight = sum
	}

	updated, err := s.client.Carton.UpdateOneID(cid).
		SetTotalWeight(weight).
		SetStatus(carton.StatusPacked).
		SetPackedAt(time.Now()).
		SetSyncStatus(carton.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return toCartonResponse(updated), nil
}

// GetCarton 获取纸箱详情。
func (s *CartonService) GetCarton(ctx context.Context, id string) (*CartonResponse, error) {
	cid, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的纸箱ID"}
	}

	row, err := s.client.Carton.Get(ctx, cid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "纸箱不存在"}
		}
		return nil, err
	}

	return toCartonResponse(row), nil
}

// ListCartons 按批次 / 状态 / 托盘分页查询纸箱，按创建时间倒序。
func (s *CartonService) ListCartons(ctx context.Context, page, pageSize int, lotID, status, palletID string) ([]*CartonResponse, int, error) {
	query := s.client.Carton.Query()

	if lotID != "" {
		id, err := uuid.Parse(lotID)
		if err != nil {
			return nil, 0, &ServiceError{Code: 10001, Message: "无效的批次ID"}
		}
		query = query.Where(carton.LotIDEQ(id))
	}
	if palletID != "" {
		id, err := uuid.Parse(palletID)
		if err != nil {
			return nil, 0, &ServiceError{Code: 10001, Message: "无效的托盘ID"}
		}
		query = query.Where(carton.PalletIDEQ(id))
	}
	if status != "" {
		query = query.Where(carton.StatusEQ(carton.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(carton.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*CartonResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCartonResponse(r))
	}

	return out, total, nil
}

// generateCartonNumber 生成纸箱编号：CTN-<日期>-<随机段>。
func generateCartonNumber() string {
	return fmt.Sprintf("CTN-%s-%s",
		time.Now().Format("20060102"),
		uuid.NewString()[:8],
	)
}

func toCartonResponse(c *ent.Carton) *CartonResponse {
	resp := &CartonResponse{
		ID:           c.ID.String(),
		CartonNumber: c.CartonNumber,
		BobbinCount:  c.BobbinCount,
		Capacity:     c.Capacity,
		Status:       string(c.Status),
		LabelPrinted: c.LabelPrinted,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}

	// lot_id / pallet_id 在 schema 里都是 Optional：零值即「未设置」，转 JSON null。
	if c.LotID != uuid.Nil {
		v := c.LotID.String()
		resp.LotID = &v
	}
	if c.PalletID != uuid.Nil {
		v := c.PalletID.String()
		resp.PalletID = &v
	}
	if c.TotalWeight != 0 {
		v := c.TotalWeight
		resp.TotalWeight = &v
	}
	if !c.PrintedAt.IsZero() {
		v := c.PrintedAt.Format(time.RFC3339)
		resp.PrintedAt = &v
	}
	if !c.PackedAt.IsZero() {
		v := c.PackedAt.Format(time.RFC3339)
		resp.PackedAt = &v
	}

	return resp
}
