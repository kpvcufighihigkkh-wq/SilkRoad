package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
)

// BarrelService 落纱桶服务。
//
// 桶是落纱工位上的载具：单个落纱操作产出的丝锭按锭位（1-9）装进桶，
// 装满后整桶流转到下一工段。与托盘一样，创建与装桶都由 Edge 产生，
// Center 只读。
//
// 与托盘的区别：桶的容量是硬上限（capacity，默认 9），装满即不可再装，
// 且同一锭位上不能同时有两枚丝锭 —— 后者是现场「一个锭位放一枚」的
// 物理约束，schema 层只是普通复合索引，只能在这里守。
type BarrelService struct {
	client *ent.Client
}

// NewBarrelService 创建落纱桶服务
func NewBarrelService(client *ent.Client) *BarrelService {
	return &BarrelService{client: client}
}

// CreateBarrelRequest 创建桶请求。
//
// BarrelNumber 可省略：产线边建桶边生成编号，与托盘同一约定。
type CreateBarrelRequest struct {
	LotID        string `json:"lot_id" binding:"required"`
	BarrelNumber string `json:"barrel_number"`
	Capacity     int    `json:"capacity"`
}

// LoadBobbinsRequest 往桶里装丝锭。
//
// 丝锭的锭位由 SpinPositions 显式给出：桶内锭位是物理位置，
// 不能从丝锭自身的 spinning_position 推断（那是线体位号，不是桶内位号）。
type LoadBobbinsRequest struct {
	// BobbinIDs 与 SpinPositions 必须一一对应，长度相同。
	BobbinIDs     []string `json:"bobbin_ids" binding:"required,min=1"`
	SpinPositions []int    `json:"spin_positions" binding:"required,min=1"`
}

// SealBarrelRequest 装满封桶。
//
// 不要求恰好装满 capacity 枚：现场可能因缺料提前封桶，
// 由操作员显式确认，而不是系统替人做主。
type SealBarrelRequest struct{}

