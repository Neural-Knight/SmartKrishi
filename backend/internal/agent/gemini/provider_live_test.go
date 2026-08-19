package gemini_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/smartkrishi/backend/internal/agent/gemini"
	"github.com/smartkrishi/backend/internal/agent/llm"
)

// live returns the API key and skips the test when it is unset, mirroring the
// chat API integration tests that skip without DATABASE_URL.
func live(t *testing.T) string {
	t.Helper()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		t.Skip("no GEMINI_API_KEY / GOOGLE_API_KEY set; skipping live Gemini smoke test")
	}
	return key
}

func model() string {
	if m := os.Getenv("AGENT_MODEL"); m != "" {
		return m
	}
	return "gemini-2.5-flash"
}

// TestLiveGenerate is a minimal round-trip against the real API.
func TestLiveGenerate(t *testing.T) {
	key := live(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	p := gemini.New(key)
	resp, err := p.Generate(ctx, llm.Request{Prompt: "Reply with exactly the word: pong"}, llm.Opts{Model: model()})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if resp.Text == "" {
		t.Fatalf("empty response text")
	}
	t.Logf("Generate text: %q", resp.Text)
}

// TestLiveStreamThinking verifies streamed answer chunks and thought parts.
func TestLiveStreamThinking(t *testing.T) {
	key := live(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	p := gemini.New(key)
	req := llm.Request{Prompt: "Briefly, what is crop rotation? Answer in two sentences."}
	opts := llm.Opts{Model: model(), Thinking: true}

	var answerChunks, thoughtChunks int
	var answer string
	for chunk, err := range p.GenerateStream(ctx, req, opts) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		switch {
		case chunk.Thought && chunk.Text != "":
			thoughtChunks++
		case chunk.Text != "":
			answerChunks++
			answer += chunk.Text
		}
	}

	t.Logf("answer chunks=%d thought chunks=%d", answerChunks, thoughtChunks)
	t.Logf("answer: %q", answer)
	if answerChunks == 0 {
		t.Fatalf("no answer chunks received")
	}
	// Thinking is best-effort: flash models may or may not emit thoughts.
	// Do not hard-fail on zero thought chunks, just report.
	if thoughtChunks == 0 {
		t.Logf("note: no thought chunks emitted by %s (acceptable)", model())
	}
}

// TestLiveStreamGrounding verifies GoogleSearch grounding metadata flows through
// the streaming path (web search queries and/or sources).
func TestLiveStreamGrounding(t *testing.T) {
	key := live(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	p := gemini.New(key)
	req := llm.Request{Prompt: "Using web search, what is today's approximate mandi price of wheat in Punjab, India? Cite sources."}
	opts := llm.Opts{Model: model(), Tools: llm.NativeTools{GoogleSearch: true}}

	var groundingChunks, sources, queries int
	for chunk, err := range p.GenerateStream(ctx, req, opts) {
		if err != nil {
			t.Fatalf("stream error: %v", err)
		}
		if chunk.Grounding != nil {
			groundingChunks++
			sources += len(chunk.Grounding.Sources)
			queries += len(chunk.Grounding.WebSearchQueries)
		}
	}

	t.Logf("grounding chunks=%d sources=%d queries=%d", groundingChunks, sources, queries)
	if groundingChunks == 0 {
		t.Fatalf("no grounding metadata received; expected GoogleSearch grounding")
	}
}
