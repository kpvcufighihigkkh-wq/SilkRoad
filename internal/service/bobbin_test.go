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
