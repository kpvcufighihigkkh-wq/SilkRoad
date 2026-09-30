package center

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/edge"
	"github.com/yourusername/igh-silkroad/internal/database/ent/grade"
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
	case "grades":
		return p.getGrades(ctx)
	case "edges":
		return p.getEdges(ctx)
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
		data := map[string]interface{}{
			"id":          line.ID.String(),
			"line_number": line.LineNumber,
			"line_name":   line.LineName,
			"status":      line.Status,
			"created_at":  line.CreatedAt,
			"updated_at":  line.UpdatedAt,
		}

		// 可选字段
		if line.Location != "" {
			data["location"] = line.Location
		}
		if line.Capacity > 0 {
			data["capacity"] = line.Capacity
		}
		if line.CurrentLotID != uuid.Nil {
			data["current_lot_id"] = line.CurrentLotID.String()
		}

		result = append(result, data)
	}

	return result, nil
}

// getGrades 获取等级基础数据
func (p *BaseDataProvider) getGrades(ctx context.Context) ([]map[string]interface{}, error) {
	grades, err := p.client.Grade.Query().
		Where(grade.IsActiveEQ(true)).
		Order(ent.Asc(grade.FieldSortOrder)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(grades))
	for _, g := range grades {
		result = append(result, map[string]interface{}{
			"id":          g.ID.String(),
			"grade_type":  string(g.GradeType),
			"grade_code":  g.GradeCode,
			"grade_name":  g.GradeName,
			"description": g.Description,
			"sort_order":  g.SortOrder,
			"is_active":   g.IsActive,
		})
	}

	return result, nil
}

// getEdges 获取边端设备基础数据
func (p *BaseDataProvider) getEdges(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := p.client.Edge.Query().
		Order(ent.Asc(edge.FieldEdgeCode)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(rows))
	for _, e := range rows {
		result = append(result, map[string]interface{}{
			"id":         e.ID.String(),
			"edge_code":  e.EdgeCode,
			"edge_name":  e.EdgeName,
			"ip_address": e.IPAddress,
			"status":     string(e.Status),
			"version":    e.Version,
		})
	}

	return result, nil
}
