package store

import (
	"database/sql"
	"fmt"

	"task280-regevocompat/internal/model"
)

// SaveSnapshot 持久化兼容快照。
func (s *Store) SaveSnapshot(snap *model.CompatSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO compat_snapshots (id, plan_id, state, content_json, hash, superseded_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET state=excluded.state, content_json=excluded.content_json,
		   hash=excluded.hash, superseded_by=excluded.superseded_by`,
		snap.ID, snap.PlanID, snap.State, snap.ContentJSON, snap.Hash, string(snap.SupersededBy), snap.CreatedAt)
	if err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	return nil
}

// ListSnapshotsByPlan 按计划列出兼容快照。
func (s *Store) ListSnapshotsByPlan(planID model.PlanID) ([]*model.CompatSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, plan_id, state, content_json, hash, superseded_by, created_at
		 FROM compat_snapshots WHERE plan_id = ? ORDER BY created_at`, planID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	var out []*model.CompatSnapshot
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// GetSnapshot 按 ID 读取兼容快照。
func (s *Store) GetSnapshot(id model.SnapshotID) (*model.CompatSnapshot, error) {
	row := s.db.QueryRow(
		`SELECT id, plan_id, state, content_json, hash, superseded_by, created_at
		 FROM compat_snapshots WHERE id = ?`, id)
	return scanSnapshot(row)
}

func scanSnapshot(r interface {
	Scan(...interface{}) error
}) (*model.CompatSnapshot, error) {
	var id, plan, state, content, hash, sup string
	var created int64
	if err := r.Scan(&id, &plan, &state, &content, &hash, &sup, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan snapshot: %w", err)
	}
	return &model.CompatSnapshot{
		ID:          model.SnapshotID(id),
		PlanID:      model.PlanID(plan),
		State:       state,
		ContentJSON: content,
		Hash:        hash,
		SupersededBy: model.SnapshotID(sup),
		CreatedAt:   created,
	}, nil
}
