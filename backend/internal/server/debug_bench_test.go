package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// mount builds a router with only the bench routes registered, so the test
// exercises the env gate in isolation (no DB or full server needed).
func mount(t *testing.T) http.Handler {
	t.Helper()
	r := chi.NewRouter()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registerBenchRoutes(r, nil, logger)
	return r
}

func TestBenchRoutes_OffByDefault(t *testing.T) {
	t.Setenv("SMARTKRISHI_BENCH", "") // explicitly unset
	h := mount(t)

	for _, path := range []string{"/_bench/pool", "/_bench/runtime", "/_debug/pprof/"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("with bench disabled, %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestBenchRoutes_OnWhenEnabled(t *testing.T) {
	t.Setenv("SMARTKRISHI_BENCH", "1")
	h := mount(t)

	// /_bench/pool with a nil pool returns 200 and {"pool":null} (no DB needed).
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_bench/pool", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/_bench/pool = %d, want 200", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/_bench/runtime", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/_bench/runtime = %d, want 200", rec.Code)
	}
}

func TestBenchPprof_LoopbackOnly(t *testing.T) {
	t.Setenv("SMARTKRISHI_BENCH", "1")
	h := mount(t)

	// Non-loopback client is rejected (404), even with the flag on.
	req := httptest.NewRequest(http.MethodGet, "/_debug/pprof/", nil)
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("pprof from non-loopback = %d, want 404", rec.Code)
	}

	// Loopback client is allowed through the gate (pprof index responds 200).
	req = httptest.NewRequest(http.MethodGet, "/_debug/pprof/", nil)
	req.RemoteAddr = "127.0.0.1:5555"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("pprof from loopback = %d, want 200", rec.Code)
	}
}
