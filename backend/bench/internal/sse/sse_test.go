package sse

import (
	"strings"
	"testing"
	"time"
)

// buildStream joins frames using the exact wire format the server emits:
// `data: {json}\n\n`.
func buildStream(frames ...string) string {
	var b strings.Builder
	for _, f := range frames {
		b.WriteString("data: ")
		b.WriteString(f)
		b.WriteString("\n\n")
	}
	return b.String()
}

func TestParse_HappyPath(t *testing.T) {
	stream := buildStream(
		`{"type":"log","message":"starting"}`,
		`{"type":"plan","steps":["a","b"]}`,
		`{"type":"tool_call","tool":"weather_api"}`,
		`{"type":"thinking","text":"hmm"}`,
		`{"type":"response_chunk","text":"Hello "}`,
		`{"type":"response_chunk","text":"world"}`,
		`{"type":"response","text":"Hello world"}`,
		`{"type":"end"}`,
	)

	start := time.Now()
	m, err := Parse(strings.NewReader(stream), start)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if m.EventsTotal != 8 {
		t.Errorf("EventsTotal = %d, want 8", m.EventsTotal)
	}
	if m.ToolCallCount != 1 {
		t.Errorf("ToolCallCount = %d, want 1", m.ToolCallCount)
	}
	if len(m.ToolCallNames) != 1 || m.ToolCallNames[0] != "weather_api" {
		t.Errorf("ToolCallNames = %v, want [weather_api]", m.ToolCallNames)
	}
	if !m.StreamCompleted {
		t.Errorf("StreamCompleted = false, want true")
	}
	if m.SawError {
		t.Errorf("SawError = true, want false")
	}
	if m.TimeToFirstEvent <= 0 {
		t.Errorf("TimeToFirstEvent = %v, want > 0", m.TimeToFirstEvent)
	}
	if m.TimeToPlan <= 0 {
		t.Errorf("TimeToPlan = %v, want > 0", m.TimeToPlan)
	}
	if m.TimeToFirstToken <= 0 {
		t.Errorf("TimeToFirstToken = %v, want > 0", m.TimeToFirstToken)
	}
	// Ordering invariant: plan arrives before first token, first token before
	// stream end.
	if !(m.TimeToPlan <= m.TimeToFirstToken) {
		t.Errorf("TimeToPlan (%v) should be <= TimeToFirstToken (%v)", m.TimeToPlan, m.TimeToFirstToken)
	}
	if !(m.TimeToFirstToken <= m.StreamDuration) {
		t.Errorf("TimeToFirstToken (%v) should be <= StreamDuration (%v)", m.TimeToFirstToken, m.StreamDuration)
	}
}

