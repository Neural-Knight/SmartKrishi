package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReadyHandler_NoPool: with no DB configured there is no hard dependency to
// check, so /ready reports ready (200). (A live-DB down case would return 503,
// but that path needs a real broken pool and is covered by manual/ops checks.)
func TestReadyHandler_NoPool(t *testing.T) {
	h := readyHandler(nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when no pool configured", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"ready"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
