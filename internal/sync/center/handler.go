package center

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/barrel"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/carton"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/module"
	"github.com/yourusername/igh-silkroad/internal/database/ent/pallet"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// entityDependencyOrder 定义上传实体的依赖顺序。
//
// 外键要求被引用者先到：barrel 引用 lot，bobbin 引用 lot 与 barrel，
// 并可能引用 pallet（按 pallet_code 解引用，见 resolveParentPalletID）。
// 乱序到达会使 Ent 的 FK 校验失败，导致整批记录被拒。
var entityDependencyOrder = map[string]int{
	"lots":     0,
	"doffings": 1,
	"barrels":  2,
	"pallets":  3,
	"cartons":  4,
	"modules":  5,
	"bobbins":  6,
}

// SortEntriesByDependency 按外键依赖顺序稳定排序上传条目。
//
// 未知表排在已知表之后，并保持其原有相对顺序（sort.SliceStable）。
func SortEntriesByDependency(entries []models.UploadEntry) []models.UploadEntry {
	if len(entries) < 2 {
		return entries
	}

	sorted := make([]models.UploadEntry, len(entries))
	copy(sorted, entries)

	rank := func(table string) int {
		if r, ok := entityDependencyOrder[table]; ok {
			return r
		}
		return len(entityDependencyOrder) // 未知表最后
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		return rank(sorted[i].Table) < rank(sorted[j].Table)
	})

	return sorted
}

// UploadHandler 处理边端上传的数据
type UploadHandler struct {
	client *ent.Client
}

// NewUploadHandler 创建上传处理器
func NewUploadHandler(client *ent.Client) *UploadHandler {
	return &UploadHandler{
		client: client,
	}
}

// HandleUpload 处理上传请求
func (h *UploadHandler) HandleUpload(ctx context.Context, req *models.UploadRequest) (*models.UploadResponse, error) {
	log.Printf("📥 Receiving upload from edge_id=%s, entries=%d", req.EdgeID, len(req.Entries))

	// 权威身份由 Center 的鉴权层推导并写入 req.EdgeID（必须是设备行的 UUID，
	// 而非 edge_code）。解析失败或解析出全零 UUID 都说明调用链上游出错了 ——
	// 此前这里退化为 uuid.Nil 并继续，结果是记录被写入且 edge_id 为 NULL，
	// 返回 applied: N，与「身份被伪造」在观测上完全一致。
	// 宁可整批拒绝，也不留下无法归属的数据。
	//
	// 必须同时排除 uuid.Nil：uuid.Parse("00000000-0000-0000-0000-000000000000")
	// 返回的是 err == nil，仅靠上面的错误检查拦不住它，它随后会命中
	// applyEntry → upsertLot 里的 `if edgeID != uuid.Nil`，跳过 SetEdgeID，
	// 同样得到 edge_id = NULL。所以「能解析」不等于「是有效身份」。
	authoritativeEdgeID, err := uuid.Parse(req.EdgeID)
	if err != nil {
		return nil, fmt.Errorf("invalid authoritative edge id %q: %w", req.EdgeID, err)
	}
	if authoritativeEdgeID == uuid.Nil {
		return nil, fmt.Errorf("invalid authoritative edge id %q: 不得为空 UUID", req.EdgeID)
	}

	applied := 0
	rejected := 0
	var errors []string

	// 按外键依赖顺序处理，避免被引用记录尚未到达
	entries := SortEntriesByDependency(req.Entries)

	for _, entry := range entries {
		if err := h.applyEntry(ctx, &entry, authoritativeEdgeID); err != nil {
			rejected++
			errors = append(errors, fmt.Sprintf("%s/%s: %v", entry.Table, entry.ID, err))
			log.Printf("❌ Failed to apply %s/%s: %v", entry.Table, entry.ID, err)
		} else {
			applied++
		}
	}

	log.Printf("✅ Upload processed: applied=%d, rejected=%d", applied, rejected)

	return &models.UploadResponse{
		Applied:  applied,
		Rejected: rejected,
		Errors:   errors,
	}, nil
}

// applyEntry 应用单条记录
func (h *UploadHandler) applyEntry(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	switch entry.Operation {
	case "create":
		return h.handleCreate(ctx, entry, edgeID)
	case "update":
		return h.handleUpdate(ctx, entry)
	case "delete":
		return h.handleDelete(ctx, entry)
	default:
		return fmt.Errorf("unknown operation: %s", entry.Operation)
	}
}

// handleCreate 处理创建操作。
//
// 两条路径都是 upsert（存在则更新、不存在则插入），非字面意义的插入：
// Edge 永远发 create，重传/重试/网络重发天然安全。
func (h *UploadHandler) handleCreate(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	switch entry.Table {
	case "lots":
		return h.upsertLot(ctx, entry, edgeID)
	case "pallets":
		return h.upsertPallet(ctx, entry, edgeID)
	case "barrels":
		return h.upsertBarrel(ctx, entry)
	case "cartons":
		return h.upsertCarton(ctx, entry)
	case "modules":
		return h.upsertModule(ctx, entry)
	case "bobbins":
		return h.upsertBobbin(ctx, entry, edgeID)
	case "doffings":
		return h.upsertDoffing(ctx, entry)
	case "bobbin_grades":
		return h.upsertBobbinGrade(ctx, entry)
	default:
		return fmt.Errorf("unknown table: %s", entry.Table)
	}
}

