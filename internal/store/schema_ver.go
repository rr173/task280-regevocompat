package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"task280-regevocompat/internal/model"
)

// SaveSchemaVersion 持久化一个 schema 版本。
func (s *Store) SaveSchemaVersion(v *model.SchemaVersion) error {
	fields, err := json.Marshal(v.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO schema_versions (id, tag, content_hash, fields_json, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET tag=excluded.tag, content_hash=excluded.content_hash,
		   fields_json=excluded.fields_json`,
		v.ID, v.Tag, v.ContentHash, string(fields), v.CreatedAt)
	if err != nil {
		return fmt.Errorf("save schema version: %w", err)
	}
	return nil
}

// GetSchemaVersion 按 ID 读取 schema 版本。
func (s *Store) GetSchemaVersion(id model.SchemaVersionID) (*model.SchemaVersion, error) {
	row := s.db.QueryRow(
		`SELECT id, tag, content_hash, fields_json, created_at FROM schema_versions WHERE id = ?`, id)
	return scanSchemaVersion(row)
}

// ListSchemaVersions 列出全部 schema 版本。
func (s *Store) ListSchemaVersions() ([]*model.SchemaVersion, error) {
	rows, err := s.db.Query(
		`SELECT id, tag, content_hash, fields_json, created_at FROM schema_versions ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list schema versions: %w", err)
	}
	defer rows.Close()
	var out []*model.SchemaVersion
	for rows.Next() {
		v, err := scanSchemaVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func scanSchemaVersion(r interface {
	Scan(...interface{}) error
}) (*model.SchemaVersion, error) {
	var id, tag, hash, fieldsJSON string
	var created int64
	if err := r.Scan(&id, &tag, &hash, &fieldsJSON, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("scan schema version: %w", err)
	}
	var fields []model.Field
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return nil, fmt.Errorf("unmarshal fields: %w", err)
	}
	return &model.SchemaVersion{
		ID:          model.SchemaVersionID(id),
		Tag:         tag,
		ContentHash: hash,
		Fields:      fields,
		CreatedAt:   created,
	}, nil
}

// NowMillis 返回当前毫秒时间戳（UTC+8 业务时间由调用方决定，此处用 UTC 毫秒）。
func NowMillis() int64 { return time.Now().UnixMilli() }
