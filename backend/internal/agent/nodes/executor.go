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
	resp, err := e.llm.Generate(ctx, llm.Request{Prompt: prompt}, llm.Opts{
		Model:    e.model,
		Thinking: true,
		Tools: llm.NativeTools{
			GoogleSearch:  true,
			URLContext:    true,
			CodeExecution: true,
		},
	})
	if err != nil {
		// Match Python: leave the draft empty on model failure rather than
		// aborting the pipeline.
		state.Draft = ""
		return nil
	}
	state.Draft = resp.Text
	return nil
}

// runTools invokes each planned, available tool with the correct arguments.
// Unavailable file tools (Step 9) and unknown names are skipped gracefully.
func (e *Executor) runTools(ctx context.Context, state *agent.State) {
	if state.ToolCalls == nil {
		state.ToolCalls = make(map[string]any)
	}
	loc := state.Plan.Location
	crop := state.Plan.Crop

	for _, name := range state.Plan.ToolsNeeded {
		if e.tools == nil || !e.tools.Has(name) {
			// Deferred file tools and unknowns are simply not executed.
			continue
		}
		switch name {
		case tools.NameWeather:
			state.ToolCalls[name] = e.tools.Weather(ctx, loc)
		case tools.NameSoil:
			state.ToolCalls[name] = e.tools.Soil(ctx, loc)
		case tools.NameMarket:
			// Bug fix: market takes the CROP (Python passed location here).
			state.ToolCalls[name] = e.tools.Market(ctx, crop, regionFor(loc))
		case tools.NameChatHistory:
			state.ToolCalls[name] = e.tools.ChatHistory(ctx, tools.ChatHistoryArgs{
				Query:  state.UserQuery,
				UserID: state.UserID,
				ChatID: state.ChatID,
				Limit:  10,
			})
		}
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
