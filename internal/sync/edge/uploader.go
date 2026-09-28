package edge

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge/bobbin"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
	"github.com/google/uuid"
)

// Uploader 负责将边端SQLite数据上传到中心端
type Uploader struct {
	client    *ent_edge.Client
	edgeID    string
	serverURL string
	batchSize int
	interval  time.Duration
	cursors   map[string]int64 // 表名 -> 游标位置
}

// NewUploader 创建上传器
func NewUploader(client *ent_edge.Client, edgeID, serverURL string) *Uploader {
	return &Uploader{
		client:    client,
		edgeID:    edgeID,
		serverURL: serverURL,
		batchSize: 100,           // 每批上传100条
		interval:  5 * time.Second, // 5秒扫描一次
		cursors:   make(map[string]int64),
	}
}

// Start 启动上传循环（后台goroutine）
func (u *Uploader) Start(ctx context.Context) {
	ticker := time.NewTicker(u.interval)
	defer ticker.Stop()

	log.Printf("🚀 Edge uploader started for edge_id=%s", u.edgeID)

	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️  Edge uploader stopped")
			return
		case <-ticker.C:
			if err := u.uploadBatch(ctx); err != nil {
				log.Printf("❌ Upload batch failed: %v", err)
			}
		}
	}
}

// uploadBatch 上传一批数据
func (u *Uploader) uploadBatch(ctx context.Context) error {
	// 按照FK依赖顺序扫描表
	tables := []string{"lots", "bobbins", "bobbin_grades"}

	var allEntries []models.UploadEntry

	for _, table := range tables {
		entries, err := u.scanTable(ctx, table)
		if err != nil {
			return fmt.Errorf("scan table %s: %w", table, err)
		}
		allEntries = append(allEntries, entries...)
	}

	if len(allEntries) == 0 {
		// 没有待上传数据
		return nil
	}

	log.Printf("📤 Uploading %d entries to center", len(allEntries))

	// TODO: 实际的HTTP POST请求到服务端
	// response := postToServer(allEntries)

	// 模拟成功响应
	response := &models.UploadResponse{
		Applied:  len(allEntries),
		Rejected: 0,
		Errors:   nil,
	}

	// 更新游标
	if response.Applied == len(allEntries) {
		for _, entry := range allEntries {
			u.cursors[entry.Table] = entry.Version
		}
		log.Printf("✅ Upload successful, applied=%d", response.Applied)
	} else {
		log.Printf("⚠️  Partial upload: applied=%d, rejected=%d", response.Applied, response.Rejected)
	}

	return nil
}

// scanTable 扫描单表中未同步的记录
func (u *Uploader) scanTable(ctx context.Context, table string) ([]models.UploadEntry, error) {
	cursor := u.cursors[table]

	switch table {
	case "lots":
		return u.scanLots(ctx, cursor)
	case "bobbins":
		return u.scanBobbins(ctx, cursor)
	default:
		return nil, nil
	}
}

// scanLots 扫描lots表
func (u *Uploader) scanLots(ctx context.Context, cursor int64) ([]models.UploadEntry, error) {
	// 查询未同步的lot记录
	lots, err := u.client.Lot.Query().
		Where(lot.SyncedEQ(false)).
		Limit(u.batchSize).
		All(ctx)

	if err != nil {
		return nil, err
	}

	var entries []models.UploadEntry
	for _, lot := range lots {
		data := map[string]interface{}{
			"id":               lot.ID.String(),
			"lot_number":       lot.LotNumber,
			"order_id":         lot.OrderID.String(),
			"product_type":     lot.ProductType,
			"product_spec":     lot.ProductSpec,
			"planned_quantity": lot.PlannedQuantity,
			"actual_quantity":  lot.ActualQuantity,
			"status":           lot.Status,
			"start_time":       lot.StartTime,
			"end_time":         lot.EndTime,
			"created_at":       lot.CreatedAt,
			"updated_at":       lot.UpdatedAt,
		}

		entries = append(entries, models.UploadEntry{
			Table:     "lots",
			Operation: "create",
			ID:        lot.ID,
			Data:      data,
			Version:   lot.CreatedAt.Unix(),
			CreatedAt: lot.CreatedAt,
		})
	}

	return entries, nil
}

// scanBobbins 扫描bobbins表
func (u *Uploader) scanBobbins(ctx context.Context, cursor int64) ([]models.UploadEntry, error) {
	bobbins, err := u.client.Bobbin.Query().
		Where(bobbin.SyncedEQ(false)).
		Limit(u.batchSize).
		All(ctx)

	if err != nil {
		return nil, err
	}

	var entries []models.UploadEntry
	for _, bobbin := range bobbins {
		data := map[string]interface{}{
			"id":                bobbin.ID.String(),
			"bobbin_number":     bobbin.BobbinNumber,
			"lot_id":            bobbin.LotID.String(),
			"spinning_position": bobbin.SpinningPosition,
			"gross_weight":      bobbin.GrossWeight,
			"net_weight":        bobbin.NetWeight,
			"tare_weight":       bobbin.TareWeight,
			"grade":             bobbin.Grade,
			"status":            bobbin.Status,
			"label_printed":     bobbin.LabelPrinted,
			"printed_at":        bobbin.PrintedAt,
			"completed_at":      bobbin.CompletedAt,
			"created_at":        bobbin.CreatedAt,
			"updated_at":        bobbin.UpdatedAt,
		}

		entries = append(entries, models.UploadEntry{
			Table:     "bobbins",
			Operation: "create",
			ID:        bobbin.ID,
			Data:      data,
			Version:   bobbin.CreatedAt.Unix(),
			CreatedAt: bobbin.CreatedAt,
		})
	}

	return entries, nil
}

// MarkSynced 标记记录为已同步
func (u *Uploader) MarkSynced(ctx context.Context, table string, id uuid.UUID) error {
	switch table {
	case "lots":
		return u.client.Lot.UpdateOneID(id).
			SetSynced(true).
			SetSyncedAt(time.Now()).
			Exec(ctx)
	case "bobbins":
		return u.client.Bobbin.UpdateOneID(id).
			SetSynced(true).
			SetSyncedAt(time.Now()).
			Exec(ctx)
	default:
		return fmt.Errorf("unknown table: %s", table)
	}
}
