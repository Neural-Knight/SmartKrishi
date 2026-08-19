package agent

import "encoding/json"

// EventType enumerates the NDJSON event types emitted by the pipeline. These
// strings MUST match the Python /ask_stream contract and the frontend switch in
// useStreamingChat.ts / chatService.ts exactly — Step 7 (SSE) forwards these
// verbatim, so any drift breaks the unchanged frontend.
type EventType string

const (
	EventLog                     EventType = "log"
	EventPlan                    EventType = "plan"
	EventToolCall                EventType = "tool_call"
	EventThinking                EventType = "thinking"
	EventResponseChunk           EventType = "response_chunk"
	EventCodeExecution           EventType = "code_execution"
	EventGroundingWebSearchQuery EventType = "grounding_web_search_queries"
	EventGroundingChunks         EventType = "grounding_chunks"
	EventGroundingSupports       EventType = "grounding_supports"
	EventResponse                EventType = "response"
	EventEnd                     EventType = "end"
	EventError                   EventType = "error"
)

// Event is a single NDJSON event. It is a superset carrying every field any
// event kind uses; unused fields are omitted via omitempty so each serialized
// event matches the Python shape for its type. The JSON keys are the contract —
// do not rename them.
type Event struct {
	Type EventType `json:"type"`

	// Correlation fields set by the SSE layer on EVERY forwarded event. The
	// frontend (useStreamingChat.ts) drops any event without a message_id, so
	// these must be stamped on each frame. They are not produced by the pipeline
	// itself — the HTTP handler wraps events and fills them in.
	MessageID string `json:"message_id,omitempty"`
	ChatID    string `json:"chat_id,omitempty"`

	// FinalContent is set on the terminal end event so the frontend can
	// finalize the assistant message from a single field.
	FinalContent string `json:"final_content,omitempty"`

	// log
	Stage   string `json:"stage,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`

	// plan
	Plan        *Plan  `json:"plan,omitempty"`
	RawResponse string `json:"raw_response,omitempty"`

	// tool_call
	Tool   string `json:"tool,omitempty"`
	Args   any    `json:"args,omitempty"`
	Result any    `json:"result,omitempty"`

	// thinking / response_chunk
	Content string `json:"content,omitempty"`

	// code_execution
	Code     string `json:"code,omitempty"`
	Language string `json:"language,omitempty"`
	Outcome  string `json:"outcome,omitempty"`

	// grounding_web_search_queries
	Queries []string `json:"queries,omitempty"`
	// grounding_chunks
	Sources []GroundingSourceEvent `json:"sources,omitempty"`
	// grounding_supports
	Supports []GroundingSupportEvent `json:"supports,omitempty"`

	// response (final)
	Response          string             `json:"response,omitempty"`
	GroundingMetadata *GroundingMetadata `json:"grounding_metadata,omitempty"`

	// error
	Error string `json:"error,omitempty"`
}

// GroundingSourceEvent is one source in a grounding_chunks event (matches the
// Python {uri, title} shape).
type GroundingSourceEvent struct {
	URI   string `json:"uri"`
	Title string `json:"title"`
}

// GroundingSupportEvent is one citation in a grounding_supports event.
type GroundingSupportEvent struct {
	Segment             GroundingSegment `json:"segment"`
	GroundingChunkIndex []int32          `json:"grounding_chunk_indices"`
}

// GroundingSegment mirrors the Python segment object.
type GroundingSegment struct {
	StartIndex int32  `json:"start_index"`
	EndIndex   int32  `json:"end_index"`
	Text       string `json:"text"`
}

// GroundingMetadata is the serialized grounding attached to the final response
// event (matches the Python serialized_grounding shape).
type GroundingMetadata struct {
	WebSearchQueries []string                `json:"web_search_queries,omitempty"`
	GroundingChunks  []GroundingSourceEvent  `json:"grounding_chunks,omitempty"`
	GroundingSupport []GroundingSupportEvent `json:"grounding_supports,omitempty"`
}

// MarshalNDJSON serializes the event as a single JSON object followed by "\n",
// which is exactly one NDJSON line (the wire format the frontend parses).
func (e Event) MarshalNDJSON() ([]byte, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// --- constructors for the common events (keep call sites readable) ---

func logEvent(stage, message string) Event {
	return Event{Type: EventLog, Stage: stage, Message: message}
}

func logDataEvent(stage string, data any) Event {
	return Event{Type: EventLog, Stage: stage, Data: data}
}

func planEvent(p Plan, raw string) Event {
	pp := p
	return Event{Type: EventPlan, Plan: &pp, RawResponse: raw}
}

func toolCallEvent(tool string, args, result any) Event {
	return Event{Type: EventToolCall, Tool: tool, Args: args, Result: result}
}

func thinkingEvent(content string) Event {
	return Event{Type: EventThinking, Content: content}
}

func responseChunkEvent(content string) Event {
	return Event{Type: EventResponseChunk, Content: content}
}

func codeEvent(code, language string) Event {
	return Event{Type: EventCodeExecution, Stage: "code", Code: code, Language: language}
}

func codeResultEvent(outcome, result string) Event {
	return Event{Type: EventCodeExecution, Stage: "result", Outcome: outcome, Result: result}
}

func responseEvent(text string, md *GroundingMetadata) Event {
	return Event{Type: EventResponse, Response: text, GroundingMetadata: md}
}

func endEvent() Event {
	return Event{Type: EventEnd}
}

func errorEvent(msg string) Event {
	return Event{Type: EventError, Error: msg}
}

// ErrorEventValue is the exported error-event constructor for callers outside
// this package (e.g. the SSE handler emitting pre-stream failures).
func ErrorEventValue(msg string) Event {
	return errorEvent(msg)
}

// ResponseChunkEventValue is the exported response_chunk constructor (used by
// the SSE handler test to exercise framing).
func ResponseChunkEventValue(content string) Event {
	return responseChunkEvent(content)
}

// EndEventValue is the exported end-event constructor.
func EndEventValue() Event {
	return endEvent()
}

// PlanEventValue is the exported plan-event constructor.
func PlanEventValue(p Plan, raw string) Event {
	return planEvent(p, raw)
}

// ResponseEventValue is the exported response-event constructor.
func ResponseEventValue(text string, md *GroundingMetadata) Event {
	return responseEvent(text, md)
}
