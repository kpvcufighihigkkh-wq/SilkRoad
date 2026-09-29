package service_test

import (
	"context"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func TestLotService_CreateLot_WithEdgeID(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewLotService(client)

	resp, err := svc.CreateLot(ctx, &service.CreateLotRequest{
		LotNumber:       "LOT-TEST-001",
		PLCLotNumber:    "PLC-8823",
		ProductType:     "FDY",
		ProductSpec:     "150D/48F",
		PlannedQuantity: 4800,
	})
	if err != nil {
		t.Fatalf("CreateLot failed: %v", err)
	}

	if resp.LotNumber != "LOT-TEST-001" {
		t.Errorf("LotNumber = %q, want %q", resp.LotNumber, "LOT-TEST-001")
	}
	if resp.PLCLotNumber != "PLC-8823" {
		t.Errorf("PLCLotNumber = %q, want %q", resp.PLCLotNumber, "PLC-8823")
	}
	if resp.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", resp.Status, "in_progress")
	}
	if resp.PlannedQuantity != 4800 {
		t.Errorf("PlannedQuantity = %d, want 4800", resp.PlannedQuantity)
	}
	if resp.ActualQuantity != 0 {
		t.Errorf("ActualQuantity = %d, want 0", resp.ActualQuantity)
	}
	if resp.Progress != 0 {
		t.Errorf("Progress = %v, want 0", resp.Progress)
	}
}

func TestLotService_CreateLot_WithEdge(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	edge, err := client.Edge.Create().
		SetEdgeCode("edge-test-01").
		SetEdgeName("测试边端").
		Save(ctx)
	if err != nil {
		t.Fatalf("failed creating edge: %v", err)
	}

	svc := service.NewLotService(client)

	resp, err := svc.CreateLot(ctx, &service.CreateLotRequest{
		LotNumber:       "LOT-TEST-002",
		EdgeID:          edge.ID.String(),
		ProductType:     "POY",
		PlannedQuantity: 1200,
	})
	if err != nil {
		t.Fatalf("CreateLot failed: %v", err)
	}

	if resp.EdgeID != edge.ID.String() {
		t.Errorf("EdgeID = %q, want %q", resp.EdgeID, edge.ID.String())
	}
}
