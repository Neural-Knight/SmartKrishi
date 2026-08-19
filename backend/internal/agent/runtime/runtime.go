// Package runtime assembles the agent pipeline from configuration and shared
// dependencies, and adapts it to the HTTP layer's AgentRunner interface. It is
// the single place that knows how to wire the Gemini provider, tool registry,
// and planner/executor nodes together for a streaming turn.
package runtime

import (
	"context"
	"errors"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/files"
	"github.com/smartkrishi/backend/internal/agent/gemini"
	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/nodes"
	"github.com/smartkrishi/backend/internal/agent/tools"
)

// errNoImageSupport is returned by AnalyzeImage when the provider can't do
// vision (e.g. a non-Gemini or mock provider).
var errNoImageSupport = errors.New("image analysis not supported by this provider")

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

// agricultureSystemPrompt ports the Python AGRICULTURE_SYSTEM_PROMPT used by
// the simple (non-agent) text path.
const agricultureSystemPrompt = `You are SmartKrishi AI, an expert agricultural assistant designed to help Indian farmers.

IMPORTANT: ALWAYS RESPOND IN ENGLISH ONLY unless the user explicitly writes in another language.

You have deep knowledge of crop management, weather & climate, pest & disease control, market intelligence, sustainable farming, and farm planning. Provide practical, actionable, cost-effective advice for Indian farming conditions and seasons, in clear simple English. Structure responses with headings and bullet points, include a "Quick Tip", and end by inviting follow-up questions.`

// imageAnalysisPrompt ports the Python IMAGE_ANALYSIS_PROMPT.
const imageAnalysisPrompt = `RESPOND IN ENGLISH ONLY unless explicitly asked otherwise.

Analyze this agricultural image and provide insights on: 1) crop/plant identification, 2) health assessment, 3) visible issues (pests, diseases, deficiencies), 4) recommended actions, 5) prevention measures. Be specific about what you observe and provide actionable advice.`

// imageAnalyzer is the optional capability the legacy image endpoints need. The
// production *gemini.Provider satisfies it; a provider without it (e.g. a mock)
// makes AnalyzeImage return an error.
type imageAnalyzer interface {
	AnalyzeImage(ctx context.Context, model, prompt string, image []byte, mime string) (string, error)
}

// AskText answers a farming question with a simple, single Gemini call (system
// prompt + history + message) — NOT the agent pipeline. Ports the Python
// ai_service.process_text_message used by /ask and /send. history entries are
// {role, content} maps.
func (r *Runner) AskText(ctx context.Context, message string, history []map[string]string) (string, error) {
	msgs := make([]llm.Message, 0, len(history)+1)
	for _, h := range history {
		role := llm.RoleUser
		if h["role"] == "assistant" || h["role"] == "model" {
			role = llm.RoleModel
		}
		if c := h["content"]; c != "" {
			msgs = append(msgs, llm.Message{Role: role, Text: c})
		}
	}
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Text: message})

	resp, err := r.provider.Generate(ctx, llm.Request{Messages: msgs}, llm.Opts{
		Model:  r.agentModel,
		System: agricultureSystemPrompt,
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// AnalyzeImage runs a stateless vision call over the image (legacy image
// endpoints). Ports ai_service.process_image_message. Returns an error if the
// underlying provider does not support image analysis.
func (r *Runner) AnalyzeImage(ctx context.Context, message string, image []byte, mime string) (string, error) {
	ia, ok := r.provider.(imageAnalyzer)
	if !ok {
		return "", errNoImageSupport
	}
	prompt := imageAnalysisPrompt + "\n\nUser Question: " + message
	return ia.AnalyzeImage(ctx, r.agentModel, prompt, image, mime)
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
