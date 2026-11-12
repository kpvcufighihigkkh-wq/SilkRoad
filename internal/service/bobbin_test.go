package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbingrade"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// 质检必须留下历史记录：同一丝饼质检两次 → 两条 BobbinGrade
func TestInspectBobbin_AppendsGradeHistory(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-INSPECT-1").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-INSPECT-1").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{Grade: "A"}); err != nil {
		t.Fatalf("first inspect: %v", err)
	}
	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{Grade: "B"}); err != nil {
		t.Fatalf("second inspect: %v（重检不得因唯一约束失败）", err)
	}

	n, err := client.BobbinGrade.Query().
		Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).
		Count(ctx)
	if err != nil {
		t.Fatalf("count grades: %v", err)
	}
	if n != 2 {
		t.Errorf("质检记录数 = %d, want 2（应保留历史，不覆盖）", n)
	}
}

// 现场陈述的合格性必须透传，不得被丢弃。
//
// is_qualified 是三态：NULL=未陈述 / false / true（见 schema/bobbingrade.go 的
// Optional().Nillable()）。若 InspectBobbin 不设该列，评级 C/D 的质检也会落成
// NULL —— 一张只增不改的「历史事实」表里丢掉现场结论，且永远无法补回。
// 这里用「不合格」的显式入参验证透传：删掉 SetNillableIsQualified，测试必红。
func TestInspectBobbin_PassesThroughIsQualified(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-INSPECT-QUAL").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-INSPECT-QUAL").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	notQualified := false
	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{
		Grade:       "C",
		IsQualified: &notQualified,
	}); err != nil {
		t.Fatalf("inspect: %v", err)
	}

	got, err := client.BobbinGrade.Query().
		Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).
		Only(ctx)
	if err != nil {
		t.Fatalf("query grade: %v", err)
	}
	if got.IsQualified == nil {
		t.Fatal("IsQualified = NULL, want 非 nil（入参未透传，现场陈述被丢弃）")
	}
	if *got.IsQualified {
		t.Error("IsQualified = true, want false（入参未透传，落成了相反结论）")
	}
}

// 不存在的丝锭 UUID 必须映射成 40001（业务入参错误），而不是冒成 500。
//
// UpdateBobbinStatus / MarkBobbinPrinted 此前直接用 UpdateOneID(...).Exec()，
// ent 的 "not found" 错误没有任何判别类型，冒到 HTTP 层就是 500 —— 告诉运维
// 「同步服务坏了」，而事实只是「id 写错了」。同一文件里的 WeighBobbin /
// InspectBobbin 早已按 40001 处理；Task 9 新暴露的
// PUT /v1/bobbins/:id/status 与 POST /v1/bobbins/:id/print 必须与它们一致。
func TestBobbinWrite_NonexistentIDMapsTo40001(t *testing.T) {
	ctx := context.Background()
	missing := uuid.New().String()

	cases := []struct {
		name string
		call func(t *testing.T, svc *service.BobbinService) error
	}{
		{
			name: "UpdateBobbinStatus",
			call: func(t *testing.T, svc *service.BobbinService) error {
				t.Helper()
				return svc.UpdateBobbinStatus(ctx, missing, "completed")
			},
		},
		{
			name: "MarkBobbinPrinted",
			call: func(t *testing.T, svc *service.BobbinService) error {
				t.Helper()
				return svc.MarkBobbinPrinted(ctx, missing)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := newTestClient(t)
			svc := service.NewBobbinService(client)

			err := tc.call(t, svc)
			if err == nil {
				t.Fatal("不存在的丝锭应返回错误")
			}
			if code := serviceErrorCode(t, err); code != 40001 {
				t.Errorf("ServiceError.Code = %d, want 40001（丝锭不存在；500 会让运维以为同步服务故障）", code)
			}
		})
	}
}

// 未知的 status 枚举值必须映射成 10001，与路由的 @Failure 400 承诺一致。
//
// 生成代码在 save 前才跑 bobbin.StatusValidator，其错误没有可判别的类型，
// 从前会冒成 500。
func TestUpdateBobbinStatus_InvalidStatusMapsTo10001(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-STATUS-BAD").
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}
	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber("BOBBIN-STATUS-BAD").
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	err = svc.UpdateBobbinStatus(ctx, bobbinRow.ID.String(), "not-a-status")
	if err == nil {
		t.Fatal("非法 status 应返回错误")
	}
	if code := serviceErrorCode(t, err); code != 10001 {
		t.Errorf("ServiceError.Code = %d, want 10001（未知枚举值属于入参错误）", code)
	}
}

// serviceErrorCode 取出 *service.ServiceError 的 Code；非该类型时返回 0。
func serviceErrorCode(t *testing.T, err error) int {
	t.Helper()
	var svcErr *service.ServiceError
	if !errors.As(err, &svcErr) {
		t.Fatalf("错误类型 = %T (%v), want *service.ServiceError", err, err)
	}
	return svcErr.Code
}

