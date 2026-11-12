package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/grade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/spinningline"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
)

// SyncClient 边端数据同步客户端（下行：Center → Edge 基础配置）
//
// 上行不在这里：上传由 Uploader 负责，它带待同步状态机与重试计数。
// 本客户端只处理「拉取 Center 的权威配置并落地到本地」这一件事。
type SyncClient struct {
	edgeID    string
	centerURL string
	token     string
	client    *ent.Client
	http      *http.Client
}

// NewSyncClient 创建同步客户端。
//
// token 是设备凭证（CENTER_TOKEN），与 Uploader 用的是同一个 ——
// Center 的 base-data 端点走同一套 authenticateEdge 校验。
func NewSyncClient(client *ent.Client, edgeID, centerURL, token string) *SyncClient {
	return &SyncClient{
		client:    client,
		edgeID:    edgeID,
		centerURL: centerURL,
		token:     token,
		http:      &http.Client{Timeout: 30 * time.Second},
	}
}

// baseDataTables 是本客户端拉取的表。
//
// spinning_lines 是机台名称的权威来源（Center 建行，Edge 从无生产者）；
// grades 是等级编码字典。edges 不拉 —— 设备只需要自己那一行，而
// /health 已经带上了 edge_id，没有消费方。
var baseDataTables = []string{"spinning_lines", "grades"}

// DownloadResult 一次下载的结果。
type DownloadResult struct {
	// Downloaded 是 Center 下发的记录数（含已存在而无需改动的）。
	Downloaded int `json:"downloaded"`
	// Applied 是实际写入本地库的记录数（新建 + 更新的合计）。
	Applied int `json:"applied"`
	// ByTable 按表拆分，便于排查「某一类配置没下来」。
	ByTable map[string]int `json:"by_table"`
}

// DownloadData 从 Center 拉取基础配置并落地到本地。
//
// 幂等：重复调用不会产生重复行，也不会把已存在的本地行改成另一套 ID。
func (c *SyncClient) DownloadData(ctx context.Context) (*DownloadResult, error) {
	payload, err := c.fetchBaseData(ctx)
	if err != nil {
		return nil, err
	}

	result := &DownloadResult{ByTable: make(map[string]int, len(payload.Data))}

	for _, table := range baseDataTables {
		rows := payload.Data[table]
		result.Downloaded += len(rows)
		if len(rows) == 0 {
			result.ByTable[table] = 0
			continue
		}

		n, err := c.applyTable(ctx, table, rows)
		if err != nil {
			return nil, fmt.Errorf("apply %s: %w", table, err)
		}
		result.ByTable[table] = n
		result.Applied += n
	}

	return result, nil
}

