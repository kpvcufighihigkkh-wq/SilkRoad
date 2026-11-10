package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/bobbin"
	"github.com/yourusername/igh-silkroad/internal/database/ent/doffing"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// seedBobbinsForLot 在指定批次下造 n 个丝锭，返回其 ID。
func seedBobbinsForLot(t *testing.T, client *ent.Client, lotID uuid.UUID, n int) []uuid.UUID {
	t.Helper()

	out := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		row, err := client.Bobbin.Create().
			SetBobbinNumber("BOB-" + uuid.NewString()[:8]).
			SetLotID(lotID).
			SetSpinningPosition(i + 1).
			SetGrossWeight(5.5).
			SetNetWeight(5.1).
			SetStatus(bobbin.StatusCompleted).
			Save(context.Background())
		if err != nil {
			t.Fatalf("seed bobbin #%d: %v", i, err)
		}
		out = append(out, row.ID)
	}

	return out
}

func TestGradeService_ListGrades_ByLotID(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewGradeService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "GRADE-LOT-A")
	lotB := seedLot(t, client, "GRADE-LOT-B")
	bobbinsA := seedBobbinsForLot(t, client, lotA, 2)
	bobbinsB := seedBobbinsForLot(t, client, lotB, 1)

	now := time.Now().UTC()
	for _, b := range append(bobbinsA, bobbinsB...) {
		if _, err := client.BobbinGrade.Create().
			SetBobbinID(b).
			SetGrade("A").
			SetInspectedAt(now).
			Save(ctx); err != nil {
			t.Fatalf("seed grade: %v", err)
		}
	}

	got, total, err := svc.ListGrades(ctx, 1, 20, service.GradeListFilter{LotID: lotA.String()})
	if err != nil {
		t.Fatalf("ListGrades: %v", err)
	}

	// 批次 A 下 2 个丝锭各有 1 条记录；批次 B 的不应出现。
	if total != 2 || len(got) != 2 {
		t.Fatalf("total=%d len=%d, want 2/2（按批次过滤）", total, len(got))
	}
	for _, g := range got {
		if g.BobbinID != bobbinsA[0].String() && g.BobbinID != bobbinsA[1].String() {
			t.Errorf("返回了批次 B 的丝锭 %s", g.BobbinID)
		}
	}
}

// 三态过滤必须能分别命中 true / false / NULL —— 这是本服务最容易写错的点。
func TestGradeService_ListGrades_IsQualifiedTriState(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewGradeService(client)
	ctx := context.Background()

	now := time.Now().UTC()
	truthy, falsy := true, false
	seeds := []*bool{&truthy, &truthy, &falsy, nil, nil}
	for _, q := range seeds {
		b := client.BobbinGrade.Create().
			SetBobbinID(uuid.New()).
			SetGrade("B").
			SetInspectedAt(now)
		if q != nil {
			b.SetNillableIsQualified(q)
		}
		if _, err := b.Save(ctx); err != nil {
			t.Fatalf("seed grade: %v", err)
		}
	}

	cases := []struct {
		filter string
		want   int
	}{
		{"true", 2},
		{"false", 1},
		{"unspecified", 2},
		{"", 5},
	}

	for _, tc := range cases {
		t.Run("is_qualified="+tc.filter, func(t *testing.T) {
			got, total, err := svc.ListGrades(ctx, 1, 20, service.GradeListFilter{IsQualified: tc.filter})
			if err != nil {
				t.Fatalf("ListGrades: %v", err)
			}
			if total != tc.want || len(got) != tc.want {
				t.Errorf("total=%d len=%d, want %d", total, len(got), tc.want)
			}
		})
	}
}

// 未知取值必须报参数错误，而不是静默返回全部记录。
func TestGradeService_ListGrades_RejectsUnknownIsQualified(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewGradeService(client)

	_, _, err := svc.ListGrades(context.Background(), 1, 20, service.GradeListFilter{IsQualified: "yes"})
	if err == nil {
		t.Fatal("want error for is_qualified=yes")
	}
	if _, ok := err.(*service.ServiceError); !ok {
		t.Errorf("want *ServiceError, got %T", err)
	}
}

// 同一丝饼复检会留下多条记录，列表按记录返回，不去重。
func TestGradeService_ListGrades_KeepsRepeatInspections(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewGradeService(client)
	ctx := context.Background()

	bobbinID := uuid.New()
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if _, err := client.BobbinGrade.Create().
			SetBobbinID(bobbinID).
			SetGrade("B").
			SetInspectedAt(now.Add(time.Duration(i) * time.Minute)).
			Save(ctx); err != nil {
			t.Fatalf("seed grade: %v", err)
		}
	}

	got, total, err := svc.ListGrades(ctx, 1, 20, service.GradeListFilter{BobbinID: bobbinID.String()})
	if err != nil {
		t.Fatalf("ListGrades: %v", err)
	}
	if total != 3 || len(got) != 3 {
		t.Errorf("total=%d len=%d, want 3（复检各计一条）", total, len(got))
	}
	// 按质检时间倒序：最新的在最前
	if !got[0].InspectedAt.After(got[1].InspectedAt) {
		t.Error("未按 inspected_at 倒序")
	}
}

