package store

import (
	"fmt"

	"task280-regevocompat/internal/model"
)

// SaveSemantic 持久化读写语义声明。
func (s *Store) SaveSemantic(se *model.Semantic) error {
	_, err := s.db.Exec(
		`INSERT INTO read_write_semantics (id, schema_version_id, kind, hash, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET schema_version_id=excluded.schema_version_id,
		   kind=excluded.kind, hash=excluded.hash`,
		se.ID, se.SchemaVersionID, string(se.Kind), se.Hash, se.CreatedAt)
	if err != nil {
		return fmt.Errorf("save semantic: %w", err)
	}
	return nil
}

// ListSemantics 列出全部读写语义声明。
func (s *Store) ListSemantics() ([]*model.Semantic, error) {
	rows, err := s.db.Query(
		`SELECT id, schema_version_id, kind, hash, created_at FROM read_write_semantics ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list semantics: %w", err)
	}
	defer rows.Close()
	var out []*model.Semantic
	for rows.Next() {
		var id, sv, kind, hash string
		var created int64
		if err := rows.Scan(&id, &sv, &kind, &hash, &created); err != nil {
			return nil, fmt.Errorf("scan semantic: %w", err)
		}
		out = append(out, &model.Semantic{
			ID:              model.SemanticID(id),
			SchemaVersionID: model.SchemaVersionID(sv),
			Kind:            model.SemanticKind(kind),
			Hash:            hash,
			CreatedAt:       created,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveSample 持久化样本记录。
func (s *Store) SaveSample(rec *model.SampleRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO sample_records (id, schema_version_id, payload_json, hash, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET schema_version_id=excluded.schema_version_id,
		   payload_json=excluded.payload_json, hash=excluded.hash`,
		rec.ID, rec.SchemaVersionID, rec.PayloadJSON, rec.Hash, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("save sample: %w", err)
	}
	return nil
}

// AppendAudit 追加一条审计事件。
func (s *Store) AppendAudit(ev *model.AuditEvent) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_events (id, action, detail, created_at) VALUES (?, ?, ?, ?)`,
		ev.ID, ev.Action, ev.Detail, ev.CreatedAt)
	if err != nil {
		return fmt.Errorf("append audit: %w", err)
	}
	return nil
}
