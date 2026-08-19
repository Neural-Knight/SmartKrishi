// Package nodes implements the agent pipeline nodes (planner, executor,
// checker). Each node depends only on llm.Provider (and, for the executor, the
// tools.Registry) so it is unit testable with a mock provider and stubbed tools
// — no network, no API key.
package nodes

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/llm"
)

// plannerToolDescriptions is the planner's tool catalog. File tools are listed
// so the model can plan for them, even when the executor skips unavailable
// ones.
const plannerToolDescriptions = `
Available tools and their capabilities:
- weather_api: Get current weather conditions, forecasts, and historical weather data for any location
- soil_api: Get soil analysis, pH levels, nutrient content, and soil health recommendations
- market_api: Get current crop prices, market trends, and commodity information
- chat_history: Access previous conversations and advice given to the user
- get_pdf_content: Extract and analyze text content from uploaded PDF documents
- get_image_analysis: Analyze uploaded images for plant diseases, pests, crop conditions, or equipment
- list_uploaded_files: List all files uploaded by the user in this chat
- search_user_files: Search through user's uploaded files for specific content
- ask_question_about_files: Answer questions based on the content of uploaded files

Your role is PLANNING ONLY - do not execute any tools, just plan which ones would be helpful.`

// Planner produces a Plan for a user query using the LLM in JSON mode.
type Planner struct {
	llm   llm.Provider
	model string
}

// NewPlanner builds a Planner. model is the resolved AGENT_PLANNER_MODEL.
func NewPlanner(provider llm.Provider, model string) *Planner {
	return &Planner{llm: provider, model: model}
}

// Run fills state.Plan. On any LLM or parse failure it falls back to a safe
// default plan (weather_api).
func (p *Planner) Run(ctx context.Context, state *agent.State) error {
	_, err := p.Plan(ctx, state)
	return err
}

// Plan fills state.Plan and also returns the raw model text, so the streaming
// pipeline can emit it as the `plan` event's raw_response. On any LLM or parse
// failure it sets the safe default plan and returns nil error (the raw text, if
// any, is still returned).
func (p *Planner) Plan(ctx context.Context, state *agent.State) (raw string, err error) {
	prompt := buildPlannerPrompt(state.UserQuery)

	resp, err := p.llm.Generate(ctx, llm.Request{Prompt: prompt}, llm.Opts{
		Model: p.model,
		JSON:  true,
	})
	if err != nil {
		state.Plan = defaultPlan()
		return "", nil
	}

	plan, ok := parsePlan(resp.Text)
	if !ok {
		state.Plan = defaultPlan()
		return resp.Text, nil
	}
	state.Plan = plan
	return resp.Text, nil
}

func buildPlannerPrompt(query string) string {
	var b strings.Builder
	b.WriteString("You are an agricultural planning assistant. Analyze the user's query and create a plan for how to help them, but do NOT execute any tools.\n")
	b.WriteString(plannerToolDescriptions)
	b.WriteString("\n\nUser Query: \"")
	b.WriteString(query)
	b.WriteString("\"\n\n")
	b.WriteString("Return ONLY a JSON object with these keys:\n")
	b.WriteString(`- primary_intent: main goal (e.g. "weather_forecast", "crop_advice", "market_analysis", "file_analysis")` + "\n")
	b.WriteString("- tools_needed: array of tool names that would help answer this query\n")
	b.WriteString(`- location: any location mentioned, or "unknown"` + "\n")
	b.WriteString(`- crop: any crop mentioned, or "general"` + "\n")
	b.WriteString("- reasoning: brief explanation of the tool choices\n")
	return b.String()
}

// parsePlan tolerantly parses the model's JSON, stripping ```json fences the
// model sometimes adds despite JSON mode.
func parsePlan(text string) (agent.Plan, bool) {
	cleaned := stripCodeFence(text)
	if cleaned == "" {
		return agent.Plan{}, false
	}
	var plan agent.Plan
	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return agent.Plan{}, false
	}
	// A plan with no intent and no tools is not usable.
	if plan.PrimaryIntent == "" && len(plan.ToolsNeeded) == 0 {
		return agent.Plan{}, false
	}
	return plan, true
}

func defaultPlan() agent.Plan {
	return agent.Plan{
		PrimaryIntent: "advise",
		ToolsNeeded:   []string{"weather_api"},
		Location:      "unknown",
		Crop:          "general",
		Reasoning:     "Default fallback plan",
	}
}

// stripCodeFence removes a leading/trailing markdown code fence if present.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// Compile-time checks that the nodes satisfy the pipeline's interfaces.
var (
	_ agent.Planner  = (*Planner)(nil)
	_ agent.Executor = (*Executor)(nil)
)