// BarrelResponse 桶响应
type BarrelResponse struct {
	ID           string    `json:"id"`
	BarrelNumber string    `json:"barrel_number"`
	LotID        string    `json:"lot_id"`
	Capacity     int       `json:"capacity"`
	CurrentCount int       `json:"current_count"`
	Status       string    `json:"status"`
	FilledAt     *string   `json:"filled_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateBarrel 创建桶，初始状态 active。
func (s *BarrelService) CreateBarrel(ctx context.Context, req *CreateBarrelRequest) (*BarrelResponse, error) {
	lotID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次ID"}
	}

	if _, err := s.client.Lot.Get(ctx, lotID); err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "批次不存在"}
		}
		return nil, err
	}

	capacity := req.Capacity
	if capacity == 0 {
		capacity = 9
	}
	if capacity < 1 || capacity > 99 {
		return nil, &ServiceError{Code: 10001, Message: "容量必须在 1-99 之间"}
	}

	number := req.BarrelNumber
	if number == "" {
		number = generateBarrelNumber()
	}

	row, err := s.client.Barrel.Create().
		SetBarrelNumber(number).
		SetLotID(lotID).
		SetCapacity(capacity).
		SetCurrentCount(0).
		SetStatus(barrel.StatusActive).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, &ServiceError{Code: 40002, Message: "桶编号已存在：" + number}
		}
		return nil, err
	}

	return toBarrelResponse(row), nil
}

// LoadBobbins 把丝锭装进桶。
//
// 校验：
//   - 桶必须是 active，且装载后不超过容量；
//   - 丝锭与桶同批次；
//   - 丝锭不得已装进别的桶、不得已码盘（一个丝锭只能有一个上级容器）；
//   - 同一桶内锭位不得重复，包括与已装载丝锭的锭位冲突。
//
// 同一批丝锭重复提交是幂等的（已在桶内且锭位一致的跳过）。
//
// 装满（current_count == capacity）时自动转 full 并写 filled_at ——
// 那是客观事实，不需要操作员再点一次确认。
func (s *BarrelService) LoadBobbins(ctx context.Context, barrelID string, req *LoadBobbinsRequest) (*BarrelResponse, error) {
	bid, err := uuid.Parse(barrelID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的桶ID"}
	}

	if len(req.BobbinIDs) == 0 {
		return nil, &ServiceError{Code: 10001, Message: "至少需要一个丝锭"}
	}
	if len(req.SpinPositions) != len(req.BobbinIDs) {
		return nil, &ServiceError{Code: 10001, Message: "丝锭数量与锭位数量不一致"}
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	target, err := tx.Barrel.Get(ctx, bid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "桶不存在"}
		}
		return nil, err
	}
	if target.Status != barrel.StatusActive {
		return nil, &ServiceError{Code: 30001, Message: "只有 active 状态的桶可以装锭"}
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

	// 已占用锭位：桶里已有的 + 本批请求内重复的。两者都会让物理位置冲突。
	occupied := make(map[int]string, target.CurrentCount)
	existing, err := tx.Bobbin.Query().
		Where(bobbin.BarrelIDEQ(bid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range existing {
		if b.BarrelPosition > 0 {
			occupied[b.BarrelPosition] = b.BobbinNumber
		}
	}

	added := 0
	for i, b := range rows {
		pos := req.SpinPositions[i]
		if pos < 1 || pos > target.Capacity {
			return nil, &ServiceError{Code: 10001,
				Message: fmt.Sprintf("锭位 %d 超出桶容量 1-%d", pos, target.Capacity)}
		}
		if b.LotID != target.LotID {
			return nil, &ServiceError{Code: 30001, Message: "丝锭与桶不属于同一批次"}
		}
		// 一个丝锭只能有一个上级容器：已码盘的丝锭装桶会让托盘计数与实物脱节。
		if b.PalletID != uuid.Nil {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已码在托盘上，不能装桶"}
		}
		if b.BarrelID != uuid.Nil && b.BarrelID != bid {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已装在其它桶上"}
		}
		if b.BarrelID == bid && b.BarrelPosition == pos {
			continue // 幂等：已在桶内且锭位一致
		}
		if other, taken := occupied[pos]; taken && other != b.BobbinNumber {
			return nil, &ServiceError{Code: 30001,
				Message: fmt.Sprintf("锭位 %d 已被丝锭 %s 占用", pos, other)}
		}

		if _, err := tx.Bobbin.UpdateOneID(b.ID).
			SetBarrelID(bid).
			SetBarrelPosition(pos).
			// 归属变更必须重新排队同步，否则 Center 看不到桶关联。
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx); err != nil {
			return nil, err
		}
		occupied[pos] = b.BobbinNumber
		added++
	}

	count := target.CurrentCount + added

	builder := tx.Barrel.UpdateOneID(bid).
		SetCurrentCount(count).
		SetSyncStatus(barrel.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt()

	if count >= target.Capacity {
		builder.SetStatus(barrel.StatusFull)
		if target.FilledAt.IsZero() {
			builder.SetFilledAt(time.Now())
		}
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return toBarrelResponse(updated), nil
}

// SealBarrel 封桶：把桶推进到 packed，表示已完成落纱流转。
func (s *BarrelService) SealBarrel(ctx context.Context, barrelID string) (*BarrelResponse, error) {
	bid, err := uuid.Parse(barrelID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的桶ID"}
	}

	target, err := s.client.Barrel.Get(ctx, bid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "桶不存在"}
		}
		return nil, err
	}

	// 空桶封桶没有意义，账实不符。
	if target.CurrentCount == 0 {
		return nil, &ServiceError{Code: 30001, Message: "空桶不能封桶"}
	}
	if target.Status == barrel.StatusPacked {
		return toBarrelResponse(target), nil // 幂等
	}

	updated, err := s.client.Barrel.UpdateOneID(bid).
		SetStatus(barrel.StatusPacked).
		SetSyncStatus(barrel.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return toBarrelResponse(updated), nil
}

// GetBarrel 获取桶详情。
func (s *BarrelService) GetBarrel(ctx context.Context, id string) (*BarrelResponse, error) {
	bid, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的桶ID"}
	}

	row, err := s.client.Barrel.Get(ctx, bid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "桶不存在"}
		}
		return nil, err
	}

	return toBarrelResponse(row), nil
}

// ListBarrels 按批次 / 状态分页查询桶，按创建时间倒序。
func (s *BarrelService) ListBarrels(ctx context.Context, page, pageSize int, lotID, status string) ([]*BarrelResponse, int, error) {
	query := s.client.Barrel.Query()

	if lotID != "" {
		id, err := uuid.Parse(lotID)
		if err != nil {
			return nil, 0, &ServiceError{Code: 10001, Message: "无效的批次ID"}
		}
		query = query.Where(barrel.LotIDEQ(id))
	}

	if status != "" {
		query = query.Where(barrel.StatusEQ(barrel.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(barrel.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*BarrelResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toBarrelResponse(r))
	}

	return out, total, nil
}

// generateBarrelNumber 生成桶编号：BRL-<日期>-<随机段>。
func generateBarrelNumber() string {
	return fmt.Sprintf("BRL-%s-%s",
		time.Now().Format("20060102"),
		uuid.NewString()[:8],
	)
}

func toBarrelResponse(b *ent.Barrel) *BarrelResponse {
	resp := &BarrelResponse{
		ID:           b.ID.String(),
		BarrelNumber: b.BarrelNumber,
		LotID:        b.LotID.String(),
		Capacity:     b.Capacity,
		CurrentCount: b.CurrentCount,
		Status:       string(b.Status),
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}

	// filled_at 是可空列：零值即「未装满」，转 JSON null。
	if !b.FilledAt.IsZero() {
		v := b.FilledAt.Format(time.RFC3339)
		resp.FilledAt = &v
	}

	return resp
}
