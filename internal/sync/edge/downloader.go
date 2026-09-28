package edge

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// Downloader 负责从中心端拉取基础数据
type Downloader struct {
	client    *ent_edge.Client
	edgeID    string
	serverURL string
	interval  time.Duration
}

// NewDownloader 创建下载器
func NewDownloader(client *ent_edge.Client, edgeID, serverURL string) *Downloader {
	return &Downloader{
		client:    client,
		edgeID:    edgeID,
		serverURL: serverURL,
		interval:  30 * time.Minute, // 每30分钟拉取一次基础数据
	}
}

// Start 启动下载循环
func (d *Downloader) Start(ctx context.Context) {
	// 启动时立即拉取一次
	if err := d.pullBaseData(ctx); err != nil {
		log.Printf("❌ Initial base data pull failed: %v", err)
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	log.Printf("🚀 Edge downloader started for edge_id=%s", d.edgeID)

	for {
		select {
		case <-ctx.Done():
			log.Println("⏹️  Edge downloader stopped")
			return
		case <-ticker.C:
			if err := d.pullBaseData(ctx); err != nil {
				log.Printf("❌ Base data pull failed: %v", err)
			}
		}
	}
}

// pullBaseData 拉取基础数据
func (d *Downloader) pullBaseData(ctx context.Context) error {
	log.Printf("📥 Pulling base data from center")

	// TODO: 实际的HTTP GET请求
	// response := getFromServer(tables)

	// 模拟响应（当前Edge端没有需要同步的基础数据表）
	response := &models.BaseDataPullResponse{
		Data: make(map[string][]map[string]interface{}),
	}

	// 应用到本地数据库
	for table, records := range response.Data {
		if err := d.applyTableData(ctx, table, records); err != nil {
			return fmt.Errorf("apply table %s: %w", table, err)
		}
		log.Printf("✅ Applied %d records for table %s", len(records), table)
	}

	return nil
}

// applyTableData 应用表数据到本地
func (d *Downloader) applyTableData(ctx context.Context, table string, records []map[string]interface{}) error {
	// Edge端当前没有需要从Center同步的基础数据表
	// 未来可以在这里添加需要的表，如产品配置等
	return nil
}

// 辅助函数（与center/handler.go中的类似）

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
