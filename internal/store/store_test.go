package store

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/model"
)

func TestPlanRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := &model.MigrationPlan{
		ID:                "pl_test",
		Name:              "roundtrip",
		State:             model.PlanDraft,
		BaselineVersionID: "sv_a",
		TargetVersionID:   "sv_b",
		CreatedAt:         1,
	}
	if err := s.SavePlan(p); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	got, err := s2.GetPlan(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "roundtrip" || got.State != model.PlanDraft {
		t.Fatalf("got %+v", got)
	}
}