func TestGradeService_ListGrades_NullInspectorStaysNull(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewGradeService(client)
	ctx := context.Background()

	if _, err := client.BobbinGrade.Create().
		SetBobbinID(uuid.New()).
		SetGrade("A").
		SetInspectedAt(time.Now().UTC()).
		Save(ctx); err != nil {
		t.Fatalf("seed grade: %v", err)
	}

	got, _, err := svc.ListGrades(ctx, 1, 20, service.GradeListFilter{})
	if err != nil {
		t.Fatalf("ListGrades: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d, want 1", len(got))
	}
	if got[0].InspectorID != nil {
		t.Errorf("InspectorID = %v, want nil（未记录检测员不得变成零值 UUID）", *got[0].InspectorID)
	}
}

func TestDoffingService_ListDoffings_ByLotID(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewDoffingService(client)
	ctx := context.Background()

	lotA := seedLot(t, client, "DOFF-LOT-A")
	lotB := seedLot(t, client, "DOFF-LOT-B")
	lineID := uuid.New()
	now := time.Now().UTC()

	for i, lotID := range []uuid.UUID{lotA, lotA, lotB} {
		if _, err := client.Doffing.Create().
			SetLotID(lotID).
			SetSpinningLineID(lineID).
			SetSpinningPosition(i + 1).
			SetStatus(doffing.StatusConfirmed).
			SetDoffingTime(now.Add(time.Duration(i) * time.Minute)).
			Save(ctx); err != nil {
			t.Fatalf("seed doffing: %v", err)
		}
	}

	got, total, err := svc.ListDoffings(ctx, 1, 20, service.DoffingListFilter{LotID: lotA.String()})
	if err != nil {
		t.Fatalf("ListDoffings: %v", err)
	}
	if total != 2 || len(got) != 2 {
		t.Errorf("total=%d len=%d, want 2（按批次过滤）", total, len(got))
	}
}

func TestDoffingService_ListDoffings_ByOperatorAndStatus(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewDoffingService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "DOFF-LOT-C")
	lineID := uuid.New()
	opA, opB := uuid.New(), uuid.New()
	now := time.Now().UTC()

	seeds := []struct {
		op     *uuid.UUID
		status doffing.Status
	}{
		{&opA, doffing.StatusConfirmed},
		{&opA, doffing.StatusCancelled},
		{&opB, doffing.StatusConfirmed},
		{nil, doffing.StatusPending},
	}

	for i, s := range seeds {
		b := client.Doffing.Create().
			SetLotID(lotID).
			SetSpinningLineID(lineID).
			SetSpinningPosition(i + 1).
			SetStatus(s.status).
			SetDoffingTime(now.Add(time.Duration(i) * time.Minute))
		if s.op != nil {
			b.SetOperatorID(*s.op)
		}
		if _, err := b.Save(ctx); err != nil {
			t.Fatalf("seed doffing #%d: %v", i, err)
		}
	}

	byOp, total, err := svc.ListDoffings(ctx, 1, 20, service.DoffingListFilter{OperatorID: opA.String()})
	if err != nil {
		t.Fatalf("ListDoffings by operator: %v", err)
	}
	if total != 2 || len(byOp) != 2 {
		t.Errorf("operator 过滤 total=%d, want 2", total)
	}

	byStatus, total, err := svc.ListDoffings(ctx, 1, 20, service.DoffingListFilter{Status: "confirmed"})
	if err != nil {
		t.Fatalf("ListDoffings by status: %v", err)
	}
	if total != 2 || len(byStatus) != 2 {
		t.Errorf("status=confirmed total=%d, want 2", total)
	}
}

// 空条件返回全部，按落纱时间倒序。
func TestDoffingService_ListDoffings_OrderAndEmptyFilter(t *testing.T) {
	client := newTestClient(t)
	svc := service.NewDoffingService(client)
	ctx := context.Background()

	lotID := seedLot(t, client, "DOFF-LOT-D")
	lineID := uuid.New()
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		if _, err := client.Doffing.Create().
			SetLotID(lotID).
			SetSpinningLineID(lineID).
			SetSpinningPosition(i + 1).
			SetStatus(doffing.StatusConfirmed).
			SetDoffingTime(now.Add(time.Duration(i) * time.Minute)).
			Save(ctx); err != nil {
			t.Fatalf("seed doffing: %v", err)
		}
	}

	got, total, err := svc.ListDoffings(ctx, 1, 20, service.DoffingListFilter{})
	if err != nil {
		t.Fatalf("ListDoffings: %v", err)
	}
	if total != 3 {
		t.Fatalf("total=%d, want 3", total)
	}
	if !got[0].DoffingTime.After(got[1].DoffingTime) {
		t.Error("未按 doffing_time 倒序")
	}
}

// lot 表在本文件里被引用，避免 import 被误删导致的隐性依赖。
var _ = lot.StatusInProgress
