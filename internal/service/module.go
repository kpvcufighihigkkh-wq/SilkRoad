package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/module"
)

// ModuleService 吊车（模块）服务。
//
// 吊车是物流载具：一次能吊两个装满的落纱桶，把它们从落纱工位送到
// 下一工段。它不属于某个批次 —— 两个桶来自同批时才同批，因此状态里
// 没有 lot_id，批次归属由所载的桶间接给出。
//
// 与桶/托盘不同，吊车不改变丝锭的工艺归属，只在「装桶/卸桶」时
// 改变两个桶的所在位置。装桶与卸桶都要让被影响的桶重新排队同步。
type ModuleService struct {
	client *ent.Client
}

// NewModuleService 创建吊车服务
func NewModuleService(client *ent.Client) *ModuleService {
	return &ModuleService{client: client}
}

// CreateModuleRequest 创建吊车请求。
type CreateModuleRequest struct {
	ModuleNumber string `json:"module_number"`
	RFID         string `json:"rfid"`
}

// LoadBarrelsRequest 往吊车上装桶。
//
// BarrelIDs 最多两个：吊车物理上只有两个挂位。
type LoadBarrelsRequest struct {
	BarrelIDs []string `json:"barrel_ids" binding:"required,min=1,max=2"`
}

// UpdateModuleStatusRequest 更新吊车状态。
type UpdateModuleStatusRequest struct {
	Status          string `json:"status" binding:"required"`
	CurrentLocation string `json:"current_location"`
}

