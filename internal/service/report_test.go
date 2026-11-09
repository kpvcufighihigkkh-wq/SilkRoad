package service_test

import (
	"context"
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
