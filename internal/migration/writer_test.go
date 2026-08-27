package migration

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func TestWriterFieldsDropsOldFieldAfterStop(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mig.db"))
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
	plan, err := CreatePlan(s, "split", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	split, err := AddStep(s, plan.ID, 0, model.StepSplit, "customer_name",
		[]string{"first_name", "last_name"}, true, false, "split")
	if err != nil {
		t.Fatal(err)
	}
	stop, err := AddStep(s, plan.ID, 1, model.StepStopDualWrite, "customer_name",
		nil, false, true, "stop")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(s, split.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(s, stop.ID, model.StepDualWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(s, stop.ID, model.StepBackfill); err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(s, stop.ID, model.StepFinalize); err != nil {
		t.Fatal(err)
	}
	steps, err := ListSteps(s, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields := WriterFields(v1, steps)
	set := map[string]bool{}
	for _, f := range fields {
		set[f] = true
	}
	if set["customer_name"] {
		t.Fatalf("stop_dual_write should drop customer_name, got %v", fields)
	}
	if !set["first_name"] || !set["last_name"] {
		t.Fatalf("writer should keep split fields, got %v", fields)
	}
}
