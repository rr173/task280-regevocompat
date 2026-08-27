package store

import (
	"path/filepath"
	"testing"
	"time"

	"task280-regevocompat/internal/model"
)

// TestListSemanticsDoesNotExhaustConnection 回归测试：ListSemantics 曾在末尾执行一次
// 未关闭的 db.Query，独占连接池（SetMaxOpenConns(1)），导致后续任意查询永久阻塞。
// 调用 ListSemantics 之后必须能立即执行后续查询。
func TestListSemanticsDoesNotExhaustConnection(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "sem.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	se := &model.Semantic{
		ID:              model.SemanticID("sm_one"),
		SchemaVersionID: model.SchemaVersionID("sv_any"),
		Kind:            model.SemanticRead,
		Hash:            "deadbeef",
		CreatedAt:       1,
	}
	if err := s.SaveSemantic(se); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListSemantics(); err != nil {
		t.Fatalf("list semantics: %v", err)
	}

	// 后续查询必须立刻返回，而非等待被泄漏 Rows 占据的连接。
	done := make(chan struct{})
	go func() {
		_, _ = s.ListSemantics()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ListSemantics blocked after a prior list: connection leaked")
	}
}

// TestListSchemaVersionsDoesNotExhaustConnection 回归测试：ListSchemaVersions 存在同样的
// 泄漏末尾 Query，调用后必须能继续查询。
func TestListSchemaVersionsDoesNotExhaustConnection(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "sv.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })

	v := &model.SchemaVersion{
		ID:          model.SchemaVersionID("sv_one"),
		Tag:         "v1",
		ContentHash: "hash",
		Fields:      []model.Field{{Name: "id", Type: model.FieldString}},
		CreatedAt:   1,
	}
	if err := s.SaveSchemaVersion(v); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListSchemaVersions(); err != nil {
		t.Fatalf("list schema versions: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_, _ = s.ListSchemaVersions()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ListSchemaVersions blocked after a prior list: connection leaked")
	}
}
