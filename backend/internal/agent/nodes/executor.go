package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/llm"
	"github.com/smartkrishi/backend/internal/agent/tools"
)

// Executor runs the planned tools and generates the draft answer. It is the Go
// equivalent of the Python main_agent_node, minus the streaming/event emission
// which lands in Step 6c (pipeline). Here it produces a buffered draft via
// llm.Generate so it stays deterministically unit-testable.
type Executor struct {
	llm   llm.Provider
	tools *tools.Registry
	model string
}

// NewExecutor builds an Executor. model is the resolved AGENT_MODEL.
func NewExecutor(provider llm.Provider, registry *tools.Registry, model string) *Executor {
	return &Executor{llm: provider, tools: registry, model: model}
}

// Run executes the plan's tools into state.ToolCalls, then generates
// state.Draft. Tool routing fixes the Python bug: market gets plan.Crop while
// weather/soil get plan.Location.
func (e *Executor) Run(ctx context.Context, state *agent.State) error {
	e.runTools(ctx, state)

	prompt := buildAgentPrompt(state)
	resp, err := e.llm.Generate(ctx, llm.Request{Prompt: prompt}, e.AgentOpts())
	if err != nil {
		// Match Python: leave the draft empty on model failure rather than
		// aborting the pipeline.
		state.Draft = ""
		return nil
	}
	state.Draft = resp.Text
	return nil
}

// RunTools invokes each planned, available tool with the correct arguments,
// populating state.ToolCalls. It is exported so the streaming pipeline (6c) can
// run tools and emit a tool_call event per tool while sharing the exact same
// routing/args as the buffered Run path.
func (e *Executor) RunTools(ctx context.Context, state *agent.State) {
	e.runTools(ctx, state)
}

// RunTool executes a single planned tool by name with the correct arguments,
// stores the result in state.ToolCalls, and returns (args, result, true). If
// the tool is unavailable (deferred file tool or unknown) it returns
// (nil, nil, false) and does nothing. The streaming pipeline uses this to emit
// one tool_call event per tool. args mirrors the Python tool_call "args" field.
func (e *Executor) RunTool(ctx context.Context, state *agent.State, name string) (args any, result any, ran bool) {
	if e.tools == nil || !e.tools.Has(name) {
		return nil, nil, false
	}
	if state.ToolCalls == nil {
		state.ToolCalls = make(map[string]any)
	}
	loc := state.Plan.Location
	crop := state.Plan.Crop

	switch name {
	case tools.NameWeather:
		args = loc
		result = e.tools.Weather(ctx, loc)
	case tools.NameSoil:
		args = loc
		result = e.tools.Soil(ctx, loc)
	case tools.NameMarket:
		args = crop // bug fix: market takes the crop, not the location
		result = e.tools.Market(ctx, crop, regionFor(loc))
	case tools.NameChatHistory:
		args = "chat_history_args"
		result = e.tools.ChatHistory(ctx, tools.ChatHistoryArgs{
			Query:  state.UserQuery,
			UserID: state.UserID,
			ChatID: state.ChatID,
			Limit:  10,
		})
	default:
		return nil, nil, false
	}
	state.ToolCalls[name] = result
	return args, result, true
}

// BuildPrompt returns the SmartKrishi agent prompt for the current state. It is
// exported so the pipeline can build the prompt once and stream the LLM call
// itself (the buffered Run and the streaming pipeline share this prompt).
func (e *Executor) BuildPrompt(state *agent.State) string {
	return buildAgentPrompt(state)
}

// AgentOpts returns the llm.Opts the executor uses for the main agent call
// (thinking + native tools). Shared by Run and the streaming pipeline so both
// paths request identical model behavior.
func (e *Executor) AgentOpts() llm.Opts {
	return llm.Opts{
		Model:    e.model,
		Thinking: true,
		Tools: llm.NativeTools{
			GoogleSearch:  true,
			URLContext:    true,
			CodeExecution: true,
		},
	}
}

// Provider exposes the underlying llm.Provider so the pipeline can stream.
func (e *Executor) Provider() llm.Provider { return e.llm }

// runTools invokes each planned, available tool with the correct arguments.
// Unavailable file tools (Step 9) and unknown names are skipped gracefully.
// It delegates to RunTool so buffered and streaming paths route identically.
func (e *Executor) runTools(ctx context.Context, state *agent.State) {
	for _, name := range state.Plan.ToolsNeeded {
		e.RunTool(ctx, state, name)
	}
}

// regionFor maps a planner location to a market region filter. "unknown"/empty
// means national scope.
func regionFor(loc string) string {
	if loc == "" || strings.EqualFold(loc, "unknown") {
		return "national"
	}
	return loc
}

// buildAgentPrompt ports the SmartKrishi agent prompt from main.py /ask_stream:
// language mirroring, code-output instruction, and the thinking guide, with the
// user query, history, and gathered tool data.
func buildAgentPrompt(state *agent.State) string {
	var hist strings.Builder
	for _, m := range state.History {
		fmt.Fprintf(&hist, "%s: %s\n", m.Role, m.Msg)
	}

	toolData, _ := json.MarshalIndent(state.ToolCalls, "", "  ")

	var b strings.Builder
	b.WriteString("You are SmartKrishi Agent, an advanced AI agricultural advisor specializing in farming intelligence, crop management, and agricultural technology.\n\n")
	b.WriteString("LANGUAGE INSTRUCTION: Respond in the EXACT same language and script as the user's query. Match the user's linguistic style completely.\n\n")
	b.WriteString("CODE EXECUTION OUTPUT INSTRUCTION: Don't just mention that you ran code - show the user what the code produced by writing the output yourself in the answer as text/markdown.\n\n")
	b.WriteString("THINKING GUIDE:\n")
	b.WriteString("1. Identify and clearly define the core agricultural problem or question\n")
	b.WriteString("2. State any assumptions transparently; ask for clarification if the query is ambiguous\n")
	b.WriteString("3. Break the problem into logical parts and solve them systematically\n")
	b.WriteString("4. Search the internet and write code when required for data analysis or agricultural calculations\n")
	b.WriteString("5. Reflect on the steps taken and check for gaps before answering\n")
	b.WriteString("6. Provide the comprehensive agricultural solution with clear explanations\n\n")
	fmt.Fprintf(&b, "User Query: %s\n\n", state.UserQuery)
	fmt.Fprintf(&b, "Conversation History:\n%s\n", hist.String())
	fmt.Fprintf(&b, "Available Agricultural Data & Tools:\n%s\n\n", string(toolData))
	b.WriteString("As SmartKrishi Agent, provide expert agricultural guidance following the thinking guide above.")
	return b.String()
}
