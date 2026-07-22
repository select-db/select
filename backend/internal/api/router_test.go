package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/internal/api"
)

// Method enforcement lives at the router (POST patterns), not in the handlers,
// so a wrong method is rejected with 405 before any auth or handler code runs.
func TestRouter_RejectsWrongMethod(t *testing.T) {
	mux := http.NewServeMux()
	api.Register(mux)

	for _, path := range []string{"/apikey/list", "/datasource/upsert", "/sync/v1/sync", "/mcp"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want 405", path, rec.Code)
		}
	}
}
