package service

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/predicate"
	"github.com/yourusername/igh-silkroad/internal/database/ent/spinningline"
)

// defectScanCap 缺陷原因分组在 SQL 层的最大取数。
//
// 缺陷原因是自由文本，分组数没有天然上限；Top N 的 N 通常不超过 50，
// 这里留一倍余量。上限存在的意义是给内存与查询时间一个确定边界，
// 不是为了在业务上限制原因种类。
const DefectScanCap = 100

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
//
// SpinningLineName 可空：spinning_line_id 来自 Edge，是不带外键约束的普通
// UUID 列。Edge 未从 Center 拉过线体配置时，这里只能给出 ID。
// 指针且不加 omitempty，让前端能区分「查不到名称」与「没有这个字段」。
type SpinningLineStat struct {
	SpinningLineID   string  `json:"spinning_line_id"`
	SpinningLineName *string `json:"spinning_line_name"`
	DoffingCount     int     `json:"doffing_count"`
	ActualWeight     float64 `json:"actual_weight"`
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

	// 机台名称在 Center 本地关联得出：spinning_line 是 Center 的权威配置，
	// 不需要经 Edge 回传。一次批量查询而不是逐行查，避免 N+1。
	lineIDs := make([]uuid.UUID, 0, len(lineRows))
	for _, r := range lineRows {
		lineIDs = append(lineIDs, r.SpinningLineID)
	}
	lineNames, err := s.spinningLineNames(ctx, lineIDs)
	if err != nil {
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
			SpinningLineID:   r.SpinningLineID.String(),
			SpinningLineName: lineNames[r.SpinningLineID],
			DoffingCount:     r.Count,
			ActualWeight:     weight,
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

// spinningLineNames 批量查线体名称，返回 ID → 名称（可空值）。
//
// 查不到的 ID 不出现在 map 里，调用方读到的就是 nil —— 语义是「这个 ID
// 在 Center 的线体表里没有对应行」，而不是「线体无名」。典型成因是 Edge
// 从未拉取过 Center 的线体配置（见缺口 C）。
//
// 空输入直接返回空 map，不发查询：分组结果为空时（区间内无落纱）
// WHERE IN () 在部分方言下是语法错误。
func (s *ReportService) spinningLineNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*string, error) {
	if len(ids) == 0 {
		return map[uuid.UUID]*string{}, nil
	}

	rows, err := s.client.SpinningLine.Query().
		Where(spinningline.IDIn(ids...)).
		Select(spinningline.FieldID, spinningline.FieldLineName).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]*string, len(rows))
	for _, r := range rows {
		name := r.LineName
		out[r.ID] = &name
	}
	return out, nil
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

	// 缺陷原因是自由文本，分组数没有天然上限：一条产线几年下来能攒出
	// 几百种写法（错别字、简繁混用、多写空格）。原实现在 Go 里截断，
	// 意味着先把全部分组读进内存再扔掉 —— 分组多时既慢又占内存。
	//
	// 改为在 SQL 层排序并限制：ORDER BY COUNT(*) DESC, defect_reason ASC
	// 保证与 Go 侧排序口径一致（先按计数降序，同数按名称升序），
	// 因此 LIMIT 取到的就是最终 Top N，Go 侧无需再排。
	//
	// 探针已验证 ent 0.14.6 的分组查询支持在闭包里追加 OrderBy/Limit。
	maxRows := DefectScanCap
	if top > 0 && top < maxRows {
		maxRows = top
	}

	var defectRows []defectGroupRow
	if err := base().
		Where(func(sel *entsql.Selector) {
			col := sel.C(bobbingrade.FieldDefectReason)
			sel.Where(entsql.NotNull(col))
			sel.Where(entsql.NEQ(col, ""))
			// 同数时按名称升序，保证结果稳定：否则同数的分组顺序由存储引擎
			// 决定，翻页或重跑会出现「同一份数据两种 Top N」。
			sel.OrderBy(entsql.Desc("COUNT(*)"), entsql.Asc(col))
			sel.Limit(maxRows)
		}).
		GroupBy(bobbingrade.FieldDefectReason).
		Aggregate(ent.Count()).
		Scan(ctx, &defectRows); err != nil {
		return nil, err
	}

	defects := make([]DefectCount, 0, len(defectRows))
	for _, r := range defectRows {
		defects = append(defects, DefectCount{DefectReason: r.DefectReason, Count: r.Count})
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

// LotStatusStat 按批次状态汇总的一行。
type LotStatusStat struct {
	Status          string `json:"status"`
	LotCount        int    `json:"lot_count"`
	PlannedQuantity int    `json:"planned_quantity"`
	ActualQuantity  int    `json:"actual_quantity"`
}

// ProductTypeStat 按产品类型汇总的一行。
type ProductTypeStat struct {
	ProductType     string `json:"product_type"`
	LotCount        int    `json:"lot_count"`
	PlannedQuantity int    `json:"planned_quantity"`
	ActualQuantity  int    `json:"actual_quantity"`
}

// EdgeLotStat 按来源设备汇总的一行。
//
// EdgeID 用指针且不加 omitempty，语义同 OperatorStat：未记录来源的批次
// （早期 seed 数据、或 Edge 尚未带上 edge_id）归入 null 组，
// 前端需要能把这一组与「没有这一组」区分开。
type EdgeLotStat struct {
	EdgeID          *string `json:"edge_id"`
	LotCount        int     `json:"lot_count"`
	PlannedQuantity int     `json:"planned_quantity"`
	ActualQuantity  int     `json:"actual_quantity"`
}

// ProductionTotals 区间内的产量总计。
//
// CompletionRate 是实际/计划的比值，0 表示 100%。计划量为 0 时无意义，
// 此时返回 0 而不是除零。
type ProductionTotals struct {
	LotCount        int     `json:"lot_count"`
	PlannedQuantity int     `json:"planned_quantity"`
	ActualQuantity  int     `json:"actual_quantity"`
	CompletionRate  float64 `json:"completion_rate"`
}

// ProductionOverviewResponse 看板一「生产概览」的响应体。
type ProductionOverviewResponse struct {
	Range     ReportRange       `json:"range"`
	Totals    ProductionTotals  `json:"totals"`
	ByStatus  []LotStatusStat   `json:"by_status"`
	ByProduct []ProductTypeStat `json:"by_product"`
	ByEdge    []EdgeLotStat     `json:"by_edge"`
}

// lotGroupRow 是批次分组聚合的扫描目标。
//
// 两个 SUM 必须用 ent.As 起不同别名：ent 扫描器把列名按 "(" 截断后
// 小写匹配结构体字段名，两个未别名的 SUM 都会匹配到 "sum"，第二个
// 会覆盖第一个（sql/scan.go: scanStruct）。
type lotGroupRow struct {
	Status      string        `json:"status"`
	ProductType string        `json:"product_type"`
	EdgeID      *uuid.UUID    `json:"edge_id"`
	Count       int           `json:"count"`
	SumPlanned  sql.NullInt64 `json:"sum_planned"`
	SumActual   sql.NullInt64 `json:"sum_actual"`
}

// ProductionOverview 汇总区间内的批次产出：状态分布、计划/实际产量、
// 完成率、按产品类型与按来源设备。
//
// 区间过滤用 COALESCE(start_time, created_at)：start_time 是真正的开工
// 时刻，但历史数据（以及 Edge 侧补写开工时刻之前产生的批次）该列为空，
// 直接按 start_time 过滤会让整批历史从看板上消失。
//
// 四个统计共用同一个区间谓词，逐条构造而非共享查询对象 —— ent 的
// Query 带可变状态，复用会让几次聚合的 Limit/OrderBy 相互污染。
func (s *ReportService) ProductionOverview(ctx context.Context, start, end time.Time) (*ProductionOverviewResponse, error) {
	inRange := func() predicate.Lot {
		return func(sel *entsql.Selector) {
			col := "COALESCE(" + sel.C(lot.FieldStartTime) + ", " + sel.C(lot.FieldCreatedAt) + ")"
			sel.Where(entsql.P(func(b *entsql.Builder) {
				b.Ident(col).WriteOp(entsql.OpGTE).Arg(start)
			}))
			sel.Where(entsql.P(func(b *entsql.Builder) {
				b.Ident(col).WriteOp(entsql.OpLT).Arg(end)
			}))
		}
	}

	aggregate := func(groupBy string, v any) error {
		return s.client.Lot.Query().Where(inRange()).
			GroupBy(groupBy).
			Aggregate(
				ent.Count(),
				ent.As(ent.Sum(lot.FieldPlannedQuantity), "sum_planned"),
				ent.As(ent.Sum(lot.FieldActualQuantity), "sum_actual"),
			).
			Scan(ctx, v)
	}

	var statusRows []lotGroupRow
	if err := aggregate(lot.FieldStatus, &statusRows); err != nil {
		return nil, err
	}
	sort.Slice(statusRows, func(i, j int) bool {
		if statusRows[i].Count != statusRows[j].Count {
			return statusRows[i].Count > statusRows[j].Count
		}
		return statusRows[i].Status < statusRows[j].Status
	})

	var productRows []lotGroupRow
	if err := aggregate(lot.FieldProductType, &productRows); err != nil {
		return nil, err
	}
	sort.Slice(productRows, func(i, j int) bool {
		if productRows[i].Count != productRows[j].Count {
			return productRows[i].Count > productRows[j].Count
		}
		return productRows[i].ProductType < productRows[j].ProductType
	})

	var edgeRows []lotGroupRow
	if err := aggregate(lot.FieldEdgeID, &edgeRows); err != nil {
		return nil, err
	}
	sort.Slice(edgeRows, func(i, j int) bool {
		if edgeRows[i].Count != edgeRows[j].Count {
			return edgeRows[i].Count > edgeRows[j].Count
		}
		return ptrStringLess(uuidPtrToString(edgeRows[i].EdgeID), uuidPtrToString(edgeRows[j].EdgeID))
	})

	// 总计由状态分组累加得出：状态是全集的划分，四组计数之和即总数，
	// 再跑一次不带 GroupBy 的聚合同样正确但多一次往返。
	totals := ProductionTotals{}
	byStatus := make([]LotStatusStat, 0, len(statusRows))
	for _, r := range statusRows {
		planned, actual := nullInt64ToInt(r.SumPlanned), nullInt64ToInt(r.SumActual)
		byStatus = append(byStatus, LotStatusStat{
			Status:          r.Status,
			LotCount:        r.Count,
			PlannedQuantity: planned,
			ActualQuantity:  actual,
		})
		totals.LotCount += r.Count
		totals.PlannedQuantity += planned
		totals.ActualQuantity += actual
	}
	totals.CompletionRate = completionRate(totals.ActualQuantity, totals.PlannedQuantity)

	byProduct := make([]ProductTypeStat, 0, len(productRows))
	for _, r := range productRows {
		byProduct = append(byProduct, ProductTypeStat{
			ProductType:     r.ProductType,
			LotCount:        r.Count,
			PlannedQuantity: nullInt64ToInt(r.SumPlanned),
			ActualQuantity:  nullInt64ToInt(r.SumActual),
		})
	}

	byEdge := make([]EdgeLotStat, 0, len(edgeRows))
	for _, r := range edgeRows {
		byEdge = append(byEdge, EdgeLotStat{
			EdgeID:          uuidPtrToString(r.EdgeID),
			LotCount:        r.Count,
			PlannedQuantity: nullInt64ToInt(r.SumPlanned),
			ActualQuantity:  nullInt64ToInt(r.SumActual),
		})
	}

	return &ProductionOverviewResponse{
		Range: ReportRange{
			Start: start.UTC().Format(time.RFC3339),
			End:   end.UTC().Format(time.RFC3339),
		},
		Totals:    totals,
		ByStatus:  byStatus,
		ByProduct: byProduct,
		ByEdge:    byEdge,
	}, nil
}

// nullInt64ToInt 把 SUM 的 NULL 归一化为 0。
//
// SUM 在组内权重全为 NULL 时返回 NULL；批次的两列都有默认值，
// 理论上不会为 NULL，但归一化让扫描目标与响应字段都不必携带可空语义。
func nullInt64ToInt(v sql.NullInt64) int {
	if !v.Valid {
		return 0
	}
	return int(v.Int64)
}

// completionRate 计算完成率，保留三位小数。
//
// 计划量为 0 时返回 0：批次创建时 planned_quantity 有 Positive 约束，
// 走到这里说明数据异常，返回 0 比返回 +Inf 或 NaN（JSON 无法编码）安全。
func completionRate(actual, planned int) float64 {
	if planned <= 0 {
		return 0
	}
	return math.Round(float64(actual)/float64(planned)*1000) / 1000
}
