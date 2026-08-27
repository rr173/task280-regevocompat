package store

import (
	"database/sql"
	"fmt"

	"task280-regevocompat/internal/model"
)

// SaveRegion 持久化区域副本。
func (s *Store) SaveRegion(r *model.RegionReplica) error {
	_, err := s.db.Exec(
		`INSERT INTO region_replicas (id, name, current_version_id, state, upgraded_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=excluded.name,
		   current_version_id=excluded.current_version_id, state=excluded.state,
		   upgraded_at=excluded.upgraded_at`,
		r.ID, r.Name, r.CurrentVersionID, r.State, r.UpgradedAt, r.CreatedAt)
	if err != nil {
		return fmt.Errorf("save region: %w", err)
	}
	return nil
}

// GetRegion 按 ID 读取区域副本。
func (s *Store) GetRegion(id model.RegionID) (*model.RegionReplica, error) {
	row := s.db.QueryRow(
		`SELECT id, name, current_version_id, state, upgraded_at, created_at
		 FROM region_replicas WHERE id = ?`, id)
	return scanRegion(row)
}

// ListRegions 列出全部区域副本。
func (s *Store) ListRegions() ([]*model.RegionReplica, error) {
	rows, err := s.db.Query(
		`SELECT id, name, current_version_id, state, upgraded_at, created_at
		 FROM region_replicas ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	defer rows.Close()
	var out []*model.RegionReplica
	for rows.Next() {
		r, err := scanRegion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanRegion(r interface {
	Scan(...interface{}) error
}) (*model.RegionReplica, error) {
	var id, name, ver, state string
	var upgraded, created int64
	if err := r.Scan(&id, &name, &ver, &state, &upgraded, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan region: %w", err)
	}
	return &model.RegionReplica{
		ID:               model.RegionID(id),
		Name:             name,
		CurrentVersionID: model.SchemaVersionID(ver),
		State:            state,
		UpgradedAt:       upgraded,
		CreatedAt:        created,
	}, nil
}
