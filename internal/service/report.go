package service

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
)

// ReportService 报表聚合服务。
//
// 只读：本服务不提供任何写方法。报表的全部数据来自 Edge 同步到 Center 的
// 记录，Center 不得在此产生或修正业务数据。
type ReportService struct {
	client *ent.Client
}

// NewReportService 创建报表聚合服务
func NewReportService(client *ent.Client) *ReportService {
	return &ReportService{client: client}
}

// ReportRange 回显实际生效的统计区间（缺省值填充之后的值）。
type ReportRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// QualifiedStats 合格率统计。
//
// IsQualified 是三态：已判定合格、已判定不合格、质检员未陈述结论（NULL）。
// QualifiedRate 的分母只含前两者 —— 把 NULL 算进分母会把「没判定」当成
// 「不合格」，这是本报表最容易出错的一处口径。
type QualifiedStats struct {
	QualifiedCount   int     `json:"qualified_count"`
	UnqualifiedCount int     `json:"unqualified_count"`
	UnspecifiedCount int     `json:"unspecified_count"`
	TotalCount       int     `json:"total_count"`
	QualifiedRate    float64 `json:"qualified_rate"`
}

// GradeCount 等级分布的一行。
type GradeCount struct {
	Grade string `json:"grade"`
	Count int    `json:"count"`
}

// DefectCount 缺陷原因 Top N 的一行。
type DefectCount struct {
	DefectReason string `json:"defect_reason"`
	Count        int    `json:"count"`
}

// QualityDistributionResponse 看板二「质量分布」的响应体。
type QualityDistributionResponse struct {
	Range      ReportRange    `json:"range"`
	Qualified  QualifiedStats `json:"qualified"`
	ByGrade    []GradeCount   `json:"by_grade"`
	TopDefects []DefectCount  `json:"top_defects"`
}

// SpinningLineStat 按机台汇总的一行。
type SpinningLineStat struct {
	SpinningLineID string  `json:"spinning_line_id"`
	DoffingCount   int     `json:"doffing_count"`
	ActualWeight   float64 `json:"actual_weight"`
}

// OperatorStat 按操作员汇总的一行。
//
// OperatorID 用指针且**不加 omitempty**：operator_id 可空，未记录操作员的落纱
// 归入 null 组。若用 omitempty，整个字段会从 JSON 中消失，前端无法区分
// 「没有这一组」与「这一组是未记录操作员的」。
type OperatorStat struct {
	OperatorID   *string `json:"operator_id"`
	DoffingCount int     `json:"doffing_count"`
}

// InspectorStat 按检测员汇总的一行。InspectorID 的指针语义同 OperatorStat。
type InspectorStat struct {
	InspectorID     *string `json:"inspector_id"`
	InspectionCount int     `json:"inspection_count"`
}

// MachineOperatorResponse 看板三「机台与操作员」的响应体。
type MachineOperatorResponse struct {
	Range          ReportRange        `json:"range"`
	BySpinningLine []SpinningLineStat `json:"by_spinning_line"`
	ByOperator     []OperatorStat     `json:"by_operator"`
	ByInspector    []InspectorStat    `json:"by_inspector"`
}

// spinningLineGroupRow 是机台汇总的扫描目标。
//
// Sum 的标签必须是 "sum"：ent 的 Aggregate 对未别名的聚合列由驱动给出
// 形如 SUM(...) 的列名，而 ent 的扫描器把结构体字段名小写后与之匹配
// （sql/scan.go: columnName）。实测写成 "sum_actual_weight" 会报
// "missing struct field for column: SUM(...)"。
type spinningLineGroupRow struct {
	SpinningLineID uuid.UUID       `json:"spinning_line_id"`
	Count          int             `json:"count"`
	Sum            sql.NullFloat64 `json:"sum"`
}

// operatorGroupRow 是操作员汇总的扫描目标。
type operatorGroupRow struct {
	OperatorID *uuid.UUID `json:"operator_id"`
	Count      int        `json:"count"`
}

// inspectorGroupRow 是检测员汇总的扫描目标。
type inspectorGroupRow struct {
	InspectorID *uuid.UUID `json:"inspector_id"`
	Count       int        `json:"count"`
}

