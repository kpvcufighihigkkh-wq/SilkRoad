package center

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
	"github.com/google/uuid"
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

	// 权威身份由 Center 的鉴权层推导并写入 req.EdgeID（UUID 字符串）；
	// 载荷中的 edge_id 一律忽略。Task 4 必须传 UUID 而非 edge_code，
	// 否则此处 Parse 失败并退化为 uuid.Nil。
	authoritativeEdgeID, err := uuid.Parse(req.EdgeID)
	if err != nil {
		authoritativeEdgeID = uuid.Nil
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

// handleCreate 处理创建操作
func (h *UploadHandler) handleCreate(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	switch entry.Table {
	case "lots":
		return h.createLot(ctx, entry, edgeID)
	case "bobbins":
		return h.createBobbin(ctx, entry, edgeID)
	default:
		return fmt.Errorf("unknown table: %s", entry.Table)
	}
}

// createLot 创建Lot记录。
//
// edgeID 为鉴权层推导的权威设备身份（uuid.Nil 表示未推导出）；
// 载荷中的 edge_id 一律不采信，避免伪造来源设备。
func (h *UploadHandler) createLot(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	// 检查是否已存在（幂等性）
	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	exists, err := h.client.Lot.Query().Where(lot.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("⚠️  Lot %s already exists, skipping", id)
		return nil
	}

	builder := h.client.Lot.Create().
		SetID(id).
		SetLotNumber(getString(data, "lot_number")).
		SetProductType(lot.ProductType(getString(data, "product_type"))).
		SetPlannedQuantity(getInt(data, "planned_quantity")).
		SetActualQuantity(getInt(data, "actual_quantity")).
		SetStatus(lot.Status(getString(data, "status"))).
		SetNillableStartTime(getTimePtr(data, "start_time")).
		SetNillableEndTime(getTimePtr(data, "end_time"))

	if edgeID != uuid.Nil {
		builder.SetEdgeID(edgeID)
	}
	if v := getString(data, "plc_lot_number"); v != "" {
		builder.SetPlcLotNumber(v)
	}
	if v := getString(data, "order_code"); v != "" {
		builder.SetOrderCode(v)
	}
	if v := getString(data, "product_spec"); v != "" {
		builder.SetProductSpec(v)
	}

	_, err = builder.Save(ctx)
	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}

	log.Printf("✅ Created lot: %s", getString(data, "lot_number"))
	return nil
}

// createBobbin 创建Bobbin记录。
//
// edgeID 为鉴权层推导的权威设备身份。bobbins 表本身没有 edge_id 列
// （来源设备经 lot 外键传递），此参数仅为与 createLot 保持一致的调用
// 链签名而保留，当前未使用。
func (h *UploadHandler) createBobbin(ctx context.Context, entry *models.UploadEntry, edgeID uuid.UUID) error {
	data := entry.Data

	id, err := uuid.Parse(getString(data, "id"))
	if err != nil {
		return fmt.Errorf("invalid entry id %q: %w", getString(data, "id"), err)
	}
	lotID, err := uuid.Parse(getString(data, "lot_id"))
	if err != nil {
		return fmt.Errorf("invalid lot_id %q: %w", getString(data, "lot_id"), err)
	}

	// 检查是否已存在（幂等性）
	exists, err := h.client.Bobbin.Query().Where(bobbin.IDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if exists {
		log.Printf("⚠️  Bobbin %s already exists, skipping", id)
		return nil
	}

	_, err = h.client.Bobbin.Create().
		SetID(id).
		SetBobbinNumber(getString(data, "bobbin_number")).
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
		return fmt.Errorf("create bobbin: %w", err)
	}

	log.Printf("✅ Created bobbin: %s", getString(data, "bobbin_number"))
	return nil
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
