// Package mock provides a deterministic, network-free implementation of
// llm.Provider for unit testing agent nodes (planner / executor / checker).
// It never touches the Gemini SDK or the network.
package mock

import (
	"context"
	"iter"

	"github.com/smartkrishi/backend/internal/agent/llm"
)

// Provider is a fake llm.Provider driven by scripted responses. The zero value
// is usable: Generate returns an empty response and GenerateStream yields a
// single chunk containing GenerateText.
//
// It records the last request/opts it received so tests can assert on them.
type Provider struct {
	// GenerateText is returned as Response.Text by Generate, and streamed as a
	// single response chunk by GenerateStream, unless the more specific fields
	// below are set.
	GenerateText string

	// GenerateFunc, when set, fully overrides Generate.
	GenerateFunc func(ctx context.Context, req llm.Request, opts llm.Opts) (llm.Response, error)

	// StreamChunks, when non-empty, is the exact sequence GenerateStream emits
	// (overriding GenerateText). Use this to script thinking / grounding /
	// code-execution events for node tests.
	StreamChunks []llm.StreamChunk

	// StreamErr, when set, is yielded after the scripted chunks (or first, if
	// there are none) to exercise error handling.
	StreamErr error

	// Recorded state from the most recent call, for assertions.
	LastRequest llm.Request
	LastOpts    llm.Opts
	Calls       int
}

// compile-time interface check.
var _ llm.Provider = (*Provider)(nil)

// Generate implements llm.Provider.
func (p *Provider) Generate(ctx context.Context, req llm.Request, opts llm.Opts) (llm.Response, error) {
	p.Calls++
	p.LastRequest = req
	p.LastOpts = opts
	if p.GenerateFunc != nil {
		return p.GenerateFunc(ctx, req, opts)
	}
	return llm.Response{Text: p.GenerateText}, nil
}

// GenerateStream implements llm.Provider. It replays StreamChunks (or a single
// GenerateText response chunk) and honors ctx cancellation between chunks.
func (p *Provider) GenerateStream(ctx context.Context, req llm.Request, opts llm.Opts) iter.Seq2[llm.StreamChunk, error] {
	p.Calls++
	p.LastRequest = req
	p.LastOpts = opts

	chunks := p.StreamChunks
	if len(chunks) == 0 && p.StreamErr == nil {
		chunks = []llm.StreamChunk{{Text: p.GenerateText}}
	}

	return func(yield func(llm.StreamChunk, error) bool) {
		for _, c := range chunks {
			if err := ctx.Err(); err != nil {
				yield(llm.StreamChunk{}, err)
				return
			}
			if !yield(c, nil) {
				return
			}
		}
		if p.StreamErr != nil {
			yield(llm.StreamChunk{}, p.StreamErr)
		}
	}
}
