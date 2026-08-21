package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// benchEnabled reports whether benchmark/debug routes should be registered.
// Controlled solely by SMARTKRISHI_BENCH=1; it is never set in production.
func benchEnabled() bool {
	return os.Getenv("SMARTKRISHI_BENCH") == "1"
}

// registerBenchRoutes mounts the benchmark/debug routes when SMARTKRISHI_BENCH=1
// and is a no-op otherwise, so production behavior is unchanged when the env var
// is unset. Exposed data is read-only introspection:
//
//	GET /_bench/pool     pgxpool.Stat() JSON (nil pool -> {"pool":null})
//	GET /_bench/runtime  goroutine count + a small mem snapshot
//	/_debug/pprof/*      standard pprof, restricted to loopback clients only
func registerBenchRoutes(r chi.Router, pool *pgxpool.Pool, logger *slog.Logger) {
	if !benchEnabled() {
		return
	}
	logger.Warn("SMARTKRISHI_BENCH=1: exposing /_bench and localhost /_debug/pprof routes (do not use in production)")

	r.Get("/_bench/pool", benchPoolHandler(pool))
	r.Get("/_bench/runtime", benchRuntimeHandler())

	// pprof, restricted to loopback so it is never reachable off-host even when
	// the bench flag is on.
	r.Group(func(pr chi.Router) {
		pr.Use(loopbackOnly)
		pr.HandleFunc("/_debug/pprof/*", pprof.Index)
		pr.HandleFunc("/_debug/pprof/cmdline", pprof.Cmdline)
		pr.HandleFunc("/_debug/pprof/profile", pprof.Profile)
		pr.HandleFunc("/_debug/pprof/symbol", pprof.Symbol)
		pr.HandleFunc("/_debug/pprof/trace", pprof.Trace)
	})
}

// benchPoolHandler serves pgxpool.Stat() as JSON. The stat counters are
// cumulative; the bench client deltas them across a run window.
func benchPoolHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if pool == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"pool": nil})
			return
		}
		s := pool.Stat()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"max_conns":              s.MaxConns(),
			"total_conns":            s.TotalConns(),
			"acquired_conns":         s.AcquiredConns(),
			"idle_conns":             s.IdleConns(),
			"constructing_conns":     s.ConstructingConns(),
			"acquire_count":          s.AcquireCount(),
			"acquire_duration_ns":    s.AcquireDuration().Nanoseconds(),
			"empty_acquire_count":    s.EmptyAcquireCount(),
			"canceled_acquire_count": s.CanceledAcquireCount(),
			"new_conns_count":        s.NewConnsCount(),
			"max_lifetime_destroy":   s.MaxLifetimeDestroyCount(),
			"max_idle_destroy":       s.MaxIdleDestroyCount(),
		})
	}
}

// benchRuntimeHandler serves a lightweight runtime snapshot.
func benchRuntimeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"goroutines":       runtime.NumGoroutine(),
			"num_cpu":          runtime.NumCPU(),
			"gomaxprocs":       runtime.GOMAXPROCS(0),
			"heap_alloc_bytes": m.HeapAlloc,
			"heap_sys_bytes":   m.HeapSys,
			"sys_bytes":        m.Sys,
			"num_gc":           m.NumGC,
			"pause_total_ns":   m.PauseTotalNs,
		})
	}
}

// loopbackOnly restricts a handler to loopback (127.0.0.1 / ::1) clients, so
// pprof is never reachable from another host even with the bench flag on.
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.RemoteAddr
		if i := strings.LastIndex(host, ":"); i >= 0 {
			host = host[:i]
		}
		host = strings.Trim(host, "[]")
		if host != "127.0.0.1" && host != "::1" && host != "localhost" {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