// upsertLot 插入或更新批次记录。
//
// 幂等与认领：先按 ID 查；未命中再按 lot_number 查（该字段唯一）。
// 编号命中说明 Center 侧已存在同一业务批次但 UUID 不同（历史数据或人工
// 创建），此时认领该行并更新字段，**保留 Center 行的 ID** —— 这样 Center
// 侧既有的 lot_id 引用（落纱/丝饼）继续有效。
//
// 为何不能只按 ID 判断：Edge 与 Center 各自用 uuid.New 生成主键，
// 同一业务批次在两端的 ID 不同，只按 ID 查必然放行，随后撞唯一约束。
//
// edgeID 为鉴权层推导的权威设备身份（uuid.Nil 表示未推导出）；
// 载荷中的 edge_id 一律不采信，避免伪造来源设备。
func (h *UploadHandler) upsertLot(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotNumber := getString(data, "lot_number")
	if lotNumber == "" {
		// lot_number 是 NotEmpty + Unique；按编号认领也依赖它，缺失即无法处理。
		return fmt.Errorf("missing lot_number for entry %s（按编号认领需要它）", id)
	}

	target, err := h.resolveLot(ctx, id, lotNumber)
	if err != nil {
		return err
	}

	// 未命中任何既有行：按 Edge 的 ID 新建
	if target == nil {
		builder := h.client.Lot.Create().
			SetID(id).
			SetLotNumber(lotNumber).
			SetProductType(lot.ProductType(getString(data, "product_type"))).
			SetPlannedQuantity(getInt(data, "planned_quantity")).
			SetActualQuantity(getInt(data, "actual_quantity")).
			SetStatus(lot.Status(getString(data, "status"))).
			SetNillableStartTime(getTimePtr(data, "start_time")).
			SetNillableEndTime(getTimePtr(data, "end_time"))

		// 可选字段三件套（plc_lot_number/order_code/product_spec）在 Create 侧
		// 用 Nillable 版本 —— 与 updateLotRow 中的非 Nillable 版本是同一组字段，
		// 改名/增删需同步另一处（两个 builder 类型不同，无法共用设置函数）。
		builder.
			SetNillablePlcLotNumber(getStringPtr(data, "plc_lot_number")).
			SetNillableOrderCode(getStringPtr(data, "order_code")).
			SetNillableProductSpec(getStringPtr(data, "product_spec"))

		// 权威设备身份：uuid.Nil 表示上游未推导出身份，此时不写外键
		// （保留原有语义）。HandleUpload 已拒绝无法解析与全零的 EdgeID，
		// 故正常路径下 edgeID 恒非 Nil。
		if edgeID != uuid.Nil {
			builder.SetEdgeID(edgeID)
		}

		created, err := builder.Save(ctx)
		if err != nil {
			// 并发（两条同编号记录同时到达）或历史数据导致唯一约束冲突：
			// 重查一次并按编号认领转更新。查不到则说明冲突不是当前批次所致，
			// 保持原错误，交由上层计重试。
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveLot(ctx, id, lotNumber); rerr == nil && again != nil {
					return h.updateLotRow(ctx, again, data, edgeID)
				}
			}
			return fmt.Errorf("create lot: %w", err)
		}
		log.Printf("✅ Created lot: %s", created.LotNumber)
		return nil
	}

	return h.updateLotRow(ctx, target, data, edgeID)
}

// resolveLot 按 ID 再按业务编号定位既有行；都不存在返回 (nil, nil)。
//
// 先 ID 后编号的顺序不可交换：ID 命中说明这就是 Edge 那条记录（正常重传），
// 优先认它是为了在「同 ID 行被改过编号」时仍能正确定位。
func (h *UploadHandler) resolvePallet(ctx context.Context, id uuid.UUID, code string) (*ent.Pallet, error) {
	byID, err := h.client.Pallet.Query().Where(pallet.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byCode, err := h.client.Pallet.Query().Where(pallet.PalletCodeEQ(code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byCode, nil
}

// upsertPallet 插入或更新托盘记录。
//
// 与批次同理：Edge 与 Center 各自生成主键，两端的同一业务托盘 ID 不同。
// 先按 ID 查，未命中再按 pallet_code 认领（该字段唯一），认领时保留
// Center 行的 ID —— 丝锭的 pallet_id 外键指向它，换 ID 会断开既有引用。
func (h *UploadHandler) upsertPallet(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	code := getString(data, "pallet_code")
	if code == "" {
		// pallet_code 是 NotEmpty + Unique，按编号认领也依赖它。
		return fmt.Errorf("missing pallet_code for entry %s（按编号认领需要它）", id)
	}

	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}

	target, err := h.resolvePallet(ctx, id, code)
	if err != nil {
		return err
	}

	if target == nil {
		builder := h.client.Pallet.Create().
			SetID(id).
			SetPalletCode(code).
			SetLotID(lotID).
			SetLevel(getInt(data, "level")).
			SetPalletType(palletTypeFromPayload(data)).
			SetBobbinsCount(getInt(data, "bobbins_count")).
			SetCartonsCount(getInt(data, "cartons_count")).
			SetStatus(pallet.Status(getString(data, "status"))).
			SetPrinted(getBool(data, "printed")).
			SetNillablePalletizerID(getUUIDPtr(data, "palletizer_id")).
			SetNillableNetWeight(getFloatPtr(data, "net_weight")).
			SetNillableGrossWeight(getFloatPtr(data, "gross_weight")).
			SetNillableTareWeight(getFloatPtr(data, "tare_weight")).
			SetNillablePrintedAt(getTimePtr(data, "printed_at")).
			SetNillableCompletedAt(getTimePtr(data, "completed_at"))

		// 来源设备以鉴权层推导的身份为准，不采信载荷：
		// 与批次一致，避免伪造 edge_id。
		if edgeID != uuid.Nil {
			builder.SetEdgeID(edgeID)
		}

		created, err := builder.Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolvePallet(ctx, id, code); rerr == nil && again != nil {
					return h.updatePalletRow(ctx, again, data)
				}
			}
			return fmt.Errorf("create pallet: %w", err)
		}
		log.Printf("✅ Created pallet: %s", created.PalletCode)
		return nil
	}

	return h.updatePalletRow(ctx, target, data)
}