// 五项等级分、综合分、质检项明细必须落到 bobbin_grade 上。
//
// 这三列此前在 schema 里存在但全仓库无人写入，导致报表的「五维度合格率」
// 「平均分」无数据源（报表 spec §6 缺口 A）。本用例钉住「写入并落库」。
func TestInspectBobbin_PersistsScoresAndDetails(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)
	bobbinRow := seedInspectBobbin(t, client, "BOBBIN-SCORE-1")

	sorting, weight, final, vision, knitting := 92.0, 88.0, 90.0, 85.0, 80.0
	finalScore := 87.5

	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{
		Grade:      "A",
		Scores:     &service.InspectionScores{Sorting: &sorting, Weight: &weight, Final: &final, Vision: &vision, Knitting: &knitting},
		FinalScore: &finalScore,
		InspectionDetails: map[string]string{
			"appearance":   "pass",
			"weight_check": "pass",
			"broken_ends":  "fail",
		},
	}); err != nil {
		t.Fatalf("inspect: %v", err)
	}

	got, err := client.BobbinGrade.Query().
		Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).
		Only(ctx)
	if err != nil {
		t.Fatalf("query grade: %v", err)
	}

	if len(got.Scores) != 5 {
		t.Fatalf("scores 项数 = %d, want 5: %+v", len(got.Scores), got.Scores)
	}
	// JSON 往返后数字是 float64；逐项比对键名与值。
	wantScores := map[string]float64{
		"sorting": 92, "weight": 88, "final": 90, "vision": 85, "knitting": 80,
	}
	for k, want := range wantScores {
		v, ok := got.Scores[k].(float64)
		if !ok {
			t.Errorf("scores[%q] 缺失或类型错误: %v", k, got.Scores[k])
			continue
		}
		if v != want {
			t.Errorf("scores[%q] = %v, want %v", k, v, want)
		}
	}

	if got.FinalScore != 87.5 {
		t.Errorf("final_score = %v, want 87.5", got.FinalScore)
	}

	if len(got.InspectionDetails) != 3 {
		t.Fatalf("inspection_details 项数 = %d, want 3: %+v", len(got.InspectionDetails), got.InspectionDetails)
	}
	if v := got.InspectionDetails["broken_ends"]; v != "fail" {
		t.Errorf("inspection_details[broken_ends] = %v, want fail", v)
	}
}

// 未提供评分时三列必须保持 NULL，而不是落成空对象或 0 分。
//
// 「未录入」与「录入了 0 分」在报表里是两种完全不同的样本：
// 前者应从分母剔除，后者是真实的不合格。
func TestInspectBobbin_LeavesScoresNullWhenOmitted(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)
	bobbinRow := seedInspectBobbin(t, client, "BOBBIN-SCORE-NULL")

	if err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{Grade: "B"}); err != nil {
		t.Fatalf("inspect: %v", err)
	}

	got, err := client.BobbinGrade.Query().
		Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).
		Only(ctx)
	if err != nil {
		t.Fatalf("query grade: %v", err)
	}
	if len(got.Scores) != 0 {
		t.Errorf("scores = %+v, want 空（未录入应保持 NULL）", got.Scores)
	}
	if len(got.InspectionDetails) != 0 {
		t.Errorf("inspection_details = %+v, want 空", got.InspectionDetails)
	}
	if got.FinalScore != 0 {
		t.Errorf("final_score = %v, want 0（NULL）", got.FinalScore)
	}
}

// 五项分必须齐全：缺一项的分数集无法算「五维度合格率」，
// 而缺项会被当成 0 分。
func TestInspectBobbin_RejectsIncompleteScores(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)
	bobbinRow := seedInspectBobbin(t, client, "BOBBIN-SCORE-INCOMPLETE")

	sorting, weight := 90.0, 88.0
	err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{
		Grade:  "A",
		Scores: &service.InspectionScores{Sorting: &sorting, Weight: &weight},
	})
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		t.Fatalf("缺项应返回 ServiceError, got %T: %v", err, err)
	}
	if svcErr.Code != 10001 {
		t.Errorf("error code = %d, want 10001", svcErr.Code)
	}

	// 校验失败不得留下半条记录，也不得改动丝饼等级。
	if n, _ := client.BobbinGrade.Query().Where(bobbingrade.BobbinIDEQ(bobbinRow.ID)).Count(ctx); n != 0 {
		t.Errorf("校验失败仍写入了 %d 条质检记录", n)
	}
	row, err := client.Bobbin.Get(ctx, bobbinRow.ID)
	if err != nil {
		t.Fatalf("reload bobbin: %v", err)
	}
	if row.Grade != "" {
		t.Errorf("bobbin.grade = %q, want 空（校验失败不得改等级）", row.Grade)
	}
}

// 质检项取值限定为 pass/fail/na：自由文本会让「合格 / OK / 通过」
// 在报表里变成三个不同的分组。
func TestInspectBobbin_RejectsUnknownVerdict(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	svc := service.NewBobbinService(client)
	bobbinRow := seedInspectBobbin(t, client, "BOBBIN-VERDICT")

	err := svc.InspectBobbin(ctx, bobbinRow.ID.String(), &service.InspectBobbinRequest{
		Grade:             "A",
		InspectionDetails: map[string]string{"appearance": "合格"},
	})
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		t.Fatalf("非法取值应返回 ServiceError, got %T: %v", err, err)
	}
	if svcErr.Code != 10001 {
		t.Errorf("error code = %d, want 10001", svcErr.Code)
	}
}

// seedInspectBobbin 造一个可质检的丝饼。
func seedInspectBobbin(t *testing.T, client *ent.Client, number string) *ent.Bobbin {
	t.Helper()
	ctx := context.Background()

	lotRow, err := client.Lot.Create().
		SetLotNumber("LOT-" + number).
		SetProductType(lot.ProductType("FDY")).
		SetPlannedQuantity(10).
		Save(ctx)
	if err != nil {
		t.Fatalf("create lot: %v", err)
	}

	bobbinRow, err := client.Bobbin.Create().
		SetBobbinNumber(number).
		SetLotID(lotRow.ID).
		SetSpinningPosition(1).
		SetGrossWeight(1.5).
		SetNetWeight(1.2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create bobbin: %v", err)
	}

	return bobbinRow
}
