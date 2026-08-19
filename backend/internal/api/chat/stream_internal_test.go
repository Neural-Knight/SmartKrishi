package chat

import (
	"net/http/httptest"
	"strings"
	"testing"

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
