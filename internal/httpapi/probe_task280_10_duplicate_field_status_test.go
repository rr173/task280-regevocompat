package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"task280-regevocompat/internal/service"
	"task280-regevocompat/internal/store"
)

func newProbeServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "probe-http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	h := New(service.New(st))
	mux := http.NewServeMux()
	h.Routes(mux)
	return mux
}

func TestDuplicateFieldMapsConflict(t *testing.T) {
	h := newProbeServer(t)
	body := strings.NewReader(`{"tag":"dup","fields":[{"name":"a","type":"string"},{"name":"a","type":"string"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schema-versions", body)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["error"] != "CONFLICT" {
		t.Fatalf("error=%q body=%v", got["error"], got)
	}
}
