// Package llm defines a thin, provider-agnostic abstraction over a large
// language model used by the agent pipeline (planner / executor / checker).
//
// The interface is intentionally minimal: only Generate and GenerateStream,
// plus the option/message types they need. Nodes depend on llm.Provider only,
// never on a concrete SDK (e.g. google.golang.org/genai). This keeps nodes
// unit-testable against llm/mock with no network or API key.
//
// The single production implementation lives in internal/agent/gemini.
// We deliberately do NOT model OpenAI/Anthropic here (see MIGRATION.md:
// "Agent LLM abstraction — DECISION"). The types below capture the shape of a
// Gemini streaming response (thinking, grounding, code execution) because the
// frontend NDJSON/SSE contract already depends on those signals, but nothing
// in this package imports the Gemini SDK.
package llm

// Role identifies who authored a message in a conversation.
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
)

// Message is a single turn of conversation content sent to the model.
// Only text is modeled here; file/media parts are deferred to Step 9.
type Message struct {
	Role Role
	Text string
}

// NativeTools toggles Gemini's server-side tools. These are Gemini-specific
// capabilities, but they are expressed as plain booleans here so nodes can
// request them without importing the SDK. A non-Gemini provider is free to
// ignore fields it cannot honor.
type NativeTools struct {
	GoogleSearch  bool
	URLContext    bool
	CodeExecution bool
}

// Opts carries per-call configuration. Model is required (callers pass the
// per-role model name resolved from config, e.g. AGENT_PLANNER_MODEL).
type Opts struct {
	// Model is the provider model identifier, e.g. "gemini-2.5-flash".
	Model string

	// System is an optional system instruction prepended to the request.
	System string

	// Temperature, when non-nil, overrides the provider default.
	Temperature *float32

	// Thinking requests the model expose its reasoning as thought parts.
	// Streamed thought text arrives as StreamChunk with Thought=true.
	Thinking bool

	// Tools enables Gemini native server-side tools.
	Tools NativeTools

	// JSON requests the model return a raw JSON object (used by the planner).
	// Implementations should set the response MIME type accordingly.
	JSON bool
}

// Request is a single generation request.
type Request struct {
	// Messages is the ordered conversation history plus the current turn.
	// If empty, Prompt is used as a single user message.
	Messages []Message

	// Prompt is a convenience for single-shot calls. Ignored when Messages
	// is non-empty.
	Prompt string
}

// GroundingSource is one web source returned by Google Search grounding.
type GroundingSource struct {
	URI   string
	Title string
}

// GroundingSupport links a span of the response text to grounding sources.
type GroundingSupport struct {
	StartIndex int32
	EndIndex   int32
	Text       string
	// ChunkIndices index into the Sources slice of the owning Grounding.
	ChunkIndices []int32
}

// Grounding aggregates Google Search grounding metadata for a response.
type Grounding struct {
	WebSearchQueries []string
	Sources          []GroundingSource
	Supports         []GroundingSupport
}

// IsEmpty reports whether any grounding data is present.
func (g *Grounding) IsEmpty() bool {
	if g == nil {
		return true
	}
	return len(g.WebSearchQueries) == 0 && len(g.Sources) == 0 && len(g.Supports) == 0
}

// CodeExecution captures either code the model chose to run or its result.
// Exactly one of Code / Result semantics applies per chunk, distinguished by
// Stage ("code" or "result").
type CodeExecution struct {
	Stage    string // "code" | "result"
	Code     string // set when Stage == "code"
	Language string // set when Stage == "code"
	Outcome  string // set when Stage == "result"
	Result   string // set when Stage == "result"
}

// StreamChunk is one incremental piece of a streaming response. A chunk carries
// exactly one kind of payload; consumers should check the fields in order.
type StreamChunk struct {
	// Text is a piece of the answer or, when Thought is true, a piece of the
	// model's reasoning.
	Text string

	// Thought marks Text as reasoning rather than final answer content.
	// This maps to the Gemini part.thought flag and drives the frontend's
	// "thinking" vs "response_chunk" events.
	Thought bool

	// Grounding, when non-nil, carries Google Search grounding metadata
	// surfaced during streaming.
	Grounding *Grounding

	// Code, when non-nil, carries an executable-code or code-result event.
	Code *CodeExecution
}

// Response is the aggregated result of a non-streaming Generate call.
type Response struct {
	// Text is the full answer (thought parts excluded).
	Text string

	// Thoughts is the concatenated reasoning, if Thinking was requested.
	Thoughts string

	// Grounding is the final grounding metadata, if any.
	Grounding *Grounding
}
