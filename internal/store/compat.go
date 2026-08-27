package store

import (
	"database/sql"
	"fmt"

	"task280-regevocompat/internal/model"
)

// SaveWindow 持久化兼容窗口。
func (s *Store) SaveWindow(w *model.CompatWindow) error {
	_, err := s.db.Exec(
		`INSERT INTO compat_windows
		 (id, plan_id, reader_version_id, writer_version_id, rule_type, rule_payload, state, valid_from, valid_to, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET reader_version_id=excluded.reader_version_id,
		   writer_version_id=excluded.writer_version_id, rule_type=excluded.rule_type,
		   rule_payload=excluded.rule_payload, state=excluded.state,
		   valid_from=excluded.valid_from, valid_to=excluded.valid_to`,
		w.ID, w.PlanID, w.ReaderVersionID, w.WriterVersionID, string(w.RuleType),
		w.RulePayload, w.State, w.ValidFrom, w.ValidTo, w.CreatedAt)
	if err != nil {
		return fmt.Errorf("save window: %w", err)
	}
	return nil
}

// ListWindowsByPlan 按计划列出兼容窗口。
func (s *Store) ListWindowsByPlan(planID model.PlanID) ([]*model.CompatWindow, error) {
	rows, err := s.db.Query(
		`SELECT id, plan_id, reader_version_id, writer_version_id, rule_type, rule_payload, state, valid_from, valid_to, created_at
		 FROM compat_windows WHERE plan_id = ? ORDER BY created_at`, planID)
	if err != nil {
		return nil, fmt.Errorf("list windows: %w", err)
	}
	defer rows.Close()
	var out []*model.CompatWindow
	for rows.Next() {
		w, err := scanWindow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetWindow 按 ID 读取兼容窗口。
func (s *Store) GetWindow(id model.WindowID) (*model.CompatWindow, error) {
	row := s.db.QueryRow(
		`SELECT id, plan_id, reader_version_id, writer_version_id, rule_type, rule_payload, state, valid_from, valid_to, created_at
		 FROM compat_windows WHERE id = ?`, id)
	return scanWindow(row)
}

func scanWindow(r interface {
	Scan(...interface{}) error
}) (*model.CompatWindow, error) {
	var id, plan, rv, wv, rt, payload, state string
	var vf, vt, created int64
	if err := r.Scan(&id, &plan, &rv, &wv, &rt, &payload, &state, &vf, &vt, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan window: %w", err)
	}
	return &model.CompatWindow{
		ID:              model.WindowID(id),
		PlanID:          model.PlanID(plan),
		ReaderVersionID: model.SchemaVersionID(rv),
		WriterVersionID: model.SchemaVersionID(wv),
		RuleType:        model.WindowRuleType(rt),
		RulePayload:     payload,
		State:           state,
		ValidFrom:       vf,
		ValidTo:         vt,
		CreatedAt:       created,
	}, nil
}
