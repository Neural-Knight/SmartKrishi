// Package agent hosts the SmartKrishi agentic pipeline ported from the Python
// Agentic-AI/app. Step 6b adds the shared State/Plan types, the tool layer
// (internal/agent/tools), and the planner/executor/checker nodes
// (internal/agent/nodes). Step 6c wires them into pipeline.Run + event stream.
package agent

// Plan is the planner's structured output. It mirrors the JSON contract the
// Python planner_node emits (primary_intent, tools_needed, location, crop,
// reasoning), plus is the input the executor consumes to decide which tools to
// run and with what arguments.
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
// checker. It ports the Python dataclass State (state.py). Confidence defaults
// to 0.7 as in Python; construct with NewState to get that default.
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
// logs, an agent-model override, and an optional tool allow-list (mirrors the
// Python include_tools filter; empty = all planned tools eligible).
type RunOptions struct {
	Logs  bool
	Model string
	Tools []string
}

// NewState returns a State initialized like the Python dataclass defaults:
// empty maps/slices and Confidence 0.7.
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
