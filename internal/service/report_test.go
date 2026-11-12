package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// gradeSeed 描述一条待造的质检记录。isQualified 用三种状态表达口径：
// nil = 质检员未陈述结论（落 NULL），&true / &false = 明确结论。
type gradeSeed struct {
	bobbinID     *uuid.UUID
	grade        string
	inspectedAt  time.Time
	isQualified  *bool
	defectReason string
	// finalScore / inspectionDetails 为 nil 表示未录入（对应列保持 NULL）。
	finalScore        *float64
	inspectionDetails map[string]interface{}
}

// seedGrades 按 seed 列表造质检记录。
//
// bobbin_grade 在 schema 里没有外键边（BobbinGrade.Edges() 返回 nil），
// bobbin_id 只是普通 UUID 列，因此这里可以直接造随机 ID，不需要先建丝饼。
func seedGrades(t *testing.T, client *ent.Client, seeds []gradeSeed) {
	t.Helper()
	ctx := context.Background()

	for i, s := range seeds {
		bobbinID := uuid.New()
		if s.bobbinID != nil {
			bobbinID = *s.bobbinID
		}

		b := client.BobbinGrade.Create().
			SetBobbinID(bobbinID).
			SetGrade(s.grade).
			SetInspectedAt(s.inspectedAt)
		if s.isQualified != nil {
			b.SetNillableIsQualified(s.isQualified)
		}
		if s.defectReason != "" {
			b.SetDefectReason(s.defectReason)
		}
		if s.finalScore != nil {
			b.SetNillableFinalScore(s.finalScore)
		}
		if len(s.inspectionDetails) > 0 {
			b.SetInspectionDetails(s.inspectionDetails)
		}

		if _, err := b.Save(ctx); err != nil {
			t.Fatalf("seed grade #%d: %v", i, err)
		}
	}
}

// reportWindow 返回测试统一使用的区间与内部时间点。
func reportWindow() (start, end, inside, before, after time.Time) {
	end = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)
	start = end.Add(-7 * 24 * time.Hour)
	inside = start.Add(24 * time.Hour)
	before = start.Add(-24 * time.Hour)
	after = end.Add(24 * time.Hour)
	return
}

func TestReportService_QualityDistribution_QualifiedRate(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	truthy, falsy := true, false
	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: inside, isQualified: &truthy},
		{grade: "A", inspectedAt: inside, isQualified: &truthy},
		{grade: "A", inspectedAt: inside, isQualified: &truthy},
		{grade: "B", inspectedAt: inside, isQualified: &falsy},
		{grade: "B", inspectedAt: inside}, // NULL：未陈述结论
		{grade: "B", inspectedAt: inside}, // NULL
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.Qualified.QualifiedCount != 3 {
		t.Errorf("QualifiedCount = %d, want 3", got.Qualified.QualifiedCount)
	}
	if got.Qualified.UnqualifiedCount != 1 {
		t.Errorf("UnqualifiedCount = %d, want 1", got.Qualified.UnqualifiedCount)
	}
	if got.Qualified.UnspecifiedCount != 2 {
		t.Errorf("UnspecifiedCount = %d, want 2（NULL 必须单独可观测）", got.Qualified.UnspecifiedCount)
	}
	if got.Qualified.TotalCount != 6 {
		t.Errorf("TotalCount = %d, want 6", got.Qualified.TotalCount)
	}
	// 分母只含已有结论的 4 条：3/4 = 0.75，而不是 3/6 = 0.5。
	if got.Qualified.QualifiedRate != 0.75 {
		t.Errorf("QualifiedRate = %v, want 0.75（分母排除 NULL）", got.Qualified.QualifiedRate)
	}
}

func TestReportService_QualityDistribution_AllUnspecified(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: inside},
		{grade: "B", inspectedAt: inside},
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.Qualified.QualifiedRate != 0 {
		t.Errorf("QualifiedRate = %v, want 0（分母为 0 时不得出现 NaN 或除零）", got.Qualified.QualifiedRate)
	}
	if got.Qualified.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2", got.Qualified.TotalCount)
	}
}

