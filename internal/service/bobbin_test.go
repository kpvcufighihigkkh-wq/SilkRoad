package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

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

// 现场陈述的合格性必须透传，不得由 schema 默认值顶替。
//
// BobbinGrade.is_qualified 在 schema 中 Default(true)。若 InspectBobbin 不设该列，
// 评级 C/D 的质检也会落成 is_qualified=true —— 一张只增不改的「历史事实」表里
// 写入与事实相反的结论，且永远无法纠正。
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
	if got.IsQualified {
		t.Error("IsQualified = true, want false（入参未透传，落成了 schema 默认值）")
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
