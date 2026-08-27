package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestMissingPlanMapsNotFound(t *testing.T) {
	h := newProbeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/plans/pl_missing", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "NOT_FOUND" {
		t.Fatalf("error code=%q body=%v", body["error"], body)
	}
}