// updatePalletRow 以 Edge 数据更新既有托盘（Edge 是权威源）。
// 保留 Center 行的 ID；pallet_code 是认领依据，不重写。
func (h *UploadHandler) updatePalletRow(ctx context.Context, row *ent.Pallet, data map[string]interface{}) error {
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}

	err = row.Update().
		SetLotID(lotID).
		SetLevel(getInt(data, "level")).
		SetPalletType(palletTypeFromPayload(data)).
		SetBobbinsCount(getInt(data, "bobbins_count")).
		SetCartonsCount(getInt(data, "cartons_count")).
		SetStatus(pallet.Status(getString(data, "status"))).
		SetPrinted(getBool(data, "printed")).
		SetNillablePalletizerID(getUUIDPtr(data, "palletizer_id")).
		SetNillableNetWeight(getFloatPtr(data, "net_weight")).
		SetNillableGrossWeight(getFloatPtr(data, "gross_weight")).
		SetNillableTareWeight(getFloatPtr(data, "tare_weight")).
		SetNillablePrintedAt(getTimePtr(data, "printed_at")).
		SetNillableCompletedAt(getTimePtr(data, "completed_at")).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update pallet: %w", err)
	}
	log.Printf("🔁 Updated pallet: %s", row.PalletCode)
	return nil
}

// resolveBarrel 按 ID 再按桶编号定位既有行；都不存在返回 (nil, nil)。
func (h *UploadHandler) resolveBarrel(ctx context.Context, id uuid.UUID, number string) (*ent.Barrel, error) {
	byID, err := h.client.Barrel.Query().Where(barrel.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Barrel.Query().Where(barrel.BarrelNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// upsertBarrel 插入或更新落纱桶。
//
// 与托盘同理：先按 ID 查，未命中再按 barrel_number 认领，认领时保留
// Center 行的 ID —— 丝锭的 barrel_id 外键指向它。
func (h *UploadHandler) upsertBarrel(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	number := getString(data, "barrel_number")
	if number == "" {
		return fmt.Errorf("missing barrel_number for entry %s（按编号认领需要它）", id)
	}

	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}

	target, err := h.resolveBarrel(ctx, id, number)
	if err != nil {
		return err
	}

	if target == nil {
		_, err = h.client.Barrel.Create().
			SetID(id).
			SetBarrelNumber(number).
			SetLotID(lotID).
			SetCapacity(getInt(data, "capacity")).
			SetCurrentCount(getInt(data, "current_count")).
			SetStatus(barrel.Status(getString(data, "status"))).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveBarrel(ctx, id, number); rerr == nil && again != nil {
					return h.updateBarrelRow(ctx, again, data)
				}
			}
			return fmt.Errorf("create barrel: %w", err)
		}
		log.Printf("✅ Created barrel: %s", number)
		return nil
	}

	return h.updateBarrelRow(ctx, target, data)
}

// updateBarrelRow 以 Edge 数据更新既有桶。保留 Center 行 ID；编号不重写。
func (h *UploadHandler) updateBarrelRow(ctx context.Context, row *ent.Barrel, data map[string]interface{}) error {
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}

	err = row.Update().
		SetLotID(lotID).
		SetCapacity(getInt(data, "capacity")).
		SetCurrentCount(getInt(data, "current_count")).
		SetStatus(barrel.Status(getString(data, "status"))).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update barrel: %w", err)
	}
	log.Printf("🔁 Updated barrel: %s", row.BarrelNumber)
	return nil
}

// resolveParentBarrelID 按载荷的 barrel_number 查 Center 侧的桶行。
//
// 与托盘同一契约：丝锭未装桶时载荷不带 barrel_number，返回 (nil, nil)；
// 带了却查不到说明桶尚未到达，必须报错计重试 —— 静默当成「未装桶」
// 会把已装载的丝锭记成散锭。
func (h *UploadHandler) resolveParentBarrelID(ctx context.Context, data map[string]interface{}) (*uuid.UUID, error) {
	number := getString(data, "barrel_number")
	if number == "" {
		return nil, nil
	}
	parent, err := h.client.Barrel.Query().Where(barrel.BarrelNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("parent barrel %q not found: %w", number, err)
		}
		return nil, err
	}
	return &parent.ID, nil
}

