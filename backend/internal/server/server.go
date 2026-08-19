package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	agentruntime "github.com/smartkrishi/backend/internal/agent/runtime"
	"github.com/smartkrishi/backend/internal/agent/tools"
	"github.com/smartkrishi/backend/internal/api"
	authhandler "github.com/smartkrishi/backend/internal/api/auth"
	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/config"
	"github.com/smartkrishi/backend/internal/database"
	"github.com/smartkrishi/backend/internal/firebase"
	appmiddleware "github.com/smartkrishi/backend/internal/middleware"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

const version = "1.0.0"

// Server wraps the HTTP server and dependencies.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	pool   *pgxpool.Pool
	http   *http.Server
}

// New creates a configured HTTP server.
func New(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var pool *pgxpool.Pool
	var err error
	if cfg.DatabaseURL != "" {
		pool, err = database.Connect(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
	}

	router := newRouter(cfg, logger, pool)

	return &Server{
		cfg:    cfg,
		logger: logger,
		pool:   pool,
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       120 * time.Second,
		},
	}, nil
}

func newRouter(cfg *config.Config, logger *slog.Logger, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	r.Use(chimiddleware.RealIP)
	r.Use(appmiddleware.RequestID)
	r.Use(appmiddleware.Logging(logger))
	r.Use(appmiddleware.Recovery(logger))
	r.Use(appmiddleware.CORS(cfg.CORSOrigins))

	r.Get("/", rootHandler)
	r.Get("/health", healthHandler(pool))

	if pool != nil {
		tokenManager, err := authservice.NewTokenManager(cfg.SecretKey, cfg.Algorithm, cfg.AccessTokenExpireMinutes)
		if err != nil {
			logger.Error("auth disabled: invalid token config", "error", err)
		} else {
			userRepo := postgres.NewUserRepository(pool)
			authSvc := authservice.NewService(userRepo, tokenManager)

			// Enable mobile (Firebase phone) auth when credentials are configured.
			fbClient := firebase.NewClient(cfg.FirebaseCredentials, cfg.FirebaseProjectID)
			if fbClient.Configured() {
				authSvc.WithFirebase(fbClient)
				logger.Info("firebase mobile auth enabled", "project_id", cfg.FirebaseProjectID)
			} else {
				logger.Warn("firebase mobile auth disabled: FIREBASE_CREDENTIALS not set")
			}

			authHandler := authhandler.NewHandler(authSvc)

			r.Route(cfg.APIV1Str+"/auth", authHandler.Routes)

			chatRepo := postgres.NewChatRepository(pool)
			chatSvc := chatservice.NewService(chatRepo)
			chatHandler := chathandler.NewHandler(chatSvc, authSvc)

			// Enable the streaming AI endpoint when a Gemini key is configured.
			// The agent runs in-process; chatRepo backs the chat_history tool.
			if cfg.GeminiAPIKey != "" {
				runner := agentruntime.NewFromGemini(cfg.GeminiAPIKey, chatRepo, agentruntime.Config{
					PlannerModel: cfg.AgentPlannerModel,
					AgentModel:   cfg.AgentModel,
					Tools: tools.Config{
						WeatherAPIKey: cfg.WeatherAPIKey,
						DataGovKey:    cfg.DataGovKey,
						AgmarknetID:   cfg.AgmarknetID,
					},
				})
				chatHandler.WithAgent(runner)
				logger.Info("agent streaming enabled", "planner_model", cfg.AgentPlannerModel, "agent_model", cfg.AgentModel)
			} else {
				logger.Warn("agent streaming disabled: GEMINI_API_KEY not set")
			}

			r.Route(cfg.APIV1Str+"/chat", chatHandler.Routes)
		}
	}

	r.Route(cfg.APIV1Str, func(r chi.Router) {
		r.Get("/status", func(w http.ResponseWriter, _ *http.Request) {
			api.WriteJSON(w, http.StatusOK, map[string]string{
				"status":  "ok",
				"message": "SmartKrishi Go API",
				"version": version,
			})
		})
	})

	return r
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Welcome to SmartKrishi API",
		"version": version,
		"docs":    "/docs",
		"redoc":   "/redoc",
	})
}

func healthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dbStatus := "disconnected"
		status := "unhealthy"

		if pool != nil {
			if err := database.Ping(r.Context(), pool); err == nil {
				dbStatus = "connected"
				status = "healthy"
			} else {
				dbStatus = "disconnected"
			}
		}

		api.WriteJSON(w, http.StatusOK, map[string]string{
			"status":   status,
			"service":  "SmartKrishi API",
			"database": dbStatus,
			"version":  version,
		})
	}
}

// Start begins serving HTTP traffic.
func (s *Server) Start() error {
	s.logger.Info("starting server", "addr", s.http.Addr, "env", s.cfg.Environment)
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")
	if s.pool != nil {
		defer s.pool.Close()
	}
	return s.http.Shutdown(ctx)
}
