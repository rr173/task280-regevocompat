package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func seedSplitPlan(t *testing.T, svc *Service) (model.PlanID, model.RegionID) {
	t.Helper()
	s := svc.Store()
	v1, err := schemaver.Register(s, "v1", []model.Field{{Name: "customer_name", Type: model.FieldString}})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := schemaver.Register(s, "v2", []model.Field{
		{Name: "first_name", Type: model.FieldString},
		{Name: "last_name", Type: model.FieldString},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.Register(s, "east", v2.ID); err != nil {
		t.Fatal(err)
	}
	west, err := region.Register(s, "west", v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.MarkLagging(s, west.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := migration.CreatePlan(s, "split", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	split, err := migration.AddStep(s, plan.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "split")
	if err != nil {
		t.Fatal(err)
	}
	stop, err := migration.AddStep(s, plan.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "stop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, split.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepBackfill); err != nil {
		t.Fatal(err)
	}
	if _, err := migration.Advance(s, stop.ID, model.StepFinalize); err != nil {
		t.Fatal(err)
	}
	return plan.ID, west.ID
}

func TestVerifyAndExploreFindsWestConflict(t *testing.T) {
	svc := newTestService(t)
	planID, westID := seedSplitPlan(t, svc)
	conflicts, err := svc.VerifyAndExplore(context.Background(), planID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range conflicts {
		if c.RegionID == westID && c.Field == "customer_name" && !c.Resolved {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unresolved west customer_name conflict, got %+v", conflicts)
	}
}

// TestRunExploreCanceledDropsConflictsAndKeepsState 验证：探索请求被取消时，
// 已算出的冲突不得落库，计划状态也不得被翻转为 conflicted。
// 前置：先把计划置为 verifying（conflicted 的合法前驱态），再用预取消的 ctx 跑探索，
// 这样 explore.Explore 会跑完返回真实冲突（含 5ms sleep），但取消闸门必须拦在落库之前。
func TestRunExploreCanceledDropsConflictsAndKeepsState(t *testing.T) {
	svc := newTestService(t)
	planID, _ := seedSplitPlan(t, svc)
	s := svc.Store()
	if _, err := migration.Verify(s, planID); err != nil {
		t.Fatalf("verify plan: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预取消：探索跑完后、落库前必须被拦下

	conflicts, err := svc.RunExplore(ctx, planID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v (conflicts=%d)", err, len(conflicts))
	}

	// 不应落下任何冲突路径（半成品）。
	got, err := svc.Conflicts(planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("canceled explore must not persist conflicts, got %d", len(got))
	}
	// 计划应停留在 verifying，未被翻转为 conflicted。
	p, err := s.GetPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if p.State != model.PlanVerifying {
		t.Fatalf("canceled explore must not flip plan state, got %s", p.State)
	}
}
