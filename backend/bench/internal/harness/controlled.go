package harness

import (
	"context"
	"iter"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smartkrishi/backend/internal/agent/llm"
	agentruntime "github.com/smartkrishi/backend/internal/agent/runtime"
	"github.com/smartkrishi/backend/internal/agent/tools"
	authhandler "github.com/smartkrishi/backend/internal/api/auth"
	chathandler "github.com/smartkrishi/backend/internal/api/chat"
	"github.com/smartkrishi/backend/internal/database"
	"github.com/smartkrishi/backend/internal/repository/postgres"
	authservice "github.com/smartkrishi/backend/internal/service/auth"
	chatservice "github.com/smartkrishi/backend/internal/service/chat"
)

// Controlled is an in-process server for controlled-mode benchmarks. It wires
// the real auth + chat handlers over a scripted LLM provider and stubbed tool
// HTTP endpoints, backed by a real Postgres pool. This isolates our HTTP + DB +
// SSE + pipeline overhead from external (Gemini / weather / market) variance.
//
// It does NOT modify or replace production wiring — it reuses the exported
// constructors the production server uses, substituting only the LLM provider
// and tool HTTP endpoints via their public seams.
type Controlled struct {
	Server   *httptest.Server
	pool     *pgxpool.Pool
	toolStub *httptest.Server
	secret   string
}

// NewControlled builds the controlled server against the given DATABASE_URL and
// JWT secret. agentModel is recorded in results and passed to the runner.
func NewControlled(ctx context.Context, databaseURL, secret, agentModel string) (*Controlled, error) {
	pool, err := database.Connect(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	tokenMgr, err := authservice.NewTokenManager(secret, "HS256", 60)
	if err != nil {
		pool.Close()
		return nil, err
	}
	userRepo := postgres.NewUserRepository(pool)
	authSvc := authservice.NewService(userRepo, tokenMgr)

	fileRepo := postgres.NewFileRepository(pool)
	chatRepo := postgres.NewChatRepository(pool).WithFiles(fileRepo)
	chatSvc := chatservice.NewService(chatRepo)

	// Stub HTTP server for weather/market tools: returns instant canned JSON so
	// controlled runs never touch the Internet.
	toolStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Minimal shapes the tools tolerate; exact fields don't matter for timing.
		_, _ = w.Write([]byte(`{"records":[{"modal_price":"2000"}],"location":{"name":"Bench"},"current":{"temp_c":30,"condition":{"text":"Clear"}}}`))
	}))

	runner := agentruntime.New(scriptedProvider{}, chatRepo, agentruntime.Config{
		PlannerModel: agentModel,
		AgentModel:   agentModel,
		Tools: tools.Config{
			WeatherAPIKey:  "bench",
			WeatherBaseURL: toolStub.URL,
			DataGovKey:     "bench",
			AgmarknetID:    "bench",
			MarketBaseURL:  toolStub.URL,
		},
	})
	chatHandler := chathandler.NewHandler(chatSvc, authSvc).WithAgent(runner)

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Route("/api/v1/auth", authhandler.NewHandler(authSvc).Routes)
	r.Route("/api/v1/chat", chatHandler.Routes)

	return &Controlled{
		Server:   httptest.NewServer(r),
		pool:     pool,
		toolStub: toolStub,
		secret:   secret,
	}, nil
}

// BaseURL returns the controlled server base URL.
func (c *Controlled) BaseURL() string { return c.Server.URL }

// MaxConns returns the DB pool's configured max connections.
func (c *Controlled) MaxConns() int32 { return c.pool.Stat().MaxConns() }

// Close shuts down the controlled server, tool stub, and DB pool.
func (c *Controlled) Close() {
	c.Server.Close()
	c.toolStub.Close()
	c.pool.Close()
}

// scriptedProvider is a deterministic llm.Provider for controlled mode. It
// returns a fixed JSON plan for Generate (so the planner selects tools
// deterministically) and emits a scripted streaming answer with small, fixed
// inter-chunk delays so stream timing is stable and Internet-independent.
type scriptedProvider struct{}

// Generate returns a fixed plan (JSON mode) or a short answer otherwise.
func (scriptedProvider) Generate(_ context.Context, _ llm.Request, opts llm.Opts) (llm.Response, error) {
	if opts.JSON {
		// A plan requesting the three network-free-in-controlled tools.
		return llm.Response{Text: `{"primary_intent":"advise","tools_needed":["weather_api","soil_api","market_api"],"location":"Punjab","crop":"wheat","reasoning":"bench"}`}, nil
	}
	return llm.Response{Text: "Controlled-mode answer."}, nil
}

// GenerateStream emits a deterministic sequence: two thinking chunks then five
// answer chunks, each after a small fixed delay, ending the stream. The delays
// give realistic (but stable) stream timing without any external call.
func (scriptedProvider) GenerateStream(ctx context.Context, _ llm.Request, _ llm.Opts) iter.Seq2[llm.StreamChunk, error] {
	chunks := []llm.StreamChunk{
		{Text: "Considering the farm context.", Thought: true},
		{Text: "Planning the response.", Thought: true},
		{Text: "For your crop, "},
		{Text: "monitor soil moisture, "},
		{Text: "irrigate at crown-root, "},
		{Text: "and track mandi prices "},
		{Text: "before selling."},
	}
	const delay = 5 * time.Millisecond
	return func(yield func(llm.StreamChunk, error) bool) {
		for _, c := range chunks {
			select {
			case <-ctx.Done():
				yield(llm.StreamChunk{}, ctx.Err())
				return
			case <-time.After(delay):
			}
			if !yield(c, nil) {
				return
			}
		}
	}
}
