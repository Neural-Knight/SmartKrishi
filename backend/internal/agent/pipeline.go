package agent

import (
	"context"

	"github.com/smartkrishi/backend/internal/agent/llm"
)

// Planner is the pipeline's view of the planning node. *nodes.Planner
// satisfies it. It fills state.Plan and returns the raw model text so the
// pipeline can emit it in the plan event.
type Planner interface {
	Plan(ctx context.Context, state *State) (raw string, err error)
}

// Executor is the pipeline's view of the executor node. *nodes.Executor
// satisfies it. The pipeline drives tool execution and the streaming LLM call
// through this seam so buffered and streaming paths share identical routing,
// prompt, and model options.
type Executor interface {
	// RunTool executes one planned tool by name, storing the result in
	// state.ToolCalls. It returns (args, result, true) when the tool ran, or
	// (nil, nil, false) for deferred/unknown tools. args mirrors the Python
	// tool_call "args" field.
	RunTool(ctx context.Context, state *State, name string) (args any, result any, ran bool)
	// BuildPrompt returns the main-agent prompt for the current state.
	BuildPrompt(state *State) string
	// AgentOpts returns the llm.Opts (model + thinking + native tools) for the
	// main-agent call.
	AgentOpts() llm.Opts
	// Provider returns the underlying LLM provider to stream from.
	Provider() llm.Provider
}

// Pipeline orchestrates planner -> tools -> streaming executor and emits the
// NDJSON Event stream consumed (via Step 7 SSE) by the frontend. It is the Go
// equivalent of the Python /ask_stream event_generator, minus the persistence
// and HTTP concerns (those belong to Step 7).
//
// The checker is intentionally NOT part of the streaming path — per the
// migration handoff it is only used by the non-streaming /ask flow.
type Pipeline struct {
	planner  Planner
	executor Executor
	logs     bool // when true, emit verbose log events (Python `logs` flag)
	// toolAllow, when non-nil, restricts which planned tools may run (mirrors
	// the Python include_tools filter). nil = every planned tool is eligible.
	toolAllow map[string]struct{}
}

// NewPipeline builds a Pipeline. Set logs to emit the verbose log events the
// Python endpoint produces when its `logs` form field is true.
func NewPipeline(planner Planner, executor Executor, logs bool) *Pipeline {
	return &Pipeline{planner: planner, executor: executor, logs: logs}
}

// WithToolAllowList restricts the tools the pipeline will run to the given
// names (mirrors Python include_tools). An empty/nil list is a no-op (all
// planned tools remain eligible). Returns the pipeline for chaining.
func (p *Pipeline) WithToolAllowList(names []string) *Pipeline {
	if len(names) == 0 {
		p.toolAllow = nil
		return p
	}
	allow := make(map[string]struct{}, len(names))
	for _, n := range names {
		allow[n] = struct{}{}
	}
	p.toolAllow = allow
	return p
}

// Run executes the pipeline for state and calls emit for each Event in order.
// Emission order matches Python /ask_stream:
//
//	[log:initialization] [log:planner_start] plan [log:planner_complete]
//	tool_call* [log:tools_complete] [log:agent_start]
//	(thinking | response_chunk | code_execution | grounding_*)*
//	response [log:complete] end
//
// If emit returns false the pipeline stops early (consumer disconnected). Run
// itself does not error: failures are surfaced as error events; ctx cancellation
// during streaming ends the stream after emitting an error event. The caller is
// responsible for persisting state.Draft (Step 7).
func (p *Pipeline) Run(ctx context.Context, state *State, emit func(Event) bool) {
	if state.ToolCalls == nil {
		state.ToolCalls = make(map[string]any)
	}

	if p.logs {
		if !emit(logDataEvent("initialization", map[string]any{
			"user_id":       state.UserID,
			"chat_id":       state.ChatID,
			"query":         state.UserQuery,
			"history_count": len(state.History),
		})) {
			return
		}
		if !emit(logEvent("planner_start", "Starting planning phase")) {
			return
		}
	}

	// --- Planner ---
	raw, _ := p.planner.Plan(ctx, state)
	if !emit(planEvent(state.Plan, raw)) {
		return
	}
	if p.logs {
		if !emit(logEvent("planner_complete", "Plan created")) {
			return
		}
	}

	// --- Tools ---
	for _, name := range state.Plan.ToolsNeeded {
		if p.toolAllow != nil {
			if _, ok := p.toolAllow[name]; !ok {
				continue // filtered out by the include_tools allow-list
			}
		}
		args, result, ran := p.executor.RunTool(ctx, state, name)
		if !ran {
			continue
		}
		if !emit(toolCallEvent(name, args, result)) {
			return
		}
	}
	if p.logs {
		if !emit(logEvent("tools_complete", "Tool calls complete")) {
			return
		}
		if !emit(logEvent("agent_start", "Starting AI analysis")) {
			return
		}
	}

	// --- Main agent (streaming) ---
	p.streamAgent(ctx, state, emit)
}

// streamAgent runs the streaming LLM call, translating each llm.StreamChunk into
// the corresponding events, accumulating the final answer and grounding, then
// emits the terminal response + end events.
func (p *Pipeline) streamAgent(ctx context.Context, state *State, emit func(Event) bool) {
	prompt := p.executor.BuildPrompt(state)
	opts := p.executor.AgentOpts()

	var answer string
	grounding := &llm.Grounding{}

	for chunk, err := range p.executor.Provider().GenerateStream(ctx, llm.Request{Prompt: prompt}, opts) {
		if err != nil {
			emit(errorEvent(err.Error()))
			return
		}

		// Grounding metadata arrives on its own chunk; fan it out into the
		// discrete grounding_* events and accumulate for the final response.
		if chunk.Grounding != nil {
			mergeGrounding(grounding, chunk.Grounding)
			for _, ev := range groundingStreamEvents(chunk.Grounding) {
				if !emit(ev) {
					return
				}
			}
		}

		// Text: thought vs answer.
		if chunk.Text != "" {
			if chunk.Thought {
				if !emit(thinkingEvent(chunk.Text)) {
					return
				}
			} else {
				answer += chunk.Text
				if !emit(responseChunkEvent(chunk.Text)) {
					return
				}
			}
		}

		// Code execution events.
		if chunk.Code != nil {
			switch chunk.Code.Stage {
			case "code":
				if !emit(codeEvent(chunk.Code.Code, chunk.Code.Language)) {
					return
				}
			case "result":
				if !emit(codeResultEvent(chunk.Code.Outcome, chunk.Code.Result)) {
					return
				}
			}
		}
	}

	state.Draft = answer
	if !emit(responseEvent(answer, serializeGrounding(grounding))) {
		return
	}
	if p.logs {
		if !emit(logEvent("complete", "Response generated")) {
			return
		}
	}
	emit(endEvent())
}