// 同一丝饼「不合格 → 返修 → 复检合格」必须计两次：设计文档 §3.2.1 的口径是
// 「这段时间内质检工作有多少通过」，不是「这些丝饼最终合格多少」。
func TestReportService_QualityDistribution_RepeatInspectionCountedTwice(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	bobbinID := uuid.New()
	truthy, falsy := true, false
	seedGrades(t, client, []gradeSeed{
		{bobbinID: &bobbinID, grade: "C", inspectedAt: inside.Add(-time.Hour), isQualified: &falsy},
		{bobbinID: &bobbinID, grade: "A", inspectedAt: inside, isQualified: &truthy},
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.Qualified.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2（复检各计一次，不做丝饼去重）", got.Qualified.TotalCount)
	}
	if got.Qualified.QualifiedRate != 0.5 {
		t.Errorf("QualifiedRate = %v, want 0.5", got.Qualified.QualifiedRate)
	}
}

func TestReportService_QualityDistribution_GradeGroups(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: inside},
		{grade: "A", inspectedAt: inside},
		{grade: "B", inspectedAt: inside},
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if len(got.ByGrade) != 2 {
		t.Fatalf("len(ByGrade) = %d, want 2: %+v", len(got.ByGrade), got.ByGrade)
	}
	// 计数降序：A(2) 在前。
	if got.ByGrade[0].Grade != "A" || got.ByGrade[0].Count != 2 {
		t.Errorf("ByGrade[0] = %+v, want {A 2}", got.ByGrade[0])
	}
	if got.ByGrade[1].Grade != "B" || got.ByGrade[1].Count != 1 {
		t.Errorf("ByGrade[1] = %+v, want {B 1}", got.ByGrade[1])
	}
}

func TestReportService_QualityDistribution_TopDefectsTruncates(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	var seeds []gradeSeed
	for i := 0; i < 5; i++ {
		seeds = append(seeds, gradeSeed{grade: "B", inspectedAt: inside, defectReason: "毛丝"})
	}
	for i := 0; i < 3; i++ {
		seeds = append(seeds, gradeSeed{grade: "B", inspectedAt: inside, defectReason: "断头"})
	}
	seeds = append(seeds, gradeSeed{grade: "B", inspectedAt: inside, defectReason: "油污"})
	seedGrades(t, client, seeds)

	got, err := svc.QualityDistribution(context.Background(), start, end, 2)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if len(got.TopDefects) != 2 {
		t.Fatalf("len(TopDefects) = %d, want 2（top=2）", len(got.TopDefects))
	}
	if got.TopDefects[0].DefectReason != "毛丝" || got.TopDefects[0].Count != 5 {
		t.Errorf("TopDefects[0] = %+v, want {毛丝 5}", got.TopDefects[0])
	}
	if got.TopDefects[1].DefectReason != "断头" || got.TopDefects[1].Count != 3 {
		t.Errorf("TopDefects[1] = %+v, want {断头 3}", got.TopDefects[1])
	}
}

// NULL 与空串都不是「有缺陷原因」：把它们计入 Top N 会造出一个假分组。
func TestReportService_QualityDistribution_SkipsNullAndEmptyDefect(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{grade: "B", inspectedAt: inside},                     // defect_reason 为 NULL
		{grade: "B", inspectedAt: inside, defectReason: "毛丝"}, // 唯一有效原因
	})
	// 手工造一条空串：SetDefectReason("") 会被 ent 写为空串而非 NULL。
	if _, err := client.BobbinGrade.Create().
		SetBobbinID(uuid.New()).
		SetGrade("B").
		SetInspectedAt(inside).
		SetDefectReason("").
		Save(context.Background()); err != nil {
		t.Fatalf("seed empty defect reason: %v", err)
	}

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if len(got.TopDefects) != 1 {
		t.Fatalf("len(TopDefects) = %d, want 1（NULL 与空串都应排除）: %+v", len(got.TopDefects), got.TopDefects)
	}
	if got.TopDefects[0].DefectReason != "毛丝" {
		t.Errorf("TopDefects[0] = %+v, want 毛丝", got.TopDefects[0])
	}
}