// resolveCarton 按 ID 再按纸箱编号定位既有行；都不存在返回 (nil, nil)。
func (h *UploadHandler) resolveCarton(ctx context.Context, id uuid.UUID, number string) (*ent.Carton, error) {
	byID, err := h.client.Carton.Query().Where(carton.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Carton.Query().Where(carton.CartonNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// upsertCarton 插入或更新纸箱（DTY 专属环节）。
func (h *UploadHandler) upsertCarton(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	number := getString(data, "carton_number")
	if number == "" {
		return fmt.Errorf("missing carton_number for entry %s（按编号认领需要它）", id)
	}

	target, err := h.resolveCarton(ctx, id, number)
	if err != nil {
		return err
	}

	lotID, err := h.optionalLotID(ctx, data)
	if err != nil {
		return err
	}
	palletID, err := h.resolvePalletByCode(ctx, data)
	if err != nil {
		return err
	}

	if target == nil {
		created, err := h.client.Carton.Create().
			SetID(id).
			SetCartonNumber(number).
			SetBobbinCount(getInt(data, "bobbin_count")).
			SetCapacity(getInt(data, "capacity")).
			SetStatus(carton.Status(getString(data, "status"))).
			SetLabelPrinted(getBool(data, "label_printed")).
			SetNillableLotID(lotID).
			SetNillablePalletID(palletID).
			SetNillableTotalWeight(getFloatPtr(data, "total_weight")).
			SetNillablePrintedAt(getTimePtr(data, "printed_at")).
			SetNillablePackedAt(getTimePtr(data, "packed_at")).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveCarton(ctx, id, number); rerr == nil && again != nil {
					return h.updateCartonRow(ctx, again, data)
				}
			}
			return fmt.Errorf("create carton: %w", err)
		}
		log.Printf("✅ Created carton: %s", created.CartonNumber)
		return nil
	}

	return h.updateCartonRow(ctx, target, data)
}

// updateCartonRow 以 Edge 数据更新既有纸箱。保留 Center 行 ID；编号不重写。
func (h *UploadHandler) updateCartonRow(ctx context.Context, row *ent.Carton, data map[string]interface{}) error {
	lotID, err := h.optionalLotID(ctx, data)
	if err != nil {
		return err
	}
	palletID, err := h.resolvePalletByCode(ctx, data)
	if err != nil {
		return err
	}

	err = row.Update().
		SetBobbinCount(getInt(data, "bobbin_count")).
		SetCapacity(getInt(data, "capacity")).
		SetStatus(carton.Status(getString(data, "status"))).
		SetLabelPrinted(getBool(data, "label_printed")).
		SetNillableLotID(lotID).
		SetNillablePalletID(palletID).
		SetNillableTotalWeight(getFloatPtr(data, "total_weight")).
		SetNillablePrintedAt(getTimePtr(data, "printed_at")).
		SetNillablePackedAt(getTimePtr(data, "packed_at")).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update carton: %w", err)
	}
	log.Printf("🔁 Updated carton: %s", row.CartonNumber)
	return nil
}

// optionalLotID 解引用可选父批次：载荷未带 lot_number 时返回 nil（写 NULL）。
//
// 纸箱的 lot_id 在 schema 里是可空的，与桶/丝锭的必填外键不同 ——
// 用 resolveParentLotID 会把「没有批次」当成错误。
func (h *UploadHandler) optionalLotID(ctx context.Context, data map[string]interface{}) (*uuid.UUID, error) {
	if getString(data, "lot_number") == "" {
		return nil, nil
	}
	resolved, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// palletTypeFromPayload 读托盘类型；载荷缺该键时回落到 bobbin。
//
// 这是向前兼容：pallet_type 是后加的字段，升级前的 Edge 不会发送它，
// 而那些托盘按当时的口径全部是直接码锭。缺键即 bobbin 才是正确历史语义，
// 绝不能让它落成空串 —— 那会让整条托盘记录被 Ent 的枚举校验拒绝。
func palletTypeFromPayload(data map[string]interface{}) pallet.PalletType {
	raw := getString(data, "pallet_type")
	if raw == "" {
		return pallet.PalletTypeBobbin
	}
	return pallet.PalletType(raw)
}

// resolvePalletByCode 按 pallet_code 查 Center 侧托盘，供纸箱解引用。
//
// 纸箱的 pallet_id 可空：载荷未带 pallet_code 时返回 nil（写 NULL），
// 带了却查不到说明托盘尚未到达，返回错误计重试。
func (h *UploadHandler) resolvePalletByCode(ctx context.Context, data map[string]interface{}) (*uuid.UUID, error) {
	code := getString(data, "pallet_code")
	if code == "" {
		return nil, nil
	}
	parent, err := h.client.Pallet.Query().Where(pallet.PalletCodeEQ(code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("parent pallet %q not found: %w", code, err)
		}
		return nil, err
	}
	return &parent.ID, nil
}

// resolveCartonByNumber 按编号查 Center 侧纸箱，供 bobbin 解引用。
func (h *UploadHandler) resolveCartonByNumber(ctx context.Context, number string) (*uuid.UUID, error) {
	if number == "" {
		return nil, nil
	}
	parent, err := h.client.Carton.Query().Where(carton.CartonNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("parent carton %q not found: %w", number, err)
		}
		return nil, err
	}
	return &parent.ID, nil
}

// resolveBarrelByNumber 按编号查 Center 侧桶，供吊车解引用。
func (h *UploadHandler) resolveBarrelByNumber(ctx context.Context, number string) (*uuid.UUID, error) {
	if number == "" {
		return nil, nil
	}
	parent, err := h.client.Barrel.Query().Where(barrel.BarrelNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("parent barrel %q not found: %w", number, err)
		}
		return nil, err
	}
	return &parent.ID, nil
}

