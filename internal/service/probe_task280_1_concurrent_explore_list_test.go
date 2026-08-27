package service

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
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

func TestConcurrentExploreAndListIsolation(t *testing.T) {
	svc := newProbeService(t)
	planID, _, _, _ := seedProbePlan(t, svc, "c1")
	if _, err := svc.VerifyAndExplore(context.Background(), planID); err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				if _, err := svc.RunExplore(context.Background(), planID); err != nil {
					errCh <- fmt.Errorf("explore %d: %w", i, err)
				}
				return
			}
			cs, err := svc.Conflicts(planID)
			if err != nil {
				errCh <- fmt.Errorf("list %d: %w", i, err)
				return
			}
			if len(cs) == 0 {
				errCh <- fmt.Errorf("list %d saw empty conflicts during explore", i)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
}
