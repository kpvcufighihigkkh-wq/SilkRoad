package center

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// entityDependencyOrder 定义上传实体的依赖顺序。
//
// 外键要求被引用者先到：barrel 引用 lot，bobbin 引用 lot 与 barrel。
// 乱序到达会使 Ent 的 FK 校验失败，导致整批记录被拒。
var entityDependencyOrder = map[string]int{
	"lots":     0,
	"doffings": 1,
	"barrels":  2,
	"bobbins":  3,
	"modules":  4,
	"pallets":  5,
	"cartons":  6,
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
func (h *UploadHandler) updateBobbinRow(ctx context.Context, row *ent.Bobbin, data map[string]interface{}) error {
	// 载荷中的 lot_id 是 Edge 本地 UUID，一律不采信；按 lot_number 解引用父行。
	lotID, err := h.resolveParentLotID(ctx, data)
	if err != nil {
		return err
	}

	err = row.Update().
		SetLotID(lotID).
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
	_, err = h.client.BobbinGrade.Create().
		SetID(id).
		SetBobbinID(parent.ID).
		SetGrade(getString(data, "grade")).
		SetIsQualified(getBool(data, "is_qualified")).
		SetNillableInspectorID(inspectorID).
		SetNillableDefectReason(getStringPtr(data, "defect_reason")).
		SetInspectedAt(getTimeOrNow(data, "inspected_at")).
		Save(ctx)
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

// handleUpdate 处理更新操作
func (h *UploadHandler) handleUpdate(ctx context.Context, entry *models.UploadEntry) error {
	// TODO: 实现更新逻辑
	return nil
}

// handleDelete 处理删除操作
func (h *UploadHandler) handleDelete(ctx context.Context, entry *models.UploadEntry) error {
	// TODO: 实现删除逻辑
	return nil
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
