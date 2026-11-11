package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/pallet"
)

// PalletService 托盘服务。
//
// 托盘由 Edge 产生：包装线在码垛开始时向系统要一个托盘号，码完后封托。
// Center 只读，不提供任何写接口。
type PalletService struct {
	client *ent.Client
}

// NewPalletService 创建托盘服务
func NewPalletService(client *ent.Client) *PalletService {
	return &PalletService{client: client}
}

// CreatePalletRequest 创建托盘请求。
//
// PalletCode 可省略：真实产线由系统生成后交 PLC（托盘号在码垛开始时就确定），
// 这里默认即生成。显式传入仅用于重放或对接既有码垛系统的场景。
type CreatePalletRequest struct {
	LotID        string `json:"lot_id" binding:"required"`
	PalletCode   string `json:"pallet_code"`
	Level        int    `json:"level"`
	PalletizerID string `json:"palletizer_id"`
	// PalletType 区分 POY/FDY 直接码锭（bobbin，默认）与 DTY 码箱（carton）。
	// 一旦创建就不允许修改，PackBobbins/PackCartons 会按此拒绝错误的码法。
	PalletType string `json:"pallet_type"`
}

// PackBobbinsRequest 把丝锭码到托盘上。
type PackBobbinsRequest struct {
	// BobbinIDs 是要码上托盘的丝锭 ID 列表。
	BobbinIDs []string `json:"bobbin_ids" binding:"required,min=1"`
}

// PackCartonsRequest DTY 码箱请求。
type PackCartonsRequest struct {
	// CartonIDs 是已封箱且待码垛的纸箱 ID 列表。
	CartonIDs []string `json:"carton_ids" binding:"required,min=1"`
}

// SealPalletRequest 封托请求。
//
// 三层重量由称重设备给出；净重可省略，省略时按毛重 - 皮重计算。
type SealPalletRequest struct {
	GrossWeight float64  `json:"gross_weight" binding:"required,min=0"`
	TareWeight  float64  `json:"tare_weight" binding:"required,min=0"`
	NetWeight   *float64 `json:"net_weight"`
}