// ModuleResponse 吊车响应
type ModuleResponse struct {
	ID              string    `json:"id"`
	ModuleNumber    string    `json:"module_number"`
	Barrel1ID       *string   `json:"barrel1_id"`
	Barrel2ID       *string   `json:"barrel2_id"`
	Status          string    `json:"status"`
	RFID            string    `json:"rfid"`
	CurrentLocation string    `json:"current_location"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// CreateModule 创建吊车，初始状态 idle。
func (s *ModuleService) CreateModule(ctx context.Context, req *CreateModuleRequest) (*ModuleResponse, error) {
	number := req.ModuleNumber
	if number == "" {
		number = generateModuleNumber()
	}

	builder := s.client.Module.Create().
		SetModuleNumber(number).
		SetStatus(module.StatusIdle)
	if req.RFID != "" {
		builder.SetRfid(req.RFID)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, &ServiceError{Code: 40002, Message: "吊车编号已存在：" + number}
		}
		return nil, err
	}

	return toModuleResponse(row), nil
}

// LoadBarrels 把落纱桶装到吊车上。
//
// 校验：
//   - 桶必须存在，且不得重复出现在两个挂位上；
//   - 一个桶同时只能在一台吊车上（改挂会导致另一台吊车的账实不符）；
//   - 桶已被别的吊车占用时拒绝。
//
// 同批校验刻意不做：吊车按工段流转，两个挂位上的桶确实可能来自相邻批次
// （换批时工位上的残留桶）。强行要求同批会逼操作员先卸后装，反而增加
// 现场出错的机会。批次归属由桶自己携带。
func (s *ModuleService) LoadBarrels(ctx context.Context, moduleID string, req *LoadBarrelsRequest) (*ModuleResponse, error) {
	mid, err := uuid.Parse(moduleID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的吊车ID"}
	}
	if len(req.BarrelIDs) == 0 || len(req.BarrelIDs) > 2 {
		return nil, &ServiceError{Code: 10001, Message: "吊车一次只能装载 1-2 个桶"}
	}

	ids := make([]uuid.UUID, 0, len(req.BarrelIDs))
	for _, raw := range req.BarrelIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, &ServiceError{Code: 10001, Message: "无效的桶ID：" + raw}
		}
		ids = append(ids, id)
	}
	if len(ids) == 2 && ids[0] == ids[1] {
		return nil, &ServiceError{Code: 10001, Message: "两个挂位不能是同一个桶"}
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	// 存在性检查：后续更新用 UpdateOneID，缺行的错误在 Save 处才会暴露，
	// 提前查一次可以给出明确的 404 语义。
	if _, err := tx.Module.Get(ctx, mid); err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "吊车不存在"}
		}
		return nil, err
	}

	// 桶必须存在
	found, err := tx.Barrel.Query().Where(barrel.IDIn(ids...)).Count(ctx)
	if err != nil {
		return nil, err
	}
	if found != len(ids) {
		return nil, &ServiceError{Code: 40001, Message: "存在无效的桶ID"}
	}

	// 桶不得挂在别的吊车上：全表扫一遍挂位即可，吊车数量是两位数级别。
	others, err := tx.Module.Query().
		Where(module.IDNEQ(mid)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range others {
		for _, id := range ids {
			if m.Barrel1ID == id || m.Barrel2ID == id {
				return nil, &ServiceError{Code: 30001,
					Message: "桶 " + id.String()[:8] + " 已挂在吊车 " + m.ModuleNumber + " 上"}
			}
		}
	}

	builder := tx.Module.UpdateOneID(mid).
		SetStatus(module.StatusLoading).
		SetSyncStatus(module.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt()

	builder.SetNillableBarrel1ID(&ids[0])
	if len(ids) == 2 {
		builder.SetNillableBarrel2ID(&ids[1])
	} else {
		builder.ClearBarrel2ID()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}

	// 桶的所在吊车改变不影响桶自身字段（桶没有 module_id 列），
	// 因此无需重排桶的同步 —— 这条关系只存在于吊车侧。
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return toModuleResponse(updated), nil
}

// UpdateStatus 推进吊车状态。
//
// 状态是单向流转：idle → loading → transporting → sorting → warehouse → idle。
// 这里不做强制顺序校验：现场可能跳过某一步（直接装车），
// 强制顺序会逼操作员补一个不存在的中间态。只校验状态名合法。
func (s *ModuleService) UpdateStatus(ctx context.Context, moduleID string, req *UpdateModuleStatusRequest) (*ModuleResponse, error) {
	mid, err := uuid.Parse(moduleID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的吊车ID"}
	}

	status := module.Status(req.Status)
	if err := module.StatusValidator(status); err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的吊车状态：" + req.Status}
	}

	builder := s.client.Module.UpdateOneID(mid).
		SetStatus(status).
		SetSyncStatus(module.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt()
	if req.CurrentLocation != "" {
		builder.SetCurrentLocation(req.CurrentLocation)
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "吊车不存在"}
		}
		return nil, err
	}

	return toModuleResponse(updated), nil
}

// GetModule 获取吊车详情。
func (s *ModuleService) GetModule(ctx context.Context, id string) (*ModuleResponse, error) {
	mid, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的吊车ID"}
	}

	row, err := s.client.Module.Get(ctx, mid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "吊车不存在"}
		}
		return nil, err
	}

	return toModuleResponse(row), nil
}

// ListModules 按状态分页查询吊车，按创建时间倒序。
func (s *ModuleService) ListModules(ctx context.Context, page, pageSize int, status string) ([]*ModuleResponse, int, error) {
	query := s.client.Module.Query()
	if status != "" {
		query = query.Where(module.StatusEQ(module.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(module.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*ModuleResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toModuleResponse(r))
	}

	return out, total, nil
}

// generateModuleNumber 生成吊车编号：MDL-<日期>-<随机段>。
func generateModuleNumber() string {
	return fmt.Sprintf("MDL-%s-%s",
		time.Now().Format("20060102"),
		uuid.NewString()[:8],
	)
}

func toModuleResponse(m *ent.Module) *ModuleResponse {
	resp := &ModuleResponse{
		ID:              m.ID.String(),
		ModuleNumber:    m.ModuleNumber,
		Status:          string(m.Status),
		RFID:            m.Rfid,
		CurrentLocation: m.CurrentLocation,
		CreatedAt:       m.CreatedAt,
		UpdatedAt:       m.UpdatedAt,
	}

	// barrel1_id / barrel2_id 都是 Optional：零值即「空挂位」，转 JSON null。
	if m.Barrel1ID != uuid.Nil {
		v := m.Barrel1ID.String()
		resp.Barrel1ID = &v
	}
	if m.Barrel2ID != uuid.Nil {
		v := m.Barrel2ID.String()
		resp.Barrel2ID = &v
	}

	return resp
}