// resolveModule 按 ID 再按吊车编号定位既有行。
func (h *UploadHandler) resolveModule(ctx context.Context, id uuid.UUID, number string) (*ent.Module, error) {
	byID, err := h.client.Module.Query().Where(module.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Module.Query().Where(module.ModuleNumberEQ(number)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// upsertModule 插入或更新吊车。
//
// 两个挂位按 barrel1_number / barrel2_number 解引用；缺编号即视为空挂位。
// 挂位关系不影响丝锭的工艺归属，不值得为它卡住整条记录。
func (h *UploadHandler) upsertModule(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	number := getString(data, "module_number")
	if number == "" {
		return fmt.Errorf("missing module_number for entry %s", id)
	}

	barrel1, err := h.resolveBarrelByNumber(ctx, getString(data, "barrel1_number"))
	if err != nil {
		return err
	}
	barrel2, err := h.resolveBarrelByNumber(ctx, getString(data, "barrel2_number"))
	if err != nil {
		return err
	}

	target, err := h.resolveModule(ctx, id, number)
	if err != nil {
		return err
	}

	if target == nil {
		_, err = h.client.Module.Create().
			SetID(id).
			SetModuleNumber(number).
			SetStatus(module.Status(getString(data, "status"))).
			SetNillableRfid(getStringPtr(data, "rfid")).
			SetNillableCurrentLocation(getStringPtr(data, "current_location")).
			SetNillableBarrel1ID(barrel1).
			SetNillableBarrel2ID(barrel2).
			Save(ctx)
		if err != nil {
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveModule(ctx, id, number); rerr == nil && again != nil {
					return h.updateModuleRow(ctx, again, data, barrel1, barrel2)
				}
			}
			return fmt.Errorf("create module: %w", err)
		}
		log.Printf("✅ Created module: %s", number)
		return nil
	}

	return h.updateModuleRow(ctx, target, data, barrel1, barrel2)
}

// updateModuleRow 以 Edge 数据更新既有吊车。
func (h *UploadHandler) updateModuleRow(ctx context.Context, row *ent.Module, data map[string]interface{}, barrel1, barrel2 *uuid.UUID) error {
	err := row.Update().
		SetStatus(module.Status(getString(data, "status"))).
		SetNillableRfid(getStringPtr(data, "rfid")).
		SetNillableCurrentLocation(getStringPtr(data, "current_location")).
		SetNillableBarrel1ID(barrel1).
		SetNillableBarrel2ID(barrel2).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update module: %w", err)
	}
	log.Printf("🔁 Updated module: %s", row.ModuleNumber)
	return nil
}

func (h *UploadHandler) resolveLot(ctx context.Context, id uuid.UUID, lotNumber string) (*ent.Lot, error) {
	byID, err := h.client.Lot.Query().Where(lot.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Lot.Query().Where(lot.LotNumberEQ(lotNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// resolveParentLotID 按载荷的 lot_number 查 Center 侧的批次行，返回其 ID。
//
// 为何不采信载荷里的 lot_id：那是 Edge 本地生成的 UUID。Center 可能已
// 存在同一业务编号、但主键不同的行（历史数据或人工创建），此时若直接
// 用载荷的 ID 写外键，要么撞外键约束失败，要么挂到错误的行上。
// 与整个同步链路的原则一致：身份由 Center 推导，载荷只提供业务键。
func (h *UploadHandler) resolveParentLotID(ctx context.Context, data map[string]interface{}) (uuid.UUID, error) {
	lotNumber := getString(data, "lot_number")
	if lotNumber == "" {
		return uuid.Nil, fmt.Errorf("missing lot_number：无法定位父批次")
	}
	parent, err := h.client.Lot.Query().Where(lot.LotNumberEQ(lotNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// 父行尚未到达：让本条目计重试，等批次上传后再挂载。
			// 不写 NULL 外键（lot_id 是 Required），也不静默跳过。
			return uuid.Nil, fmt.Errorf("parent lot %q not found: %w", lotNumber, err)
		}
		return uuid.Nil, err
	}
	return parent.ID, nil
}

// resolveParentPalletID 按载荷的 pallet_code 查 Center 侧的托盘行，返回其 ID。
//
// 与 resolveParentLotID 同一原则：载荷里的 pallet_id 是 Edge 本地 UUID，
// 两端主键不保证一致，只能按业务编号解引用。
//
// 与批次不同，托盘是**可选**父行：丝锭尚未码盘时载荷里根本没有 pallet_code，
// 这不是错误，返回 (nil, nil) 表示「无托盘」。但一旦带了 pallet_code 却查不到，
// 就必须报错计重试 —— 静默当成「无托盘」会把已包装的丝锭记成散锭。
func (h *UploadHandler) resolveParentPalletID(ctx context.Context, data map[string]interface{}) (*uuid.UUID, error) {
	code := getString(data, "pallet_code")
	if code == "" {
		return nil, nil
	}
	parent, err := h.client.Pallet.Query().Where(pallet.PalletCodeEQ(code)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// 托盘尚未到达：计重试。collectors() 中 pallets 排在 bobbins 之前，
			// 正常情况下不会走到这里；走到说明托盘上传失败或被跳过。
			return nil, fmt.Errorf("parent pallet %q not found: %w", code, err)
		}
		return nil, err
	}
	return &parent.ID, nil
}

// updateLotRow 以 Edge 侧数据更新既有行。整行以 Edge 为准（单向归属）。
//
// 认领时保留 target 的 ID —— Center 侧 lot_id 引用随之继续有效。
//
// edge_id 是「来源设备」（哪台设备生产了这条记录），属历史事实，**不可覆盖**：
// 认领的行若已有来源，说明它本属另一台设备（cmd/seed 或历史人工创建），
// 而 edge_id 被 LotService.ListLots 与 Center 批次列表的 ?edge_id= 过滤条件
// 直接使用，覆盖会让运维把 A 设备的记录误判为 B 设备生产。故仅在原本为空时
// 补上权威身份（历史/人工创建的行可能没有来源设备）。
func (h *UploadHandler) updateLotRow(ctx context.Context, row *ent.Lot, data map[string]interface{}, edgeID uuid.UUID) error {
	builder := row.Update().
		SetProductType(lot.ProductType(getString(data, "product_type"))).
		SetPlannedQuantity(getInt(data, "planned_quantity")).
		SetActualQuantity(getInt(data, "actual_quantity")).
		SetStatus(lot.Status(getString(data, "status"))).
		SetNillableStartTime(getTimePtr(data, "start_time")).
		SetNillableEndTime(getTimePtr(data, "end_time")).
		// 可选字段三件套，Update 侧用非 Nillable 版本（空串即清空），
		// 与 upsertLot 中的 Nillable 版本是同一组字段，改名/增删需同步另一处。
		SetPlcLotNumber(getString(data, "plc_lot_number")).
		SetOrderCode(getString(data, "order_code")).
		SetProductSpec(getString(data, "product_spec"))

	// 仅在既有行没有来源设备时补写；已有来源则保留（见函数注释）。
	if row.EdgeID == uuid.Nil && edgeID != uuid.Nil {
		builder.SetEdgeID(edgeID)
	}

	if err := builder.Exec(ctx); err != nil {
		return fmt.Errorf("update lot: %w", err)
	}
	log.Printf("🔁 Updated lot: %s", row.LotNumber)
	return nil
}

// upsertBobbin 插入或更新丝锭记录（与 upsertLot 同构）。
//
// 幂等与认领：先按 ID 查；未命中再按 bobbin_number 查（该字段唯一）。
// 编号命中说明 Center 侧已存在同一业务丝锭但 UUID 不同，认领该行并更新，
// **保留 Center 行的 ID**（丝饼的 lot_id 引用随之继续有效）。
//
// edgeID 为鉴权层推导的权威设备身份。bobbins 表本身没有 edge_id 列
// （来源设备经 lot 外键传递），此参数仅为与 upsertLot 保持一致的调用
// 链签名而保留，当前未使用。
func (h *UploadHandler) upsertBobbin(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	// 载荷中的 lot_id 是 Edge 本地 UUID，一律不采信；按 lot_number 解引用父行。
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}
	// 托盘同理按 pallet_code 解引用；未码盘的丝锭得到 nil。
	palletID, err := h.resolveParentPalletID(ctx, data)
	if err != nil {
		return err
	}
	// 纸箱与桶同理按业务编号解引用；未装箱/未装桶时得到 nil。
	cartonID, err := h.resolveCartonByNumber(ctx, getString(data, "carton_number"))
	if err != nil {
		return err
	}
	barrelID, err := h.resolveParentBarrelID(ctx, data)
	if err != nil {
		return err
	}
	bobbinNumber := getString(data, "bobbin_number")
	if bobbinNumber == "" {
		// bobbin_number 是 NotEmpty + Unique；按编号认领也依赖它，缺失即无法处理。
		return fmt.Errorf("missing bobbin_number for entry %s（按编号认领需要它）", id)
	}

	target, err := h.resolveBobbin(ctx, id, bobbinNumber)
	if err != nil {
		return err
	}

	// 未命中任何既有行：按 Edge 的 ID 新建
	if target == nil {
		_, err = h.client.Bobbin.Create().
			SetID(id).
			SetBobbinNumber(bobbinNumber).
			SetLotID(lotID).
			SetSpinningPosition(getInt(data, "spinning_position")).
			SetGrossWeight(getFloat(data, "gross_weight")).
			SetNetWeight(getFloat(data, "net_weight")).
			SetNillablePalletID(palletID).
			SetNillableCartonID(cartonID).
			SetNillableBarrelID(barrelID).
			SetNillableTareWeight(getFloatPtr(data, "tare_weight")).
			SetNillableGrade(getStringPtr(data, "grade")).
			SetStatus(bobbin.Status(getString(data, "status"))).
			SetLabelPrinted(getBool(data, "label_printed")).
			SetNillablePrintedAt(getTimePtr(data, "printed_at")).
			SetNillableCompletedAt(getTimePtr(data, "completed_at")).
			Save(ctx)
		if err != nil {
			// 并发或历史数据导致唯一约束冲突：重查一次并按编号认领转更新。
			if ent.IsConstraintError(err) {
				if again, rerr := h.resolveBobbin(ctx, id, bobbinNumber); rerr == nil && again != nil {
					return h.updateBobbinRow(ctx, again, data)
				}
			}
			return fmt.Errorf("create bobbin: %w", err)
		}
		log.Printf("✅ Created bobbin: %s", bobbinNumber)
		return nil
	}

	return h.updateBobbinRow(ctx, target, data)
}

// resolveBobbin 按 ID 再按业务编号定位既有行；都不存在返回 (nil, nil)。
// 顺序与理由同 resolveLot：先 ID 后编号。
func (h *UploadHandler) resolveBobbin(ctx context.Context, id uuid.UUID, bobbinNumber string) (*ent.Bobbin, error) {
	byID, err := h.client.Bobbin.Query().Where(bobbin.IDEQ(id)).Only(ctx)
	if err == nil {
		return byID, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	byNumber, err := h.client.Bobbin.Query().Where(bobbin.BobbinNumberEQ(bobbinNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return byNumber, nil
}

// updateBobbinRow 以 Edge 侧数据更新既有行（整行以 Edge 为准）。
// 保留 row 的 ID；bobbin_number 是认领依据，不重写。
//
// grade/tare_weight 用 Nillable 版本：载荷未带该键时传 nil，Setter 内的
// `if v != nil` 使其成为 no-op，从而**保留 Center 既有的值**。这是刻意的
// 取舍（Edge 侧有意抹除该字段不会传播），故此处不加 Clear*。
//
// pallet_id 是例外：丝锭从散锭码上托盘时，载荷才首次带上 pallet_code；
// 而 Edge 不会把丝锭从托盘上摘下（没有拆盘接口），所以「本来有托盘、新载荷
// 没有」只可能是同一托盘尚未重传，用 Nillable 保留既有关联才是对的。
func (h *UploadHandler) updateBobbinRow(ctx context.Context, row *ent.Bobbin, data map[string]interface{}) error {
	// 载荷中的 lot_id 是 Edge 本地 UUID，一律不采信；按 lot_number 解引用父行。
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}
	palletID, err := h.resolveParentPalletID(ctx, data)
	if err != nil {
		return err
	}
	cartonID, err := h.resolveCartonByNumber(ctx, getString(data, "carton_number"))
	if err != nil {
		return err
	}
	barrelID, err := h.resolveParentBarrelID(ctx, data)
	if err != nil {
		return err
	}

	err = row.Update().
		SetLotID(lotID).
		SetNillablePalletID(palletID).
		SetNillableCartonID(cartonID).
		SetNillableBarrelID(barrelID).
		SetSpinningPosition(getInt(data, "spinning_position")).
		SetGrossWeight(getFloat(data, "gross_weight")).
		SetNetWeight(getFloat(data, "net_weight")).
		SetNillableTareWeight(getFloatPtr(data, "tare_weight")).
		SetNillableGrade(getStringPtr(data, "grade")).
		SetStatus(bobbin.Status(getString(data, "status"))).
		SetLabelPrinted(getBool(data, "label_printed")).
		SetNillablePrintedAt(getTimePtr(data, "printed_at")).
		SetNillableCompletedAt(getTimePtr(data, "completed_at")).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update bobbin: %w", err)
	}
	log.Printf("🔁 Updated bobbin: %s", row.BobbinNumber)
	return nil
}

// upsertDoffing 插入或更新落纱记录。
//
// 父批次按 lot_number 解引用（见 resolveParentLotID）。
// spinning_line_id 在 schema 中不是外键（Doffing.Edges() 只声明 lot），
// 故直接落库，不需要查父行。
//
// 状态流转（pending → confirmed → cancelled）走同一行的更新，
// 故每次状态变更重传都应命中原行。
//
// 不带 edgeID 参数：doffings 表没有 edge_id 列（来源设备经 lot 外键传递），
// 与 upsertBobbin 同 —— 那里为对齐调用链签名而保留该参数，此处径行去掉。
func (h *UploadHandler) upsertDoffing(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}
	lineID, err := uuid.Parse(getString(data, "spinning_line_id"))
	if err != nil {
		return fmt.Errorf("invalid spinning_line_id %q: %w", getString(data, "spinning_line_id"), err)
	}

	// operator_id 是 UUID 列且可选：载荷带该键时必须是合法 UUID，
	// 解析失败按无效参数拒绝该条（不静默丢弃操作员归属）。
	// doffings 无 operator 外键（Doffing.Edges() 只声明 lot），故直接落库，
	// 不需像 lot_id 那样按业务编号解引用父行。
	var operatorID *uuid.UUID
	if raw := getString(data, "operator_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid operator_id %q: %w", raw, err)
		}
		operatorID = &parsed
	}

	existing, err := h.client.Doffing.Query().Where(doffing.IDEQ(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	}

	if existing == nil {
		_, err = h.client.Doffing.Create().
			SetID(id).
			SetLotID(lotID).
			SetSpinningLineID(lineID).
			SetSpinningPosition(getInt(data, "spinning_position")).
			SetStatus(doffing.Status(getString(data, "status"))).
			SetNillableOperatorID(operatorID).
			SetNillableBobbinNumber(getStringPtr(data, "bobbin_number")).
			SetNillableActualWeight(getFloatPtr(data, "actual_weight")).
			SetNillableGrade(getStringPtr(data, "grade")).
			SetNillableCancelReason(getStringPtr(data, "cancel_reason")).
			SetNillableDoffingTime(getTimePtr(data, "doffing_time")).
			SetNillableConfirmedAt(getTimePtr(data, "confirmed_at")).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create doffing: %w", err)
		}
		log.Printf("✅ Created doffing: %s", id)
		return nil
	}

	if err := existing.Update().
		SetLotID(lotID).
		SetSpinningLineID(lineID).
		SetSpinningPosition(getInt(data, "spinning_position")).
		SetStatus(doffing.Status(getString(data, "status"))).
		SetNillableOperatorID(operatorID).
		SetNillableBobbinNumber(getStringPtr(data, "bobbin_number")).
		SetNillableActualWeight(getFloatPtr(data, "actual_weight")).
		SetNillableGrade(getStringPtr(data, "grade")).
		SetNillableCancelReason(getStringPtr(data, "cancel_reason")).
		SetNillableDoffingTime(getTimePtr(data, "doffing_time")).
		SetNillableConfirmedAt(getTimePtr(data, "confirmed_at")).
		Exec(ctx); err != nil {
		return fmt.Errorf("update doffing: %w", err)
	}
	log.Printf("🔁 Updated doffing: %s", id)
	return nil
}

// upsertBobbinGrade 插入质检记录（只增不改）。
//
// 质检明细是历史事实，重传同一条不应覆盖已有的结论，故存在即视为已处理。
//
// Center 侧不采信载荷的 bobbin_id 直接落库，而是按 bobbin_number 找到
// 本地丝饼行 —— 与 lot 同理，两端 UUID 不同。
//
// 不带 edgeID 参数：bobbin_grades 表没有 edge_id 列，与 upsertDoffing 同。
func (h *UploadHandler) upsertBobbinGrade(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}

	exists, err := h.client.BobbinGrade.Query().Where(bobbingrade.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		// 只增语义：同 ID 重传视为已处理，不更新（质检结论不应被覆盖）
		return nil
	}

	bobbinNumber := getString(data, "bobbin_number")
	if bobbinNumber == "" {
		return fmt.Errorf("missing bobbin_number for entry %s（无法定位质检对象）", id)
	}
	parent, err := h.client.Bobbin.Query().Where(bobbin.BobbinNumberEQ(bobbinNumber)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			// 父丝饼尚未到达：让本条目计重试，等丝饼上传后再挂载。
			return fmt.Errorf("parent bobbin %q not found: %w", bobbinNumber, err)
		}
		return err
	}

	// inspector_id 是 UUID 列且可选：载荷带该键时必须是合法 UUID，
	// 解析失败按无效参数拒绝该条（不静默丢弃质检员）。
	var inspectorID *uuid.UUID
	if raw := getString(data, "inspector_id"); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid inspector_id %q: %w", raw, err)
		}
		inspectorID = &parsed
	}

	// inspected_at 非 Optional（schema 内 Default(time.Now)），用 SetInspectedAt；
	// 键缺席时以当前时间兜底（Center 无法回溯 Edge 的质检时刻）。
	//
	// is_qualified 是现场陈述，三态：键缺席 / 明确 false / 明确 true。
	// getBool 会把缺席读成 false，把每一枚未带该键的丝饼记成「质检不合格」——
	// 一个安全相关字段朝危险方向的默认值反转。用 Nillable 版本透传：
	// nil 时不写值，插入时落 NULL（schema 无默认值，见 schema/bobbingrade.go
	// 的 Optional().Nillable()）。
	builder := h.client.BobbinGrade.Create().
		SetID(id).
		SetBobbinID(parent.ID).
		SetGrade(getString(data, "grade")).
		SetNillableIsQualified(getBoolPtr(data, "is_qualified")).
		SetNillableInspectorID(inspectorID).
		SetNillableDefectReason(getStringPtr(data, "defect_reason")).
		SetNillableFinalScore(getFloatPtr(data, "final_score")).
		SetInspectedAt(getTimeOrNow(data, "inspected_at"))

	// scores 与 inspection_details 是可空 JSONB：只在载荷确实带了非空对象时
	// 才写，键缺席或空 map 都保持 NULL。「未录入」与「录入了一个空对象」
	// 必须在库里可区分，否则报表按项展开时无法判断该行有没有评分。
	if scores := getJSONObject(data, "scores"); len(scores) > 0 {
		builder.SetScores(scores)
	}
	if details := getJSONObject(data, "inspection_details"); len(details) > 0 {
		builder.SetInspectionDetails(details)
	}

	_, err = builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("create bobbin grade: %w", err)
	}
	return nil
}