// MachineOperator 汇总区间内的落纱与质检工作量。
//
// 落纱侧排除 status=cancelled：作废的落纱不是产量，计入会让机台产量与
// 操作员落纱数双双虚高。pending 与 confirmed 均计入 —— 落纱动作已发生，
// 只是确认状态不同。
//
// 时间过滤：落纱用 doffing_time，质检用 inspected_at。两个字段不同，
// 混用不会有任何报错，只会让数字静默偏差。
func (s *ReportService) MachineOperator(ctx context.Context, start, end time.Time) (*MachineOperatorResponse, error) {
	doffings := func() *ent.DoffingQuery {
		return s.client.Doffing.Query().Where(
			doffing.DoffingTimeGTE(start),
			doffing.DoffingTimeLT(end),
			doffing.StatusNEQ(doffing.StatusCancelled),
		)
	}

	var lineRows []spinningLineGroupRow
	if err := doffings().
		GroupBy(doffing.FieldSpinningLineID).
		Aggregate(ent.Count(), ent.Sum(doffing.FieldActualWeight)).
		Scan(ctx, &lineRows); err != nil {
		return nil, err
	}

	byLine := make([]SpinningLineStat, 0, len(lineRows))
	for _, r := range lineRows {
		// SUM 在整组权重皆为 NULL 时返回 NULL；归一化为 0，不把 null 透传给前端。
		weight := 0.0
		if r.Sum.Valid {
			weight = r.Sum.Float64
		}
		byLine = append(byLine, SpinningLineStat{
			SpinningLineID: r.SpinningLineID.String(),
			DoffingCount:   r.Count,
			ActualWeight:   weight,
		})
	}
	sort.Slice(byLine, func(i, j int) bool {
		if byLine[i].DoffingCount != byLine[j].DoffingCount {
			return byLine[i].DoffingCount > byLine[j].DoffingCount
		}
		return byLine[i].SpinningLineID < byLine[j].SpinningLineID
	})

	var opRows []operatorGroupRow
	if err := doffings().
		GroupBy(doffing.FieldOperatorID).
		Aggregate(ent.Count()).
		Scan(ctx, &opRows); err != nil {
		return nil, err
	}

	byOperator := make([]OperatorStat, 0, len(opRows))
	for _, r := range opRows {
		byOperator = append(byOperator, OperatorStat{
			OperatorID:   uuidPtrToString(r.OperatorID),
			DoffingCount: r.Count,
		})
	}
	sortOperatorStats(byOperator)

	var inspRows []inspectorGroupRow
	if err := s.client.BobbinGrade.Query().
		Where(
			bobbingrade.InspectedAtGTE(start),
			bobbingrade.InspectedAtLT(end),
		).
		GroupBy(bobbingrade.FieldInspectorID).
		Aggregate(ent.Count()).
		Scan(ctx, &inspRows); err != nil {
		return nil, err
	}

	byInspector := make([]InspectorStat, 0, len(inspRows))
	for _, r := range inspRows {
		byInspector = append(byInspector, InspectorStat{
			InspectorID:     uuidPtrToString(r.InspectorID),
			InspectionCount: r.Count,
		})
	}
	sort.Slice(byInspector, func(i, j int) bool {
		if byInspector[i].InspectionCount != byInspector[j].InspectionCount {
			return byInspector[i].InspectionCount > byInspector[j].InspectionCount
		}
		return ptrStringLess(byInspector[i].InspectorID, byInspector[j].InspectorID)
	})

	return &MachineOperatorResponse{
		Range: ReportRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   end.UTC().Format(time.RFC3339),
		},
		BySpinningLine: byLine,
		ByOperator:     byOperator,
		ByInspector:    byInspector,
	}, nil
}

// uuidPtrToString 把可空 UUID 转为可空字符串。nil 保持 nil，供 JSON 输出 null。
//
// 注意：不能先转成 uuid.UUID 再判空 —— ent 把 SQL NULL 扫进非指针 uuid.UUID
// 时得到的是 uuid.Nil 而非错误，真实的零值 UUID 与 NULL 将无法区分。
func uuidPtrToString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// ptrStringLess 比较两个可空字符串，nil 排在最后。
func ptrStringLess(a, b *string) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return *a < *b
}

// sortOperatorStats 按落纱数降序、同值按操作员 ID 升序（nil 排最后）。
func sortOperatorStats(rows []OperatorStat) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].DoffingCount != rows[j].DoffingCount {
			return rows[i].DoffingCount > rows[j].DoffingCount
		}
		return ptrStringLess(rows[i].OperatorID, rows[j].OperatorID)
	})
}

