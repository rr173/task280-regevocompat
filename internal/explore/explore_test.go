package explore

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/compat"
	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/region"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func TestExploreResolvesWithAdapterWindow(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "ex.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
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
	if _, err := region.Register(s, "west", v1.ID); err != nil {
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
	if _, err := compat.DeclareWindow(s, plan.ID, v1.ID, v2.ID, model.RuleAdapter,
		`[{"output":"customer_name","inputs":["first_name","last_name"],"op":"concat_space"}]`); err != nil {
		t.Fatal(err)
	}
	conflicts, err := Explore(s, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range conflicts {
		if c.Field == "customer_name" && !c.Resolved {
			t.Fatalf("adapter window should resolve customer_name: %+v", c)
		}
	}
}
