package center

import (
	"context"
	"log"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// BaseDataProvider 提供基础数据给边端
type BaseDataProvider struct {
	client *ent.Client
}

// NewBaseDataProvider 创建基础数据提供器
func NewBaseDataProvider(client *ent.Client) *BaseDataProvider {
	return &BaseDataProvider{
		client: client,
	}
}

// HandlePullRequest 处理边端的基础数据拉取请求
func (p *BaseDataProvider) HandlePullRequest(ctx context.Context, req *models.BaseDataPullRequest) (*models.BaseDataPullResponse, error) {
	log.Printf("📤 Edge %s requesting base data: tables=%v", req.EdgeID, req.Tables)

	response := &models.BaseDataPullResponse{
		Data: make(map[string][]map[string]interface{}),
	}

	for _, table := range req.Tables {
		data, err := p.getTableData(ctx, table)
		if err != nil {
			log.Printf("❌ Failed to get data for table %s: %v", table, err)
			continue
		}
		response.Data[table] = data
		log.Printf("✅ Prepared %d records for table %s", len(data), table)
	}

	return response, nil
}

// getTableData 获取指定表的数据
func (p *BaseDataProvider) getTableData(ctx context.Context, table string) ([]map[string]interface{}, error) {
	switch table {
	case "spinning_lines":
		return p.getSpinningLines(ctx)
	case "projects":
		return p.getProjects(ctx)
	default:
		return nil, nil
	}
}

// getSpinningLines 获取纺丝线体配置
func (p *BaseDataProvider) getSpinningLines(ctx context.Context) ([]map[string]interface{}, error) {
	lines, err := p.client.SpinningLine.Query().
		Where().
		All(ctx)

	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, line := range lines {
		result = append(result, map[string]interface{}{
			"id":             line.ID.String(),
			"line_number":    line.LineNumber,
			"line_name":      line.LineName,
			"position_count": line.PositionCount,
			"workshop_area":  line.WorkshopArea,
			"product_type":   line.ProductType,
			"status":         line.Status,
			"plc_ip":         line.PlcIP,
			"plc_port":       line.PlcPort,
			"plc_protocol":   line.PlcProtocol,
			"notes":          line.Notes,
			"created_at":     line.CreatedAt,
			"updated_at":     line.UpdatedAt,
		})
	}

	return result, nil
}

// getProjects 获取项目信息
func (p *BaseDataProvider) getProjects(ctx context.Context) ([]map[string]interface{}, error) {
	projects, err := p.client.Project.Query().
		Where().
		All(ctx)

	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, proj := range projects {
		result = append(result, map[string]interface{}{
			"id":               proj.ID.String(),
			"project_number":   proj.ProjectNumber,
			"project_name":     proj.ProjectName,
			"product_type":     proj.ProductType,
			"product_spec":     proj.ProductSpec,
			"status":           proj.Status,
			"planned_quantity": proj.PlannedQuantity,
			"actual_quantity":  proj.ActualQuantity,
			"start_date":       proj.StartDate,
			"end_date":         proj.EndDate,
			"notes":            proj.Notes,
			"created_at":       proj.CreatedAt,
			"updated_at":       proj.UpdatedAt,
		})
	}

	return result, nil
}