// gradeGroupRow 是等级分布的分组扫描目标。
type gradeGroupRow struct {
	Grade string `json:"grade"`
	Count int    `json:"count"`
}

// defectGroupRow 是缺陷原因分组的分组扫描目标。
type defectGroupRow struct {
	DefectReason string `json:"defect_reason"`
	Count        int    `json:"count"`
}

// QualityDistribution 汇总区间内的质检结论。
//
// 计数单位是**质检记录行**，不是丝饼：同一丝饼返修后复检会留下两条记录，
// 两条都计入（spec 2026-11-09 设计文档 §3.2.1 的口径）。若要「丝饼最终合格率」，
// 需按 bobbin_id 取最新一条，那需要原生 SQL，属另一个端点。
func (s *ReportService) QualityDistribution(ctx context.Context, start, end time.Time, top int) (*QualityDistributionResponse, error) {
	base := func() *ent.BobbinGradeQuery {
		return s.client.BobbinGrade.Query().Where(
			bobbingrade.InspectedAtGTE(start),
			bobbingrade.InspectedAtLT(end),
		)
	}

	var qualified, unqualified, unspecified int

	// 一次 GROUP BY is_qualified 拿到三态计数，而不是三次独立 Count。
	//
	// 三次独立查询有两个问题：多两次往返；且并发同步写入时三次查询可能
	// 读到不同的行集，导致 qualified+unqualified+unspecified 与实际总数
	// 对不上。扫描目标用 *bool —— 非指针 bool 遇到 SQL NULL 会静默变成
	// false，把「未判定」错记成「不合格」。
	var qualifiedRows []struct {
		IsQualified *bool `json:"is_qualified"`
		Count       int   `json:"count"`
	}
	if err := base().
		GroupBy(bobbingrade.FieldIsQualified).
		Aggregate(ent.Count()).
		Scan(ctx, &qualifiedRows); err != nil {
		return nil, err
	}

	for _, r := range qualifiedRows {
		switch {
		case r.IsQualified == nil:
			unspecified += r.Count
		case *r.IsQualified:
			qualified += r.Count
		default:
			unqualified += r.Count
		}
	}

	graded := qualified + unqualified
	rate := 0.0
	if graded > 0 {
		rate = math.Round(float64(qualified)/float64(graded)*1000) / 1000
	}

	var gradeRows []gradeGroupRow
	if err := base().
		GroupBy(bobbingrade.FieldGrade).
		Aggregate(ent.Count()).
		Scan(ctx, &gradeRows); err != nil {
		return nil, err
	}

	byGrade := make([]GradeCount, 0, len(gradeRows))
	for _, r := range gradeRows {
		byGrade = append(byGrade, GradeCount{Grade: r.Grade, Count: r.Count})
	}
	sort.Slice(byGrade, func(i, j int) bool {
		if byGrade[i].Count != byGrade[j].Count {
			return byGrade[i].Count > byGrade[j].Count
		}
		return byGrade[i].Grade < byGrade[j].Grade
	})

	var defectRows []defectGroupRow
	if err := base().
		Where(
			bobbingrade.DefectReasonNotNil(),
			bobbingrade.DefectReasonNEQ(""),
		).
		GroupBy(bobbingrade.FieldDefectReason).
		Aggregate(ent.Count()).
		Scan(ctx, &defectRows); err != nil {
		return nil, err
	}

	defects := make([]DefectCount, 0, len(defectRows))
	for _, r := range defectRows {
		defects = append(defects, DefectCount{DefectReason: r.DefectReason, Count: r.Count})
	}
	sort.Slice(defects, func(i, j int) bool {
		if defects[i].Count != defects[j].Count {
			return defects[i].Count > defects[j].Count
		}
		return defects[i].DefectReason < defects[j].DefectReason
	})
	if top > 0 && len(defects) > top {
		defects = defects[:top]
	}

	return &QualityDistributionResponse{
		Range: ReportRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   end.UTC().Format(time.RFC3339),
		},
		Qualified: QualifiedStats{
			QualifiedCount:   qualified,
			UnqualifiedCount: unqualified,
			UnspecifiedCount: unspecified,
			TotalCount:       qualified + unqualified + unspecified,
			QualifiedRate:    rate,
		},
		ByGrade:    byGrade,
		TopDefects: defects,
	}, nil
}