// 时间窗口只认 inspected_at：这条用例与 TopDefects 等用例共用同一批实体，
// 但只有落在 [start, end) 内的记录才被计入。
func TestReportService_QualityDistribution_TimeWindow(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, before, after := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: before}, // 窗口外（早于 start）
		{grade: "A", inspectedAt: inside}, // 窗口内
		{grade: "A", inspectedAt: end},    // 恰好等于 end：右开，计入窗口外
		{grade: "A", inspectedAt: after},  // 窗口外（晚于 end）
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.Qualified.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1（[start, end) 左闭右开）", got.Qualified.TotalCount)
	}
}

func TestReportService_QualityDistribution_EmptyRange(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, _, _, _ := reportWindow()

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.Qualified.TotalCount != 0 || got.Qualified.QualifiedRate != 0 {
		t.Errorf("空库时 qualified = %+v, want 全零", got.Qualified)
	}
	if got.ByGrade == nil {
		t.Error("ByGrade 为 nil，应初始化为空切片（JSON 应输出 [] 而非 null）")
	}
	if len(got.ByGrade) != 0 {
		t.Errorf("len(ByGrade) = %d, want 0", len(got.ByGrade))
	}
	if got.TopDefects == nil {
		t.Error("TopDefects 为 nil，应初始化为空切片")
	}
}

// 缺陷原因是自由文本，分组数没有天然上限。
//
// 本用例钉住两件事：
//  1. 超过 DefectScanCap 时结果被截在 SQL 层的上限，不会把全部分组拉进内存；
//  2. 截断取的是计数最高的那些分组 —— 排序在 SQL 层完成，不是取前 N 行再排。
func TestReportService_QualityDistribution_DefectScanIsCappedInSQL(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	// 造 DefectScanCap+20 种原因。全部计数为 1，只让「缺陷-000」为 5，
	// 这样结果集很小，同时能确定唯一的第一名。
	var seeds []gradeSeed
	for i := 0; i < service.DefectScanCap+20; i++ {
		reason := fmt.Sprintf("缺陷-%03d", i)
		n := 1
		if i == 0 {
			n = 5
		}
		for k := 0; k < n; k++ {
			seeds = append(seeds, gradeSeed{grade: "B", inspectedAt: inside, defectReason: reason})
		}
	}
	seedGrades(t, client, seeds)

	// top=0 表示「不按调用方限制」，此时上限只由 DefectScanCap 决定。
	got, err := svc.QualityDistribution(context.Background(), start, end, 0)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if len(got.TopDefects) != service.DefectScanCap {
		t.Fatalf("len(TopDefects) = %d, want %d（应被 SQL 层上限截断）",
			len(got.TopDefects), service.DefectScanCap)
	}
	// 计数最高的那个原因必须在首位：证明排序发生在截断之前。
	if got.TopDefects[0].DefectReason != "缺陷-000" || got.TopDefects[0].Count != 5 {
		t.Errorf("TopDefects[0] = %+v, want {缺陷-000 5}（SQL 层按计数降序后截断）", got.TopDefects[0])
	}
	// 单调不增：结果是整体有序的，不是随机子集。
	for i := 1; i < len(got.TopDefects); i++ {
		if got.TopDefects[i].Count > got.TopDefects[i-1].Count {
			t.Fatalf("TopDefects 未按计数降序：索引 %d 为 %d，前一项为 %d",
				i, got.TopDefects[i].Count, got.TopDefects[i-1].Count)
		}
	}
}

