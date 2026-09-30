package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/yourusername/igh-silkroad/internal/service"
)

func TestEdgeService_CreateEdge(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	resp, err := svc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-001",
		EdgeName:  "一号边端",
		IPAddress: "192.168.2.84",
	})
	if err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	if resp.EdgeCode != "edge-001" {
		t.Errorf("EdgeCode = %q, want %q", resp.EdgeCode, "edge-001")
	}
	if resp.IPAddress != "192.168.2.84" {
		t.Errorf("IPAddress = %q, want %q", resp.IPAddress, "192.168.2.84")
	}
	if resp.ID == "" {
		t.Error("ID 不应为空")
	}
}

func TestEdgeService_GetEdgeByCode_NotRegistered(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	_, err := svc.GetEdgeByCode(ctx, "never-registered")
	if !errors.Is(err, service.ErrEdgeNotRegistered) {
		t.Errorf("err = %v, want ErrEdgeNotRegistered", err)
	}
}

func TestEdgeService_GetEdgeByCode_Found(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	if _, err := svc.CreateEdge(ctx, &service.CreateEdgeRequest{
		EdgeCode:  "edge-002",
		EdgeName:  "二号边端",
		IPAddress: "192.168.2.85",
	}); err != nil {
		t.Fatalf("CreateEdge failed: %v", err)
	}

	found, err := svc.GetEdgeByCode(ctx, "edge-002")
	if err != nil {
		t.Fatalf("GetEdgeByCode failed: %v", err)
	}
	if found.IPAddress != "192.168.2.85" {
		t.Errorf("IPAddress = %q, want %q", found.IPAddress, "192.168.2.85")
	}
}

func TestEdgeService_CreateEdge_DuplicateCodeRejected(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	svc := service.NewEdgeService(client)

	req := &service.CreateEdgeRequest{
		EdgeCode:  "edge-dup",
		EdgeName:  "重复",
		IPAddress: "192.168.2.86",
	}
	if _, err := svc.CreateEdge(ctx, req); err != nil {
		t.Fatalf("首次 CreateEdge 失败: %v", err)
	}

	if _, err := svc.CreateEdge(ctx, req); err == nil {
		t.Error("重复 edge_code 应当被拒绝（edge_code 有 Unique 约束）")
	}
}