// getTimeOrNow 读取时间字段；缺失或无法解析时返回当前时间。
// 供非 Optional 的时间列使用（如 BobbinGrade.inspected_at）。
func getTimeOrNow(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case time.Time:
			return t
		case string:
			if parsed, err := time.Parse(time.RFC3339, t); err == nil {
				return parsed
			}
		}
	}
	return time.Now()
}

// handleUpdate 处理更新操作。
//
// 未实现，且必须显式报错而非静默返回 nil：HandleUpload 把 nil 计为 applied，
// 返回 nil 会让一条 update 记录在"没写任何东西"的情况下被回报为已应用 ——
// 正是本同步机制要消灭的那类失败。这里返回错误，让它落进 rejected 并带上
// 可读原因。Edge 侧所有采集器当前都硬编码 Operation: "create"，所以本条
// 路径在正常运行中不可达；这是为"将来有人发出 update"准备的显式拒绝。
func (h *UploadHandler) handleUpdate(ctx context.Context, entry *models.UploadEntry) error {
	return fmt.Errorf("unsupported operation: %s", entry.Operation)
}

// handleDelete 处理删除操作。理由同 handleUpdate：未实现即拒绝，不得假报成功。
func (h *UploadHandler) handleDelete(ctx context.Context, entry *models.UploadEntry) error {
	return fmt.Errorf("unsupported operation: %s", entry.Operation)
}