func TestParse_FirstTokenFromThinking(t *testing.T) {
	// thinking arrives before any response_chunk, so it should set the token.
	stream := buildStream(
		`{"type":"plan"}`,
		`{"type":"thinking","text":"reasoning"}`,
		`{"type":"response_chunk","text":"answer"}`,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.TimeToFirstToken <= 0 {
		t.Errorf("TimeToFirstToken = %v, want > 0 (set by thinking)", m.TimeToFirstToken)
	}
	if m.TimeToFirstToken < m.TimeToPlan {
		t.Errorf("TimeToFirstToken (%v) should be >= TimeToPlan (%v)", m.TimeToFirstToken, m.TimeToPlan)
	}
}

func TestParse_ErrorEvent(t *testing.T) {
	stream := buildStream(
		`{"type":"plan"}`,
		`{"type":"error","error":"tool exploded"}`,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !m.SawError {
		t.Errorf("SawError = false, want true")
	}
	if m.ErrorMessage != "tool exploded" {
		t.Errorf("ErrorMessage = %q, want %q", m.ErrorMessage, "tool exploded")
	}
}

func TestParse_FileUploaded(t *testing.T) {
	stream := buildStream(
		`{"type":"file_uploaded","name":"soil.jpg"}`,
		`{"type":"response_chunk","text":"analysis"}`,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !m.FileUploaded {
		t.Errorf("FileUploaded = false, want true")
	}
}

func TestParse_StopsAtEnd(t *testing.T) {
	// Frames after "end" must be ignored.
	stream := buildStream(
		`{"type":"plan"}`,
		`{"type":"end"}`,
		`{"type":"tool_call","tool":"should_not_count"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.EventsTotal != 2 {
		t.Errorf("EventsTotal = %d, want 2 (frames after end ignored)", m.EventsTotal)
	}
	if m.ToolCallCount != 0 {
		t.Errorf("ToolCallCount = %d, want 0", m.ToolCallCount)
	}
	if !m.StreamCompleted {
		t.Errorf("StreamCompleted = false, want true")
	}
}

func TestParse_MalformedFrameSkipped(t *testing.T) {
	stream := buildStream(
		`{"type":"plan"}`,
		`{not valid json`,
		`{"type":"response_chunk","text":"ok"}`,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	// plan + response_chunk + end = 3 parseable frames; malformed one skipped.
	if m.EventsTotal != 3 {
		t.Errorf("EventsTotal = %d, want 3 (malformed frame skipped)", m.EventsTotal)
	}
	if !m.StreamCompleted {
		t.Errorf("StreamCompleted = false, want true")
	}
}

func TestParse_NoEndClosesOnEOF(t *testing.T) {
	stream := buildStream(
		`{"type":"plan"}`,
		`{"type":"response_chunk","text":"partial"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.StreamCompleted {
		t.Errorf("StreamCompleted = true, want false (no end event)")
	}
	if m.StreamDuration <= 0 {
		t.Errorf("StreamDuration = %v, want > 0 (set on EOF)", m.StreamDuration)
	}
	if m.EventsTotal != 2 {
		t.Errorf("EventsTotal = %d, want 2", m.EventsTotal)
	}
}

func TestParse_IgnoresNonDataLines(t *testing.T) {
	// Comment lines, event: fields and blank lines must be ignored.
	stream := ": this is a comment\n" +
		"event: message\n" +
		"data: {\"type\":\"plan\"}\n\n" +
		"\n" +
		"data: {\"type\":\"end\"}\n\n"
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.EventsTotal != 2 {
		t.Errorf("EventsTotal = %d, want 2", m.EventsTotal)
	}
	if m.TimeToPlan <= 0 {
		t.Errorf("TimeToPlan = %v, want > 0", m.TimeToPlan)
	}
}

func TestParse_LongResponseChunkNoTruncation(t *testing.T) {
	// A single data line well beyond bufio.Scanner's 64KB default must be
	// parsed intact.
	big := strings.Repeat("A", 200*1024) // 200KB payload
	frame := `{"type":"response_chunk","text":"` + big + `"}`
	stream := buildStream(
		`{"type":"plan"}`,
		frame,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.EventsTotal != 3 {
		t.Errorf("EventsTotal = %d, want 3 (long frame must parse without truncation)", m.EventsTotal)
	}
	if m.TimeToFirstToken <= 0 {
		t.Errorf("TimeToFirstToken = %v, want > 0 (set by long response_chunk)", m.TimeToFirstToken)
	}
	if !m.StreamCompleted {
		t.Errorf("StreamCompleted = false, want true")
	}
}

func TestParse_MultipleToolCallsInOrder(t *testing.T) {
	stream := buildStream(
		`{"type":"tool_call","tool":"weather_api"}`,
		`{"type":"tool_call","tool":"crop_db"}`,
		`{"type":"tool_call","tool":"market_price"}`,
		`{"type":"end"}`,
	)
	m, err := Parse(strings.NewReader(stream), time.Now())
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if m.ToolCallCount != 3 {
		t.Errorf("ToolCallCount = %d, want 3", m.ToolCallCount)
	}
	want := []string{"weather_api", "crop_db", "market_price"}
	for i, w := range want {
		if i >= len(m.ToolCallNames) || m.ToolCallNames[i] != w {
			t.Errorf("ToolCallNames = %v, want %v", m.ToolCallNames, want)
			break
		}
	}
}