// 平均分只统计已录入 final_score 的行，且样本数为 0 时返回 null 而非 0。
//
// 若把 NULL 当成 0 分拉低平均，或把「无数据」表达成 0 分，
// 「全员 0 分」与「还没人评分」在报表上就分不开了。
func TestReportService_QualityDistribution_AverageScore(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	s80, s90 := 80.0, 90.0
	seedGrades(t, client, []gradeSeed{
		{grade: "A", inspectedAt: inside, finalScore: &s80},
		{grade: "B", inspectedAt: inside, finalScore: &s90},
		{grade: "C", inspectedAt: inside}, // 未录入
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.ScoredCount != 2 {
		t.Errorf("scored_count = %d, want 2（未录入的行不计入样本）", got.ScoredCount)
	}
	if got.AverageScore == nil {
		t.Fatal("average_score = nil, want 85")
	}
	if *got.AverageScore != 85 {
		t.Errorf("average_score = %v, want 85", *got.AverageScore)
	}
}

// 区间内没有任何评分时 average_score 为 null、scored_count 为 0。
func TestReportService_QualityDistribution_AverageScoreEmpty(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{{grade: "A", inspectedAt: inside}})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.ScoredCount != 0 {
		t.Errorf("scored_count = %d, want 0", got.ScoredCount)
	}
	if got.AverageScore != nil {
		t.Errorf("average_score = %v, want null（无样本不得返回 0 分）", *got.AverageScore)
	}
}

// 五维度合格率按 inspection_details 的键展开，na 不进分母。
//
// na 表示「该项未检」，把它算进分母会让「没检」拉低合格率 ——
// 这是本指标最容易错的一处口径。
func TestReportService_QualityDistribution_ByDimension(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, inside, _, _ := reportWindow()

	seedGrades(t, client, []gradeSeed{
		{
			grade: "A", inspectedAt: inside,
			inspectionDetails: map[string]interface{}{
				"appearance": "pass", "broken_ends": "fail",
			},
		},
		{
			grade: "B", inspectedAt: inside,
			inspectionDetails: map[string]interface{}{
				"appearance": "pass", "broken_ends": "na",
			},
		},
		{
			grade: "C", inspectedAt: inside,
			inspectionDetails: map[string]interface{}{
				"appearance": "fail", "broken_ends": "pass",
			},
		},
		// 未录入质检项：不应出现在任何维度里
		{grade: "A", inspectedAt: inside},
	})

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	byDim := map[string]service.DimensionStat{}
	for _, d := range got.ByDimension {
		byDim[d.Dimension] = d
	}

	app, ok := byDim["appearance"]
	if !ok {
		t.Fatalf("缺少 appearance 维度: %+v", got.ByDimension)
	}
	if app.PassCount != 2 || app.FailCount != 1 || app.NACount != 0 {
		t.Errorf("appearance = %+v, want pass=2 fail=1 na=0", app)
	}
	// 2/(2+1) = 0.667
	if app.QualifiedRate != 0.667 {
		t.Errorf("appearance qualified_rate = %v, want 0.667", app.QualifiedRate)
	}

	be, ok := byDim["broken_ends"]
	if !ok {
		t.Fatalf("缺少 broken_ends 维度")
	}
	if be.PassCount != 1 || be.FailCount != 1 || be.NACount != 1 {
		t.Errorf("broken_ends = %+v, want pass=1 fail=1 na=1", be)
	}
	// na 不进分母：1/(1+1) = 0.5，而不是 1/3
	if be.QualifiedRate != 0.5 {
		t.Errorf("broken_ends qualified_rate = %v, want 0.5（na 不计入分母）", be.QualifiedRate)
	}
}

// 空区间下 by_dimension 是空切片而非 nil。
func TestReportService_QualityDistribution_ByDimensionEmpty(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewReportService(client)
	start, end, _, _, _ := reportWindow()

	got, err := svc.QualityDistribution(context.Background(), start, end, 10)
	if err != nil {
		t.Fatalf("QualityDistribution: %v", err)
	}

	if got.ByDimension == nil {
		t.Error("by_dimension = nil, want 空切片")
	}
	if len(got.ByDimension) != 0 {
		t.Errorf("by_dimension = %+v, want 空", got.ByDimension)
	}
}