// 辅助函数：从map中提取各种类型的值

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringPtr(m map[string]interface{}, key string) *string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return &s
		}
	}
	return nil
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func getFloatPtr(m map[string]interface{}, key string) *float64 {
	if v, ok := m[key]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return &f
		}
	}
	return nil
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// getBoolPtr 读取布尔字段；键缺席或类型不符时返回 nil。
//
// 供三态布尔列使用：nil 交给 SetNillableXxx，插入时落 NULL（键缺席即
// 「未陈述」），而不是被 getBool 顶替成 false。
func getBoolPtr(m map[string]interface{}, key string) *bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return &b
		}
	}
	return nil
}

// getJSONObject 读取 JSONB 对象的载荷。
//
// 只接受 map 形态：Edge 侧的 ent 字段本身就是 map[string]interface{}，
// json.Marshal 后仍是对象。收到别的类型（字符串、数组、null）说明
// 两端对载荷的理解已经不一致，返回空 map 让调用方跳过写入，
// 由「该条是否为质检评分」在上层自行判断 —— 这里不 panic、不写半截数据。
func getJSONObject(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	obj, _ := v.(map[string]interface{})
	return obj
}

func getTimePtr(m map[string]interface{}, key string) *time.Time {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case time.Time:
			return &val
		case string:
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				return &t
			}
		}
	}
	return nil
}

// getUUIDPtr 读取可选 UUID 字段；键缺席、类型不符或串非法时返回 nil。
//
// 非法 UUID 按「未提供」处理而不是报错：palletizer_id 这类引用列不参与
// 业务解引用，静默丢弃比整条拒绝更合适 —— 非法的码垛机 ID 不该让托盘本身
// 丢失。真正需要校验 UUID 的字段（insector_id 等）各有显式解析路径。
func getUUIDPtr(m map[string]interface{}, key string) *uuid.UUID {
	raw := getString(m, key)
	if raw == "" {
		return nil
	}
	parsed, err := uuid.Parse(raw)
	if err != nil {
		log.Printf("⚠️  忽略非法 %s=%q：%v", key, raw, err)
		return nil
	}
	return &parsed
}