// fetchBaseData 调 Center 的 base-data 端点。
//
// 必须用 :code 寻址（与 Uploader 同理）：Center 的 authenticateEdge 拿
// c.Param("code") 与 token 里的 DeviceID 比对，路径缺段会被判为
// 「凭证与请求设备不符」。
func (c *SyncClient) fetchBaseData(ctx context.Context) (*models.BaseDataPullResponse, error) {
	q := url.Values{}
	for _, t := range baseDataTables {
		q.Add("tables", t)
	}

	pullURL := fmt.Sprintf("%s/v1/edges/%s/base-data?%s",
		c.centerURL, url.PathEscape(c.edgeID), q.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build base-data request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get base-data: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 凭证类失败与上传侧同等处理：这是配置问题，重试无意义，
	// 但也不该被当成「数据有问题」而反复重发。
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("base-data unauthorized: status=%d body=%s（凭证或设备注册问题）",
			resp.StatusCode, string(body))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("base-data failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Code    int                         `json:"code"`
		Message string                      `json:"message"`
		Data    models.BaseDataPullResponse `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode base-data response: %w（body=%s）", err, string(body))
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("base-data returned code=%d message=%s", envelope.Code, envelope.Message)
	}

	return &envelope.Data, nil
}

// applyTable 按表分派落地逻辑。
func (c *SyncClient) applyTable(ctx context.Context, table string, rows []map[string]interface{}) (int, error) {
	switch table {
	case "spinning_lines":
		return c.applySpinningLines(ctx, rows)
	case "grades":
		return c.applyGrades(ctx, rows)
	default:
		// 未知表不静默跳过：说明两端对 baseDataTables 的理解已经不一致，
		// 静默忽略会让「拉到了但没落地」变成无迹可查的现象。
		return 0, fmt.Errorf("unknown base-data table: %s", table)
	}
}

// applySpinningLines 落地线体配置。
//
// **主键沿用 Center 的 UUID**：这是本函数存在的理由。Center 的报表按
// doffing.spinning_line_id 关联线体名称，而该字段是 Edge 原样透传上去的。
// 若 Edge 另生成一套 ID，两边永远对不上，报表依旧只能显示 UUID。
//
// 已存在同名行时不改主键（ent 不支持改主键，且本地已有的 doffing 行可能
// 正引用它），只更新其余字段并告警 —— 这种情况说明这台 Edge 曾经自行
// 建过线体，属于需要人工介入的配置漂移。
func (c *SyncClient) applySpinningLines(ctx context.Context, rows []map[string]interface{}) (int, error) {
	applied := 0

	for _, row := range rows {
		id, err := uuid.Parse(getString(row, "id"))
		if err != nil {
			return applied, fmt.Errorf("invalid spinning_line id %q: %w", getString(row, "id"), err)
		}
		lineName := getString(row, "line_name")
		if lineName == "" {
			// line_name 是 NotEmpty + Unique，缺失即无法定位既有行。
			return applied, fmt.Errorf("spinning_line %s missing line_name", id)
		}

		existing, err := c.client.SpinningLine.Query().
			Where(spinningline.LineNameEQ(lineName)).
			First(ctx)
		switch {
		case err == nil:
			if existing.ID != id {
				log.Printf("⚠️  线体 %q 本地 ID 与 Center 不一致（本地 %s, Center %s）："+
					"保留本地 ID，机台名称在报表中将无法关联",
					lineName, existing.ID, id)
			}
			upd := c.client.SpinningLine.UpdateOneID(existing.ID).
				SetLineName(lineName).
				SetStatus(spinningline.Status(getString(row, "status")))
			applySpinningLineOptional(upd, row)
			if _, err := upd.Save(ctx); err != nil {
				return applied, fmt.Errorf("update spinning_line %q: %w", lineName, err)
			}
		case ent.IsNotFound(err):
			builder := c.client.SpinningLine.Create().
				SetID(id).
				SetLineName(lineName).
				SetStatus(spinningline.Status(getString(row, "status")))
			if v := getString(row, "line_number"); v != "" {
				builder.SetLineNumber(v)
			}
			if v := getString(row, "location"); v != "" {
				builder.SetLocation(v)
			}
			if v, ok := getInt(row, "capacity"); ok && v > 0 {
				builder.SetCapacity(v)
			}
			if _, err := builder.Save(ctx); err != nil {
				return applied, fmt.Errorf("create spinning_line %q: %w", lineName, err)
			}
		default:
			return applied, fmt.Errorf("query spinning_line %q: %w", lineName, err)
		}

		applied++
	}

	return applied, nil
}

// applySpinningLineOptional 把可选字段写到更新语句上。
//
// 与创建分支分开写是因为 ent 的 UpdateOne 与 Create 是两套 builder，
// 没有共同的 SetNillable 接口可复用。字段为空时**不写**：Center 下发
// 时省略空串键，把「未指定」写成「指定了空值」会让本地凭空多出空值。
func applySpinningLineOptional(upd *ent.SpinningLineUpdateOne, row map[string]interface{}) {
	if v := getString(row, "line_number"); v != "" {
		upd.SetLineNumber(v)
	}
	if v := getString(row, "location"); v != "" {
		upd.SetLocation(v)
	}
	if v, ok := getInt(row, "capacity"); ok && v > 0 {
		upd.SetCapacity(v)
	}
}

// applyGrades 落地等级字典。
//
// 以 (grade_type, grade_code) 定位既有行：grade_code 在 schema 里不唯一
// （同一编码可以同时存在于分拣等级与外观等级两套体系里），单用编码会
// 让两套体系互相覆盖。
func (c *SyncClient) applyGrades(ctx context.Context, rows []map[string]interface{}) (int, error) {
	applied := 0

	for _, row := range rows {
		id, err := uuid.Parse(getString(row, "id"))
		if err != nil {
			return applied, fmt.Errorf("invalid grade id %q: %w", getString(row, "id"), err)
		}
		gradeType := grade.GradeType(getString(row, "grade_type"))
		gradeCode := getString(row, "grade_code")
		if gradeCode == "" {
			return applied, fmt.Errorf("grade %s missing grade_code", id)
		}
		if err := grade.GradeTypeValidator(gradeType); err != nil {
			return applied, fmt.Errorf("grade %s has invalid grade_type %q: %w", id, gradeType, err)
		}

		existing, err := c.client.Grade.Query().
			Where(grade.GradeTypeEQ(gradeType), grade.GradeCodeEQ(gradeCode)).
			First(ctx)
		switch {
		case err == nil:
			upd := c.client.Grade.UpdateOneID(existing.ID).
				SetGradeName(getString(row, "grade_name")).
				SetSortOrder(getIntOrZero(row, "sort_order")).
				SetIsActive(getBoolDefault(row, "is_active", true))
			if v := getString(row, "description"); v != "" {
				upd.SetDescription(v)
			}
			if _, err := upd.Save(ctx); err != nil {
				return applied, fmt.Errorf("update grade %s/%s: %w", gradeType, gradeCode, err)
			}
		case ent.IsNotFound(err):
			builder := c.client.Grade.Create().
				SetID(id).
				SetGradeType(gradeType).
				SetGradeCode(gradeCode).
				SetGradeName(getString(row, "grade_name")).
				SetSortOrder(getIntOrZero(row, "sort_order")).
				SetIsActive(getBoolDefault(row, "is_active", true))
			if v := getString(row, "description"); v != "" {
				builder.SetDescription(v)
			}
			if _, err := builder.Save(ctx); err != nil {
				return applied, fmt.Errorf("create grade %s/%s: %w", gradeType, gradeCode, err)
			}
		default:
			return applied, fmt.Errorf("query grade %s/%s: %w", gradeType, gradeCode, err)
		}

		applied++
	}

	return applied, nil
}

// getString 读字符串字段，缺失或类型不符时返回空串。
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// getInt 读整数字段。JSON 解码后数字一律是 float64，故需转换。
func getInt(m map[string]interface{}, key string) (int, bool) {
	switch v := m[key].(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	}
	return 0, false
}

func getIntOrZero(m map[string]interface{}, key string) int {
	v, _ := getInt(m, key)
	return v
}

// getBoolDefault 读布尔字段，缺失时取 def。
func getBoolDefault(m map[string]interface{}, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}
