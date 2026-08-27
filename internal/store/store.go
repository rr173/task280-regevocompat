// Package store 提供基于 modernc.org/sqlite 的持久化：建表迁移与全部 CRUD。
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store 持有数据库连接，所有 DAO 方法挂在 *Store 上。
type Store struct {
	db *sql.DB
}

// Open 打开（不存在则创建）SQLite 数据库并执行建表迁移。
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(0)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(32)
	db.SetMaxIdleConns(32)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// DB 暴露底层连接（供 store 内方法使用）。
func (s *Store) DB() *sql.DB { return s.db }

// Close 关闭数据库连接。
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_versions (
			id TEXT PRIMARY KEY,
			tag TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			fields_json TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS region_replicas (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			current_version_id TEXT NOT NULL,
			state TEXT NOT NULL,
			upgraded_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS migration_plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			state TEXT NOT NULL,
			baseline_version_id TEXT NOT NULL,
			target_version_id TEXT NOT NULL,
			sealed_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS migration_steps (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			ordinal INTEGER NOT NULL,
			kind TEXT NOT NULL,
			description TEXT NOT NULL,
			from_field TEXT NOT NULL,
			to_fields_json TEXT NOT NULL,
			dual_write INTEGER NOT NULL,
			stop_dual_write INTEGER NOT NULL,
			state TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS read_write_semantics (
			id TEXT PRIMARY KEY,
			schema_version_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS compat_windows (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			reader_version_id TEXT NOT NULL,
			writer_version_id TEXT NOT NULL,
			rule_type TEXT NOT NULL,
			rule_payload TEXT NOT NULL,
			state TEXT NOT NULL,
			valid_from INTEGER NOT NULL,
			valid_to INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS conflict_paths (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			region_id TEXT NOT NULL,
			reader_version_id TEXT NOT NULL,
			writer_version_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			field TEXT NOT NULL,
			reason TEXT NOT NULL,
			severity TEXT NOT NULL,
			resolved INTEGER NOT NULL,
			detected_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS compat_snapshots (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			state TEXT NOT NULL,
			content_json TEXT NOT NULL,
			hash TEXT NOT NULL,
			superseded_by TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sample_records (
			id TEXT PRIMARY KEY,
			schema_version_id TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			hash TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			action TEXT NOT NULL,
			detail TEXT NOT NULL,
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_steps_plan ON migration_steps(plan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_semantics_sv ON read_write_semantics(schema_version_id)`,
		`CREATE INDEX IF NOT EXISTS idx_windows_plan ON compat_windows(plan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_conflicts_plan ON conflict_paths(plan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_plan ON compat_snapshots(plan_id)`,
		`CREATE INDEX IF NOT EXISTS idx_samples_sv ON sample_records(schema_version_id)`,
	}
	for _, st := range stmts {
		if _, err := s.db.Exec(st); err != nil {
			return fmt.Errorf("migrate: %w (sql=%s)", err, strings.TrimSpace(st))
		}
	}
	return nil
}
