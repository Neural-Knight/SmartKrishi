// Package runtime assembles the agent pipeline from configuration and shared
// dependencies, and adapts it to the HTTP layer's AgentRunner interface. It is
// the single place that knows how to wire the Gemini provider, tool registry,
// and planner/executor nodes together for a streaming turn.
package runtime

import (
	"context"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/files"
	"github.com/smartkrishi/backend/internal/agent/gemini"
	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/nodes"
	"github.com/smartkrishi/backend/internal/agent/tools"
)

// Runner builds and runs the agent pipeline per streaming turn. It implements
// the chat handler's AgentRunner interface.
type Runner struct {
	provider     llm.Provider
	registry     *tools.Registry
	plannerModel string
	agentModel   string
}

// Config carries the model names and tool settings the runner needs.
type Config struct {
	PlannerModel string
	AgentModel   string
	Tools        tools.Config
}

// New builds a Runner from an LLM provider, a chat message reader (for the
// chat_history tool), and config. reader may be nil (chat_history then returns
// an error result). The provider is typically *gemini.Provider in production.
func New(provider llm.Provider, reader tools.MessageReader, cfg Config) *Runner {
	return &Runner{
		provider:     provider,
		registry:     tools.NewRegistry(nil, cfg.Tools, reader),
		plannerModel: cfg.PlannerModel,
		agentModel:   cfg.AgentModel,
	}
}

// NewFromGemini is a convenience constructor that builds the production Gemini
// provider from an API key. It returns the Runner and the provider's FileStore
// so the caller can wire file tools (WithFiles) and share the store with the
// file upload service.
func NewFromGemini(apiKey string, reader tools.MessageReader, cfg Config) (*Runner, *gemini.FileStore) {
	p := gemini.New(apiKey)
	return New(p, reader, cfg), p.FileStore()
}

// WithFiles enables the file tools on the runner's registry (Step 9), so the
// pipeline can list/search/analyze the chat's uploaded files. Returns the
// runner for chaining.
func (r *Runner) WithFiles(fileReader tools.FileReader, store files.Store) *Runner {
	r.registry.WithFiles(fileReader, store, r.agentModel)
	return r
}

// Run implements the chat handler's AgentRunner. It builds a planner + executor
// over the shared provider (overriding the agent model for this turn when
// opts.Model is set), applies the optional tool allow-list, and streams the
// pipeline, forwarding each event to emit.
func (r *Runner) Run(ctx context.Context, state *agent.State, opts agent.RunOptions, emit func(agent.Event) bool) {
	agentModel := r.agentModel
	if opts.Model != "" {
		agentModel = opts.Model
	}
	planner := nodes.NewPlanner(r.provider, r.plannerModel)
	executor := nodes.NewExecutor(r.provider, r.registry, agentModel)
	pipeline := agent.NewPipeline(planner, executor, opts.Logs).WithToolAllowList(opts.Tools)
	pipeline.Run(ctx, state, emit)
}
