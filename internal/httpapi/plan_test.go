package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"task280-regevocompat/internal/service"
	"task280-regevocompat/internal/store"
)

// TestGetPlanNotFound 验证查询不存在的演进计划时返回 404 NOT_FOUND，
// 而非因哨兵错误包裹链断裂退化为 500 INTERNAL。
func TestGetPlanNotFound(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "http.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	h := New(service.New(st))
	mux := http.NewServeMux()
	h.Routes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/plans/nonexistent-plan", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("get nonexistent plan: want status %d NOT_FOUND, got %d body=%s",
			http.StatusNotFound, rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got == "" || !contains(got, "NOT_FOUND") {
		t.Fatalf("get nonexistent plan: want error code NOT_FOUND in body, got %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
