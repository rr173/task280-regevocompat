package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/model"
	"task280-regevocompat/internal/service"
	"task280-regevocompat/internal/store"
)

func newSchemaMux(t *testing.T) (*http.ServeMux, *service.Service) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "schema.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	svc := service.New(st)
	h := New(svc)
	mux := http.NewServeMux()
	h.Routes(mux)
	return mux, svc
}

// postSchema 创建 schema 版本并返回响应状态码与错误码（JSON body 中的 error 字段）。
func postSchema(t *testing.T, mux *http.ServeMux, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/schema-versions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp["error"]
}

// TestRegisterSchemaVersionDuplicateFieldConflict 验证登记重复字段名时，
// 接口应返回 409 Conflict（CONFLICT）而非 500 内部错误。
func TestRegisterSchemaVersionDuplicateFieldConflict(t *testing.T) {
	mux, _ := newSchemaMux(t)
	dup := `{"tag":"v-dup","fields":[{"name":"a","type":"string"},{"name":"a","type":"string"}]}`
	status, code := postSchema(t, mux, dup)
	if status != http.StatusConflict {
		t.Fatalf("duplicate field should return 409 Conflict, got %d (code=%s) body=%s", status, code, "")
	}
	if code != "CONFLICT" {
		t.Fatalf("duplicate field error code should be CONFLICT, got %q (status=%d)", code, status)
	}
}

// TestRegisterSchemaVersionInvalidArgument 确保参数错误仍返回 400，
// 与冲突 409 区分开，避免误把参数校验也当成冲突。
func TestRegisterSchemaVersionInvalidArgument(t *testing.T) {
	mux, _ := newSchemaMux(t)
	// 空字段名属于参数错误，应为 400 而非 409
	bad := `{"tag":"v-bad","fields":[{"name":"","type":"string"}]}`
	status, code := postSchema(t, mux, bad)
	if status != http.StatusBadRequest {
		t.Fatalf("empty field name should return 400 Bad Request, got %d (code=%s)", status, code)
	}
	if code != "INVALID_ARGUMENT" {
		t.Fatalf("empty field name error code should be INVALID_ARGUMENT, got %q (status=%d)", code, status)
	}
}

// TestRegisterSchemaVersionCreated 确保正常登记仍成功，回归不破坏正常路径。
func TestRegisterSchemaVersionCreated(t *testing.T) {
	mux, _ := newSchemaMux(t)
	ok := `{"tag":"v-ok","fields":[{"name":"a","type":"string"},{"name":"b","type":"int"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/schema-versions", bytes.NewBufferString(ok))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid schema should return 201 Created, got %d body=%s", rec.Code, rec.Body.String())
	}
	var v model.SchemaVersion
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if v.ID == "" || v.ContentHash == "" || len(v.Fields) != 2 {
		t.Fatalf("unexpected schema version: %+v", v)
	}
}