// PalletResponse 托盘响应
type PalletResponse struct {
	ID           string    `json:"id"`
	PalletCode   string    `json:"pallet_code"`
	LotID        string    `json:"lot_id"`
	EdgeID       *string   `json:"edge_id"`
	Level        int       `json:"level"`
	PalletType   string    `json:"pallet_type"`
	BobbinsCount int       `json:"bobbins_count"`
	CartonsCount int       `json:"cartons_count"`
	Status       string    `json:"status"`
	NetWeight    *float64  `json:"net_weight"`
	GrossWeight  *float64  `json:"gross_weight"`
	TareWeight   *float64  `json:"tare_weight"`
	Printed      bool      `json:"printed"`
	PrintedAt    *string   `json:"printed_at"`
	CompletedAt  *string   `json:"completed_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreatePallet 创建托盘，初始状态 building。
func (s *PalletService) CreatePallet(ctx context.Context, req *CreatePalletRequest) (*PalletResponse, error) {
	lotID, err := uuid.Parse(req.LotID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的批次ID"}
	}

	// 批次必须存在：托盘是批次的从属实体，挂空批次没有意义。
	if _, err := s.client.Lot.Get(ctx, lotID); err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "批次不存在"}
		}
		return nil, err
	}

	level := req.Level
	if level == 0 {
		level = 1
	}
	if level < 1 || level > 9 {
		return nil, &ServiceError{Code: 10001, Message: "层级必须在 1-9 之间"}
	}

	code := req.PalletCode
	if code == "" {
		code = generatePalletCode()
	}

	builder := s.client.Pallet.Create().
		SetPalletCode(code).
		SetLotID(lotID).
		SetLevel(level).
		SetBobbinsCount(0).
		SetCartonsCount(0).
		SetStatus(pallet.StatusBuilding)

	palletType := pallet.PalletType(req.PalletType)
	if req.PalletType == "" {
		palletType = pallet.PalletTypeBobbin
	}
	if err := pallet.PalletTypeValidator(palletType); err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的托盘类型：" + req.PalletType}
	}
	builder.SetPalletType(palletType)

	if req.PalletizerID != "" {
		palletizerID, err := uuid.Parse(req.PalletizerID)
		if err != nil {
			return nil, &ServiceError{Code: 10001, Message: "无效的码垛机ID"}
		}
		builder.SetPalletizerID(palletizerID)
	}

	row, err := builder.Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, &ServiceError{Code: 40002, Message: "托盘号已存在：" + code}
		}
		return nil, err
	}

	return toPalletResponse(row), nil
}

// PackBobbins 把丝锭码到托盘上。
//
// 全部丝锭必须与托盘属于同一批次，且不得已经码在别的托盘上 —— 后者会把
// 丝锭从原托盘「偷走」而原托盘的 bobbins_count 不会回退，导致数据不一致。
// 已在本托盘上的丝锭重复提交视为幂等，不报错。
//
// 整个过程在一个事务里：计数与归属要么一起生效，要么都不生效。
func (s *PalletService) PackBobbins(ctx context.Context, palletID string, req *PackBobbinsRequest) (*PalletResponse, error) {
	pid, err := uuid.Parse(palletID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的托盘ID"}
	}

	if len(req.BobbinIDs) == 0 {
		return nil, &ServiceError{Code: 10001, Message: "至少需要一个丝锭"}
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	// 提交成功后 Rollback 会返回 ErrTxDone，属预期，忽略即可。
	defer func() { _ = tx.Rollback() }()

	target, err := tx.Pallet.Get(ctx, pid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "托盘不存在"}
		}
		return nil, err
	}

	if target.Status != pallet.StatusBuilding {
		return nil, &ServiceError{Code: 30001, Message: "只有 building 状态的托盘可以码垛"}
	}
	// 直接码锭的接口只接受 bobbin 类型托盘：DTY 的 carton 托盘要走 PackCartons。
	if target.PalletType != pallet.PalletTypeBobbin {
		return nil, &ServiceError{Code: 30001, Message: "该托盘是 DTY 码箱类型，不能码丝锭"}
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
			return nil, &ServiceError{Code: 30001, Message: "丝锭与托盘不属于同一批次"}
		}
		if b.PalletID != uuid.Nil && b.PalletID != pid {
			return nil, &ServiceError{Code: 30001, Message: "丝锭已码在其它托盘上"}
		}
		if b.PalletID == pid {
			// 已在托盘上：幂等，不重复计数。
			continue
		}

		// 码盘改变了丝锭的业务归属，必须重新同步：Center 需要 pallet_code
		// 才能把丝锭挂到托盘上。已 synced 的丝锭若不动同步状态，采集器
		// 只抓 pending，这条 pallet_id 永远到不了 Center。
		// 同时清零重试计数 —— 否则重试预算已被耗尽的丝锭即使回到 pending
		// 也会被采集器的 `retry_count < 上限` 过滤挡在门外。
		if _, err := tx.Bobbin.UpdateOneID(b.ID).
			SetPalletID(pid).
			SetStatus(bobbin.StatusPacked).
			SetSyncStatus(bobbin.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx); err != nil {
			return nil, err
		}
		added++
	}

	updated, err := tx.Pallet.UpdateOneID(pid).
		SetBobbinsCount(target.BobbinsCount + added).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return toPalletResponse(updated), nil
}

// PackCartons 把已封箱的纸箱码到 DTY 托盘上。
//
// 校验：
//   - 托盘必须是 building 且 pallet_type=carton（与 PackBobbins 互斥）；
//   - 纸箱必须存在、已封箱（packed）、与托盘同批次；
//   - 纸箱不得已码在别的托盘上；
//   - 已在本托盘上的纸箱重复提交是幂等的。
//
// 码箱会改变纸箱的 pallet_id，必须重新排队同步，否则 Center 看不到挂载关系。
// 托盘本身的 cartons_count 与 sync_status 也一并重置。
func (s *PalletService) PackCartons(ctx context.Context, palletID string, req *PackCartonsRequest) (*PalletResponse, error) {
	pid, err := uuid.Parse(palletID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的托盘ID"}
	}
	if len(req.CartonIDs) == 0 {
		return nil, &ServiceError{Code: 10001, Message: "至少需要一个纸箱"}
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	target, err := tx.Pallet.Get(ctx, pid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "托盘不存在"}
		}
		return nil, err
	}
	if target.Status != pallet.StatusBuilding {
		return nil, &ServiceError{Code: 30001, Message: "只有 building 状态的托盘可以码箱"}
	}
	if target.PalletType != pallet.PalletTypeCarton {
		return nil, &ServiceError{Code: 30001, Message: "该托盘是直接码锭类型，不能码纸箱"}
	}

	ids := make([]uuid.UUID, 0, len(req.CartonIDs))
	for _, raw := range req.CartonIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, &ServiceError{Code: 10001, Message: "无效的纸箱ID：" + raw}
		}
		ids = append(ids, id)
	}

	rows, err := tx.Carton.Query().Where(carton.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, &ServiceError{Code: 40001, Message: "存在无效的纸箱ID"}
	}

	added := 0
	for _, c := range rows {
		// 纸箱 lot_id 可空，但码上托盘必须与托盘同批：用 lot_id 非空做前提。
		if c.LotID == uuid.Nil || c.LotID != target.LotID {
			return nil, &ServiceError{Code: 30001, Message: "纸箱与托盘不属于同一批次"}
		}
		if c.Status != carton.StatusPacked {
			return nil, &ServiceError{Code: 30001, Message: "只有已封箱的纸箱可以码垛：" + c.CartonNumber}
		}
		if c.PalletID != uuid.Nil && c.PalletID != pid {
			return nil, &ServiceError{Code: 30001, Message: "纸箱已码在其它托盘上：" + c.CartonNumber}
		}
		if c.PalletID == pid {
			continue // 幂等
		}

		if _, err := tx.Carton.UpdateOneID(c.ID).
			SetPalletID(pid).
			SetSyncStatus(carton.SyncStatusPending).
			SetSyncRetryCount(0).
			ClearSyncedAt().
			Save(ctx); err != nil {
			return nil, err
		}
		added++
	}

	updated, err := tx.Pallet.UpdateOneID(pid).
		SetCartonsCount(target.CartonsCount + added).
		SetSyncStatus(pallet.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return toPalletResponse(updated), nil
}

// SealPallet 封托：写入重量并把状态推进到 completed。
//
// 封托后不可再码垛 —— 重量已经称定，再加丝锭会让账实不符。
func (s *PalletService) SealPallet(ctx context.Context, palletID string, req *SealPalletRequest) (*PalletResponse, error) {
	pid, err := uuid.Parse(palletID)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的托盘ID"}
	}

	target, err := s.client.Pallet.Get(ctx, pid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "托盘不存在"}
		}
		return nil, err
	}

	if target.Status != pallet.StatusBuilding {
		return nil, &ServiceError{Code: 30001, Message: "只有 building 状态的托盘可以封托"}
	}
	if target.BobbinsCount == 0 {
		return nil, &ServiceError{Code: 30001, Message: "空托盘不能封托"}
	}

	net := req.GrossWeight - req.TareWeight
	if req.NetWeight != nil {
		net = *req.NetWeight
	}
	if net < 0 {
		return nil, &ServiceError{Code: 10001, Message: "净重不能为负（毛重须不小于皮重）"}
	}

	// 封托写入重量并推进状态：托盘若已同步过，必须重新排队，
	// 否则 Center 永远停在 building 且三层重量为空。
	updated, err := s.client.Pallet.UpdateOneID(pid).
		SetGrossWeight(req.GrossWeight).
		SetTareWeight(req.TareWeight).
		SetNetWeight(net).
		SetStatus(pallet.StatusCompleted).
		SetCompletedAt(time.Now()).
		SetSyncStatus(pallet.SyncStatusPending).
		SetSyncRetryCount(0).
		ClearSyncedAt().
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return toPalletResponse(updated), nil
}

// GetPallet 获取托盘详情。
func (s *PalletService) GetPallet(ctx context.Context, id string) (*PalletResponse, error) {
	pid, err := uuid.Parse(id)
	if err != nil {
		return nil, &ServiceError{Code: 10001, Message: "无效的托盘ID"}
	}

	row, err := s.client.Pallet.Get(ctx, pid)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, &ServiceError{Code: 40001, Message: "托盘不存在"}
		}
		return nil, err
	}

	return toPalletResponse(row), nil
}

// ListPallets 按批次 / 状态分页查询托盘，按创建时间倒序。
func (s *PalletService) ListPallets(ctx context.Context, page, pageSize int, lotID, status string) ([]*PalletResponse, int, error) {
	query := s.client.Pallet.Query()

	if lotID != "" {
		id, err := uuid.Parse(lotID)
		if err != nil {
			return nil, 0, &ServiceError{Code: 10001, Message: "无效的批次ID"}
		}
		query = query.Where(pallet.LotIDEQ(id))
	}

	if status != "" {
		query = query.Where(pallet.StatusEQ(pallet.Status(status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(pallet.FieldCreatedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*PalletResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPalletResponse(r))
	}

	return out, total, nil
}

// generatePalletCode 生成托盘号：PLT-<日期>-<随机段>。
//
// 产线上托盘号在码垛开始时就由系统生成并交给 PLC，因此这里不依赖外部传入。
func generatePalletCode() string {
	return fmt.Sprintf("PLT-%s-%s",
		time.Now().Format("20060102"),
		uuid.NewString()[:8],
	)
}

func toPalletResponse(p *ent.Pallet) *PalletResponse {
	resp := &PalletResponse{
		ID:           p.ID.String(),
		PalletCode:   p.PalletCode,
		LotID:        p.LotID.String(),
		Level:        p.Level,
		PalletType:   string(p.PalletType),
		BobbinsCount: p.BobbinsCount,
		CartonsCount: p.CartonsCount,
		Status:       string(p.Status),
		Printed:      p.Printed,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}

	// edge_id / 三层重量 / 打印与完成时间在 schema 中都是 Optional()，
	// 生成的 Go 字段是非指针类型，零值即「未设置」。这里转成 JSON null，
	// 避免把「未称重」显示成 0 kg。
	if p.EdgeID != uuid.Nil {
		id := p.EdgeID.String()
		resp.EdgeID = &id
	}
	if p.NetWeight != 0 {
		v := p.NetWeight
		resp.NetWeight = &v
	}
	if p.GrossWeight != 0 {
		v := p.GrossWeight
		resp.GrossWeight = &v
	}
	if p.TareWeight != 0 {
		v := p.TareWeight
		resp.TareWeight = &v
	}
	if !p.PrintedAt.IsZero() {
		v := p.PrintedAt.Format(time.RFC3339)
		resp.PrintedAt = &v
	}
	if !p.CompletedAt.IsZero() {
		v := p.CompletedAt.Format(time.RFC3339)
		resp.CompletedAt = &v
	}

	return resp
}
