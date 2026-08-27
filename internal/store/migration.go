package store

import (
	"database/sql"
	"fmt"

	"task280-regevocompat/internal/model"
)

// SavePlan 持久化演进计划。
func (s *Store) SavePlan(p *model.MigrationPlan) error {
	_, err := s.db.Exec(
		`INSERT INTO migration_plans (id, name, state, baseline_version_id, target_version_id, sealed_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name, state=excluded.state,
		   baseline_version_id=excluded.baseline_version_id, target_version_id=excluded.target_version_id,
		   sealed_at=excluded.sealed_at`,
		p.ID, p.Name, p.State, p.BaselineVersionID, p.TargetVersionID, p.SealedAt, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	return nil
}

// GetPlan 按 ID 读取演进计划。
func (s *Store) GetPlan(id model.PlanID) (*model.MigrationPlan, error) {
	row := s.db.QueryRow(
		`SELECT id, name, state, baseline_version_id, target_version_id, sealed_at, created_at
		 FROM migration_plans WHERE id = ?`, id)
	return scanPlan(row)
}

// ListPlans 列出全部演进计划。
func (s *Store) ListPlans() ([]*model.MigrationPlan, error) {
	rows, err := s.db.Query(
		`SELECT id, name, state, baseline_version_id, target_version_id, sealed_at, created_at
		 FROM migration_plans ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	defer rows.Close()
	var out []*model.MigrationPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func scanPlan(r interface {
	Scan(...interface{}) error
}) (*model.MigrationPlan, error) {
	var id, name, state, base, target string
	var sealed, created int64
	if err := r.Scan(&id, &name, &state, &base, &target, &sealed, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan plan: %w", err)
	}
	return &model.MigrationPlan{
		ID:                model.PlanID(id),
		Name:              name,
		State:             state,
		BaselineVersionID: model.SchemaVersionID(base),
		TargetVersionID:   model.SchemaVersionID(target),
		SealedAt:          sealed,
		CreatedAt:         created,
	}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SaveStep 持久化迁移步骤。
func (s *Store) SaveStep(st *model.MigrationStep) error {
	_, err := s.db.Exec(
		`INSERT INTO migration_steps
		 (id, plan_id, ordinal, kind, description, from_field, to_fields_json, dual_write, stop_dual_write, state, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET ordinal=excluded.ordinal, kind=excluded.kind,
		   description=excluded.description, from_field=excluded.from_field, to_fields_json=excluded.to_fields_json,
		   dual_write=excluded.dual_write, stop_dual_write=excluded.stop_dual_write, state=excluded.state`,
		st.ID, st.PlanID, st.Ordinal, string(st.Kind), st.Description, st.FromField,
		st.ToFieldsJSON, boolToInt(st.DualWrite), boolToInt(st.StopDualWrite), st.State, st.CreatedAt)
	if err != nil {
		return fmt.Errorf("save step: %w", err)
	}
	return nil
}

// GetStep 按 ID 读取迁移步骤。
func (s *Store) GetStep(id model.StepID) (*model.MigrationStep, error) {
	row := s.db.QueryRow(
		`SELECT id, plan_id, ordinal, kind, description, from_field, to_fields_json, dual_write, stop_dual_write, state, created_at
		 FROM migration_steps WHERE id = ?`, id)
	return scanStep(row)
}

// ListStepsByPlan 按计划列出迁移步骤（按 ordinal）。
func (s *Store) ListStepsByPlan(planID model.PlanID) ([]*model.MigrationStep, error) {
	rows, err := s.db.Query(
		`SELECT id, plan_id, ordinal, kind, description, from_field, to_fields_json, dual_write, stop_dual_write, state, created_at
		 FROM migration_steps WHERE plan_id = ? ORDER BY ordinal`, planID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer rows.Close()
	var out []*model.MigrationStep
	for rows.Next() {
		st, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func scanStep(r interface {
	Scan(...interface{}) error
}) (*model.MigrationStep, error) {
	var id, plan, kind, desc, from, toJSON, state string
	var ordinal, dw, sdw, created int
	if err := r.Scan(&id, &plan, &ordinal, &kind, &desc, &from, &toJSON, &dw, &sdw, &state, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan step: %w", err)
	}
	return &model.MigrationStep{
		ID:            model.StepID(id),
		PlanID:        model.PlanID(plan),
		Ordinal:       ordinal,
		Kind:          model.StepKind(kind),
		Description:   desc,
		FromField:     from,
		ToFieldsJSON:  toJSON,
		DualWrite:     dw != 0,
		StopDualWrite: sdw != 0,
		State:         state,
		CreatedAt:     int64(created),
	}, nil
}
