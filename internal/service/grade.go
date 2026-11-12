package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"

	"entgo.io/ent/dialect/sql"
)

// GradeService 质检记录查询服务。
//
// 只读：质检记录由 Edge 产生并经同步写入 Center，Center 不提供任何写接口。
type GradeService struct {
	client *ent.Client
}

// NewGradeService 创建质检记录查询服务
func NewGradeService(client *ent.Client) *GradeService {
	return &GradeService{client: client}
}

// GradeResponse 质检记录响应。
//
// IsQualified 用指针保留三态：nil = 质检员未陈述结论，
// 不能与 false（明确判定不合格）合并。
//
// Scores / InspectionDetails 是 JSONB 列，未录入时为 nil（JSON null）——
// 不能用空对象代替：报表按项展开时，「未录入」与「录入了一个空对象」
// 必须是两种可区分的事实。
type GradeResponse struct {
	ID                string                 `json:"id"`
	BobbinID          string                 `json:"bobbin_id"`
	InspectorID       *string                `json:"inspector_id"`
	Grade             string                 `json:"grade"`
	FinalScore        *float64               `json:"final_score"`
	Scores            map[string]interface{} `json:"scores"`
	InspectionDetails map[string]interface{} `json:"inspection_details"`
	IsQualified       *bool                  `json:"is_qualified"`
	DefectReason      string                 `json:"defect_reason,omitempty"`
	Notes             string                 `json:"notes,omitempty"`
	InspectedAt       time.Time              `json:"inspected_at"`
	CreatedAt         time.Time              `json:"created_at"`
}

// GradeListFilter 质检记录筛选条件。零值表示不过滤。
type GradeListFilter struct {
	LotID    string
	BobbinID string
	// IsQualified 用字符串表达三态："true" / "false" / "unspecified"。
	// 空串表示不按合格状态过滤。用字符串而非 *bool 是为了让 HTTP 查询参数
	// 能表达「未判定」这一档 —— nil 只能表达「不传」。
	IsQualified string
}

// ListGrades 按条件分页查询质检记录，按质检时间倒序。
//
// 同一丝饼可有多条记录（复检保留历史），本方法按记录返回，不做丝饼去重。
func (s *GradeService) ListGrades(ctx context.Context, page, pageSize int, f GradeListFilter) ([]*GradeResponse, int, error) {
	query := s.client.BobbinGrade.Query()

	if f.LotID != "" {
		lotID, err := uuid.Parse(f.LotID)
		if err != nil {
			return nil, 0, err
		}
		// bobbin_grade 没有指向 bobbin 的外键边（BobbinGrade.Edges() 返回 nil），
		// 只能用子查询按批次筛：bobbin_id IN (SELECT id FROM bobbins WHERE lot_id = ?)。
		// predicate.BobbinGrade 本身就是 func(*sql.Selector)，可直接构造该条件。
		query = query.Where(func(sel *sql.Selector) {
			sub := sql.Dialect(sel.Dialect()).
				Select(bobbin.FieldID).
				From(sql.Table(bobbin.Table))
			// 子查询的 FROM 是 bobbins，列名必须按 bobbins 限定。
			// 不能用 sel.C(...)：那会按**外层**表（bobbin_grades）加前缀，
			// 生成 bobbin_grades.lot_id 这种外层表根本不存在的列。
			sub.Where(sql.EQ(sub.C(bobbin.FieldLotID), lotID))
			sel.Where(sql.In(sel.C(bobbingrade.FieldBobbinID), sub))
		})
	}

	if f.BobbinID != "" {
		bobbinID, err := uuid.Parse(f.BobbinID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(bobbingrade.BobbinIDEQ(bobbinID))
	}

	switch f.IsQualified {
	case "true":
		query = query.Where(bobbingrade.IsQualified(true))
	case "false":
		query = query.Where(bobbingrade.IsQualified(false))
	case "unspecified":
		query = query.Where(bobbingrade.IsQualifiedIsNil())
	case "":
		// 不过滤
	default:
		return nil, 0, &ServiceError{Code: 10001, Message: "is_qualified 只接受 true / false / unspecified"}
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(bobbingrade.FieldInspectedAt)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*GradeResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toGradeResponse(r))
	}

	return out, total, nil
}

func toGradeResponse(g *ent.BobbinGrade) *GradeResponse {
	resp := &GradeResponse{
		ID:                g.ID.String(),
		BobbinID:          g.BobbinID.String(),
		Grade:             g.Grade,
		Scores:            g.Scores,
		InspectionDetails: g.InspectionDetails,
		IsQualified:       g.IsQualified,
		DefectReason:      g.DefectReason,
		Notes:             g.Notes,
		InspectedAt:       g.InspectedAt,
		CreatedAt:         g.CreatedAt,
	}

	// inspector_id 在 schema 中是 Optional() 而非 Nillable()，生成的是非指针
	// uuid.UUID，SQL NULL 读回为 uuid.Nil。这里用 Nil 判断并转成 JSON null，
	// 避免把「未记录检测员」显示成全零 UUID。
	if g.InspectorID != uuid.Nil {
		id := g.InspectorID.String()
		resp.InspectorID = &id
	}

	if g.FinalScore != 0 {
		score := g.FinalScore
		resp.FinalScore = &score
	}

	return resp
}

// DoffingListFilter 落纱记录筛选条件。零值表示不过滤。
type DoffingListFilter struct {
	LotID          string
	SpinningLineID string
	OperatorID     string
	Status         string
}

// ListDoffings 按条件分页查询落纱记录，按落纱时间倒序。
//
// 与 Edge 侧的 ListDoffing 分开：Edge 只需要按线体与状态查，
// Center 的批次详情页需要按批次查，两者语义不同，共用一个方法会让
// 参数列表持续膨胀。Edge 侧实现保持不变。
func (s *DoffingService) ListDoffings(ctx context.Context, page, pageSize int, f DoffingListFilter) ([]*DoffingResponse, int, error) {
	query := s.client.Doffing.Query()

	if f.LotID != "" {
		id, err := uuid.Parse(f.LotID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(doffing.LotIDEQ(id))
	}

	if f.SpinningLineID != "" {
		id, err := uuid.Parse(f.SpinningLineID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(doffing.SpinningLineIDEQ(id))
	}

	if f.OperatorID != "" {
		id, err := uuid.Parse(f.OperatorID)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where(doffing.OperatorIDEQ(id))
	}

	if f.Status != "" {
		query = query.Where(doffing.StatusEQ(doffing.Status(f.Status)))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	rows, err := query.
		Order(ent.Desc(doffing.FieldDoffingTime)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	out := make([]*DoffingResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, s.toDoffingResponse(d))
	}

	return out, total, nil
}
