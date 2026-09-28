package center

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
	"github.com/google/uuid"
)

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

	applied := 0
	rejected := 0
	var errors []string

	// 按顺序处理每条记录
	for _, entry := range req.Entries {
		if err := h.applyEntry(ctx, &entry); err != nil {
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
func (h *UploadHandler) applyEntry(ctx context.Context, entry *models.UploadEntry) error {
	switch entry.Operation {
	case "create":
		return h.handleCreate(ctx, entry)
	case "update":
		return h.handleUpdate(ctx, entry)
	case "delete":
		return h.handleDelete(ctx, entry)
	default:
		return fmt.Errorf("unknown operation: %s", entry.Operation)
	}
}

// handleCreate 处理创建操作
func (h *UploadHandler) handleCreate(ctx context.Context, entry *models.UploadEntry) error {
	switch entry.Table {
	case "lots":
		return h.createLot(ctx, entry)
	case "bobbins":
		return h.createBobbin(ctx, entry)
	default:
		return fmt.Errorf("unknown table: %s", entry.Table)
	}
}

// createLot 创建Lot记录
func (h *UploadHandler) createLot(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	// 检查是否已存在（幂等性）
	id, _ := uuid.Parse(getString(data, "id"))
	exists, err := h.client.Lot.Query().Where().Count(ctx)
	if err != nil {
		return err
	}
	if exists > 0 {
		log.Printf("⚠️  Lot %s already exists, skipping", id)
		return nil
	}

	orderID, _ := uuid.Parse(getString(data, "order_id"))

	// 创建记录
	_, err = h.client.Lot.Create().
		SetID(id).
		SetLotNumber(getString(data, "lot_number")).
		SetOrderID(orderID).
		SetProductType(lot.ProductType(getString(data, "product_type"))).
		SetProductSpec(getString(data, "product_spec")).
		SetPlannedQuantity(getInt(data, "planned_quantity")).
		SetActualQuantity(getInt(data, "actual_quantity")).
		SetStatus(lot.Status(getString(data, "status"))).
		SetNillableStartTime(getTimePtr(data, "start_time")).
		SetNillableEndTime(getTimePtr(data, "end_time")).
		Save(ctx)

	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}

	log.Printf("✅ Created lot: %s", getString(data, "lot_number"))
	return nil
}

// createBobbin 创建Bobbin记录
func (h *UploadHandler) createBobbin(ctx context.Context, entry *models.UploadEntry) error {
	data := entry.Data

	id, _ := uuid.Parse(getString(data, "id"))
	lotID, _ := uuid.Parse(getString(data, "lot_id"))

	// 检查是否已存在
	exists, err := h.client.Bobbin.Query().Where().Count(ctx)
	if err != nil {
		return err
	}
	if exists > 0 {
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
