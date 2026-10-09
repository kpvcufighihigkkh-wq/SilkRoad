package service_test

import (
	"context"
	"testing"

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
