package nodes

import (
	"context"
	"encoding/json"

	"github.com/smartkrishi/backend/internal/agent"
	"github.com/smartkrishi/backend/internal/agent/llm"
)

// Checker validates a draft answer and returns {approved, issues, conf_delta}.
//
// NOTE: the checker is intentionally NOT used in the streaming path — only the
// non-streaming answer path invokes it.
type Checker struct {
	llm   llm.Provider
	model string
}

// NewChecker builds a Checker. model is the resolved AGENT_CHECKER_MODEL.
func NewChecker(provider llm.Provider, model string) *Checker {
	return &Checker{llm: provider, model: model}
}

// verdict is the checker's JSON contract.
type verdict struct {
	Approved  bool     `json:"approved"`
	Issues    []string `json:"issues"`
	ConfDelta float64  `json:"conf_delta"`
}

// Run validates state.Draft and updates Approved / Issues / Confidence. On any
// LLM or parse failure it defaults to approved=true, so a flaky checker never
// blocks a produced answer.
func (c *Checker) Run(ctx context.Context, state *agent.State) error {
	payload := map[string]any{
		"query":  state.UserQuery,
		"plan":   state.Plan,
		"tools":  state.ToolCalls,
		"answer": state.Draft,
	}
	body, _ := json.MarshalIndent(payload, "", "  ")
	prompt := "Validate the answer below. Return JSON {approved:bool, issues:list, conf_delta:float}\n\n" + string(body)

	resp, err := c.llm.Generate(ctx, llm.Request{Prompt: prompt}, llm.Opts{
		Model: c.model,
		JSON:  true,
		Tools: llm.NativeTools{GoogleSearch: true},
	})
	if err != nil {
		state.Approved = true
		return nil
	}

	var v verdict
	if err := json.Unmarshal([]byte(stripCodeFence(resp.Text)), &v); err != nil {
		state.Approved = true
		return nil
	}
	state.Approved = v.Approved
	state.Issues = v.Issues
	state.Confidence += v.ConfDelta
	return nil
}
