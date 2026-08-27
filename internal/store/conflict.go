package store

import (
	"fmt"

	"task280-regevocompat/internal/model"
)

// SaveConflicts 批量持久化冲突路径：在同计划维度上以单事务原子地
// 「清空旧冲突 + 写入新冲突」。
//
// 把清空与重写放进同一个事务，可保证观察者要么读到替换前的旧清单、
// 要么读到替换后的新清单，绝不会读到「刚清空、新结果尚未写入」的空清单中间态。
// 同计划探索与列举之间的串行互斥由 Service 层 serialMu 保证，此处事务为底层兜底。
func (s *Store) SaveConflicts(planID model.PlanID, conflicts []*model.ConflictPath) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save conflicts: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`DELETE FROM conflict_paths WHERE plan_id = ?`, planID); err != nil {
		return fmt.Errorf("clear conflicts: %w", err)
	}
	for _, c := range conflicts {
		if _, err := tx.Exec(
			`INSERT INTO conflict_paths
			 (id, plan_id, region_id, reader_version_id, writer_version_id, step_id, field, reason, severity, resolved, detected_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, c.PlanID, c.RegionID, c.ReaderVersionID, c.WriterVersionID, c.StepID,
			c.Field, c.Reason, c.Severity, boolToInt(c.Resolved), c.DetectedAt); err != nil {
			return fmt.Errorf("insert conflict: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save conflicts: %w", err)
	}
	committed = true
	return nil
}

// ListConflictsByPlan 按计划列出冲突路径。
func (s *Store) ListConflictsByPlan(planID model.PlanID) ([]*model.ConflictPath, error) {
	rows, err := s.db.Query(
		`SELECT id, plan_id, region_id, reader_version_id, writer_version_id, step_id, field, reason, severity, resolved, detected_at
		 FROM conflict_paths WHERE plan_id = ? ORDER BY detected_at`, planID)
	if err != nil {
		return nil, fmt.Errorf("list conflicts: %w", err)
	}
	defer rows.Close()
	var out []*model.ConflictPath
	for rows.Next() {
		var id, plan, rid, rv, wv, step, field, reason, sev string
		var resolved, detected int
		if err := rows.Scan(&id, &plan, &rid, &rv, &wv, &step, &field, &reason, &sev, &resolved, &detected); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		out = append(out, &model.ConflictPath{
			ID:              model.ConflictID(id),
			PlanID:          model.PlanID(plan),
			RegionID:        model.RegionID(rid),
			ReaderVersionID: model.SchemaVersionID(rv),
			WriterVersionID: model.SchemaVersionID(wv),
			StepID:          model.StepID(step),
			Field:           field,
			Reason:          reason,
			Severity:        sev,
			Resolved:        resolved != 0,
			DetectedAt:      int64(detected),
		})
	}
	return out, rows.Err()
}

// SetConflictResolved 标记冲突是否已消解。
func (s *Store) SetConflictResolved(planID model.PlanID, id model.ConflictID, resolved bool) error {
	_, err := s.db.Exec(
		`UPDATE conflict_paths SET resolved = ? WHERE plan_id = ? AND id = ?`,
		boolToInt(resolved), planID, id)
	if err != nil {
		return fmt.Errorf("set conflict resolved: %w", err)
	}
	return nil
}
