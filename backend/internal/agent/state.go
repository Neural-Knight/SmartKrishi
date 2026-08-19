// Package agent hosts the SmartKrishi agentic pipeline: the shared State/Plan
// types, the tool layer (internal/agent/tools), the planner/executor/checker
// nodes (internal/agent/nodes), and pipeline.Run which wires them into the
// event stream.
package agent

// Plan is the planner's structured output (primary_intent, tools_needed,
// location, crop, reasoning). It is the input the executor consumes to decide
// which tools to run and with what arguments.
type Plan struct {
	PrimaryIntent string   `json:"primary_intent"`
	ToolsNeeded   []string `json:"tools_needed"`
	Location      string   `json:"location"`
	Crop          string   `json:"crop"`
	Reasoning     string   `json:"reasoning"`
}

// HistoryMessage is one prior turn of conversation used for context.
type HistoryMessage struct {
	Role string `json:"role"`
	Msg  string `json:"msg"`
}

// State is the mutable pipeline state threaded through planner -> executor ->
// checker. Confidence defaults to 0.7; construct with NewState to get that
// default.
type State struct {
	UserID     string           // multi-user support
	ChatID     string           // chat scoping
	UserQuery  string           // the current user question
	Plan       Plan             // planner output
	History    []HistoryMessage // prior conversation turns
	ToolCalls  map[string]any   // tool name -> result payload
	Draft      string           // executor's draft answer
	Approved   bool             // checker verdict
	Issues     []string         // checker-reported issues
	Confidence float64          // running confidence (starts 0.7)
}

// RunOptions carries per-turn overrides for a streaming pipeline run: verbose
// logs, an agent-model override, and an optional tool allow-list (empty = all
// planned tools eligible).
type RunOptions struct {
	Logs  bool
	Model string
	Tools []string
}

// NewState returns a State with empty maps/slices and Confidence 0.7.
func NewState(userID, chatID, query string) *State {
	return &State{
		UserID:     userID,
		ChatID:     chatID,
		UserQuery:  query,
		ToolCalls:  make(map[string]any),
		History:    []HistoryMessage{},
		Issues:     []string{},
		Confidence: 0.7,
	}
}
