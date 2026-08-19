package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/smartkrishi/backend/internal/config"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
)

const version = "1.0.0"

// Server wraps the HTTP server and dependencies.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	http   *http.Server
}

// New creates a configured HTTP server.
func New(cfg *config.Config, logger *slog.Logger) *Server {
	router := newRouter(cfg, logger)

	return &Server{
		cfg:    cfg,
		logger: logger,
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		},
	}
}

func newRouter(cfg *config.Config, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.Logging(logger))
	r.Use(appmiddleware.Recovery(logger))
	r.Use(appmiddleware.CORS(cfg.CORSOrigins))

	r.Get("/", rootHandler)
	r.Get("/health", healthHandler(cfg))

	// API v1 routes will be mounted here in subsequent milestones.
	r.Route(cfg.APIV1Str, func(r chi.Router) {
		r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{
				"status":  "ok",
				"message": "SmartKrishi Go API scaffold",
				"version": version,
			})
		})
	})

	return r
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Welcome to SmartKrishi API",
		"version": version,
		"docs":    "/docs",
		"redoc":   "/redoc",
	})
}

func healthHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		dbStatus := "disconnected"
		status := "unhealthy"

		if cfg.DatabaseURL != "" {
			// Full DB ping will be added when pgxpool is wired in Milestone 1.
			dbStatus = "configured"
			status = "healthy"
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":   status,
			"service":  "SmartKrishi API",
			"database": dbStatus,
			"version":  version,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Start begins serving HTTP traffic.
func (s *Server) Start() error {
	s.logger.Info("starting server", "addr", s.http.Addr, "env", s.cfg.Environment)
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")
	return s.http.Shutdown(ctx)
}
