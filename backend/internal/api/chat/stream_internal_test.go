package chat

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/smartkrishi/backend/internal/agent"
)

// fakeFlusherRecorder is an httptest.ResponseRecorder that also implements
// http.Flusher, so writeSSEEvent's flush path is exercised.
type fakeFlusherRecorder struct {
	*httptest.ResponseRecorder
	flushes int
}

func (f *fakeFlusherRecorder) Flush() { f.flushes++ }

func newFlusherRecorder() *fakeFlusherRecorder {
	return &fakeFlusherRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func TestWriteSSEEvent_Framing(t *testing.T) {
	rec := newFlusherRecorder()

	ok := writeSSEEvent(rec, rec, agent.ResponseChunkEventValue("hello"))
	if !ok {
		t.Fatal("writeSSEEvent returned false for a healthy writer")
	}
	body := rec.Body.String()

	// Frame must be exactly `data: {json}\n\n`.
	if !strings.HasPrefix(body, "data: ") {
		t.Fatalf("frame must start with 'data: ', got %q", body)
	}
	if !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("frame must end with blank line, got %q", body)
	}
	// The JSON payload must contain the type and content, and be on one line.
	jsonPart := strings.TrimSuffix(strings.TrimPrefix(body, "data: "), "\n\n")
	if strings.Contains(jsonPart, "\n") {
		t.Fatalf("SSE JSON payload must not contain newlines: %q", jsonPart)
	}
	if !strings.Contains(jsonPart, `"type":"response_chunk"`) || !strings.Contains(jsonPart, `"content":"hello"`) {
		t.Fatalf("payload missing fields: %q", jsonPart)
	}
	if rec.flushes != 1 {
		t.Errorf("flush count = %d, want 1", rec.flushes)
	}
}

// failWriter fails on the Nth write to simulate a client disconnect mid-frame.
type failWriter struct {
	*httptest.ResponseRecorder
	failAfter int
	writes    int
}

func (fw *failWriter) Write(p []byte) (int, error) {
	fw.writes++
	if fw.writes > fw.failAfter {
		return 0, errWrite
	}
	return fw.ResponseRecorder.Write(p)
}
func (fw *failWriter) Flush() {}

var errWrite = &writeErr{}

type writeErr struct{}

func (*writeErr) Error() string { return "connection closed" }

func TestWriteSSEEvent_ClientDisconnect(t *testing.T) {
	fw := &failWriter{ResponseRecorder: httptest.NewRecorder(), failAfter: 0} // fail on first write
	if writeSSEEvent(fw, fw, agent.EndEventValue()) {
		t.Fatal("writeSSEEvent should return false when the write fails")
	}
}

func TestWriteSSEError_Framing(t *testing.T) {
	rec := newFlusherRecorder()
	writeSSEError(rec, "boom")
	body := rec.Body.String()
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("content-type = %q", rec.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(body, "data: ") || !strings.HasSuffix(body, "\n\n") {
		t.Fatalf("bad error frame: %q", body)
	}
	if !strings.Contains(body, `"type":"error"`) || !strings.Contains(body, `"error":"boom"`) {
		t.Fatalf("error payload missing fields: %q", body)
	}
}

func TestIntToString(t *testing.T) {
	if got := intToString(42); got != "42" {
		t.Fatalf("intToString(42) = %q", got)
	}
}

func TestReasoningStepFor_SkipsAnswerEvents(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	for _, ev := range []agent.Event{
		agent.ResponseChunkEventValue("hi"),
		agent.ResponseEventValue("done", nil),
		agent.EndEventValue(),
	} {
		if _, ok := reasoningStepFor(ev, mid, cid, 1); ok {
			t.Errorf("event %s should NOT be persisted as reasoning", ev.Type)
		}
	}
}

func TestReasoningStepFor_Plan(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	ev := agent.PlanEventValue(agent.Plan{PrimaryIntent: "advise"}, "raw")
	in, ok := reasoningStepFor(ev, mid, cid, 7)
	if !ok {
		t.Fatal("plan event should be persisted")
	}
	if in.StepType != "plan" || in.MessageID != mid || in.ChatID != cid || in.UserID != 7 {
		t.Fatalf("bad step: %+v", in)
	}
	if in.StepMetadata == nil {
		t.Error("step_metadata should carry the full event")
	}
	// BLOCKING: content must be non-empty (frontend filters empty-content steps).
	if in.Content == nil || strings.TrimSpace(*in.Content) == "" {
		t.Fatal("plan step content must be non-empty")
	}
	if *in.Content != "Planning: advise" {
		t.Errorf("plan content = %q, want %q", *in.Content, "Planning: advise")
	}
}

func TestReasoningStepFor_PlanNoIntent(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	in, _ := reasoningStepFor(agent.PlanEventValue(agent.Plan{}, ""), mid, cid, 1)
	if in.Content == nil || *in.Content != "Planning: Creating strategy" {
		t.Fatalf("plan fallback content = %v", in.Content)
	}
}

func TestReasoningStepFor_ToolCall(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	ev := agent.ToolCallEventValue("weather_api", "Pune", map[string]any{"loc": "Pune"})
	in, ok := reasoningStepFor(ev, mid, cid, 1)
	if !ok {
		t.Fatal("tool_call should be persisted")
	}
	if in.ToolName == nil || *in.ToolName != "weather_api" {
		t.Fatalf("tool_name = %v", in.ToolName)
	}
	if in.ToolArgs == nil || *in.ToolArgs != "Pune" {
		t.Fatalf("tool_args = %v", in.ToolArgs)
	}
	if in.ToolResult == nil {
		t.Error("tool_result should be set")
	}
	// BLOCKING: content must be non-empty.
	if in.Content == nil || *in.Content != "Using weather_api" {
		t.Fatalf("tool_call content = %v, want 'Using weather_api'", in.Content)
	}
}

func TestReasoningStepFor_ToolCallStructuredArgs(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	// non-string args must be JSON-marshaled into tool_args, not dropped.
	ev := agent.ToolCallEventValue("market_api", map[string]any{"crop": "wheat"}, nil)
	in, _ := reasoningStepFor(ev, mid, cid, 1)
	if in.ToolArgs == nil {
		t.Fatal("structured tool args should be JSON-marshaled, not dropped")
	}
	if !strings.Contains(*in.ToolArgs, `"crop":"wheat"`) {
		t.Errorf("tool_args = %q, want JSON with crop:wheat", *in.ToolArgs)
	}
}

func TestReasoningStepFor_CodeAndGrounding(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	code, _ := reasoningStepFor(agent.CodeEventValue("print(1)", "python"), mid, cid, 1)
	if code.Content == nil || *code.Content != "Executing: python" {
		t.Errorf("code content = %v", code.Content)
	}
	res, _ := reasoningStepFor(agent.CodeResultEventValue("OK", "1"), mid, cid, 1)
	if res.Content == nil || *res.Content != "Result: OK" {
		t.Errorf("code result content = %v", res.Content)
	}
}

func TestReasoningStepFor_CodeStageAndContent(t *testing.T) {
	mid, cid := uuid.New(), uuid.New()
	ev := agent.ThinkingEventValue("pondering")
	in, ok := reasoningStepFor(ev, mid, cid, 1)
	if !ok || in.StepType != "thinking" {
		t.Fatalf("thinking step wrong: %+v ok=%v", in, ok)
	}
	if in.Content == nil || *in.Content != "pondering" {
		t.Fatalf("content = %v", in.Content)
	}
}
