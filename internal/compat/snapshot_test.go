package compat

import (
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/migration"
	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/schemaver"
	"task280-regevocompat/internal/store"
)

func TestPublishSupersedesPreviousSnapshot(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "snap.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	v1, err := schemaver.Register(s, "v1", []model.Field{{Name: "a", Type: model.FieldString}})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := schemaver.Register(s, "v2", []model.Field{{Name: "b", Type: model.FieldString}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := migration.CreatePlan(s, "p", v1.ID, v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := PublishSnapshot(s, plan.ID, `{"n":1}`)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PublishSnapshot(s, plan.ID, `{"n":2}`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSnapshot(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.SnapSuperseded || got.SupersededBy != second.ID {
		t.Fatalf("first snapshot state=%s by=%s", got.State, got.SupersededBy)
	}
	if second.State != model.SnapPublished {
		t.Fatalf("second state=%s", second.State)
	}
}
