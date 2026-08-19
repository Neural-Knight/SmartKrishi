package llm

import (
	"context"
	"iter"
)

// Provider is the thin abstraction the agent nodes depend on. It has exactly
// two methods: a buffered Generate and a streaming GenerateStream. The single
// production implementation is internal/agent/gemini; internal/agent/llm/mock
// provides a deterministic in-memory fake for unit tests.
type Provider interface {
	// Generate runs a single non-streaming request and returns the full
	// response. Opts.Model must be set.
	Generate(ctx context.Context, req Request, opts Opts) (Response, error)

	// GenerateStream runs a streaming request. It returns a Go 1.23
	// range-over-func iterator yielding (chunk, error) pairs. Iteration stops
	// on the first non-nil error or when the stream completes. Callers consume
	// it with: for chunk, err := range provider.GenerateStream(...) { ... }.
	//
	// Opts.Model must be set. Cancelling ctx stops the stream.
	GenerateStream(ctx context.Context, req Request, opts Opts) iter.Seq2[StreamChunk, error]
}
