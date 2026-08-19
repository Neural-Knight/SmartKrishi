package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flushRecorder) Flush() { f.flushed = true }

// TestLoggingMiddlewarePreservesFlusher verifies the logging wrapper still
// satisfies http.Flusher so SSE handlers can flush events to the client.
func TestLoggingMiddlewarePreservesFlusher(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	rec := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	handler := Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter lost http.Flusher after logging middleware")
		}
		w.WriteHeader(http.StatusOK)
		f.Flush()
	}))

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/chat/send-stream", nil))

	if !rec.flushed {
		t.Fatal("Flush was not called on underlying recorder")
	}
}
