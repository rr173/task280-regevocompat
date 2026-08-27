package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func newProbeService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st)
}

func seedProbePlan(t *testing.T, svc *Service, suffix string) (model.PlanID, model.RegionID, model.SchemaVersionID, model.SchemaVersionID) {
	t.Helper()
	s := svc.Store()
	v1, err := schemaver.Register(s, "v1-"+suffix, []model.Field{{Name: "customer_name", Type: model.FieldString}})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := schemaver.Register(s, "v2-"+suffix, []model.Field{
		{Name: "first_name", Type: model.FieldString},
		{Name: "last_name", Type: model.FieldString},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.Register(s, "east-"+suffix, v2.ID); err != nil {
		t.Fatal(err)
	}
	west, err := region.Register(s, "west-"+suffix, v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := region.MarkLagging(s, west.ID); err != nil {
		t.Fatal(err)
	}
	plan, err := migration.CreatePlan(s, "split-"+suffix, v1.ID, v2.ID)
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
	return plan.ID, west.ID, v1.ID, v2.ID
}

func snapshotUnresolved(t *testing.T, raw string) int {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	v, ok := payload["unresolved"]
	if !ok {
		t.Fatalf("missing unresolved: %s", raw)
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		t.Fatalf("unresolved type %T", v)
	}
	return -1
}

func TestExploreHonorsCanceledContext(t *testing.T) {
	svc := newProbeService(t)
	planID, _, _, _ := seedProbePlan(t, svc, "ctx")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := svc.RunExplore(ctx, planID)
	if err == nil {
		t.Fatal("canceled explore must return an error")
	}
	cs, err := svc.Conflicts(planID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 0 {
		t.Fatalf("canceled explore persisted %d conflicts", len(cs))
	}
	plan, err := svc.GetPlan(planID)
	if err != nil {
		t.Fatal(err)
	}
	if plan.State == model.PlanConflicted {
		t.Fatal("canceled explore must not mark plan conflicted")
	}
}
