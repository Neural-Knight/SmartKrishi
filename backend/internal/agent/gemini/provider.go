package gemini

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/genai"

	"github.com/smartkrishi/backend/internal/agent/llm"
)

// Provider is the production llm.Provider backed by google.golang.org/genai.
// It holds a per-chat client pool. Construct it with New.
type Provider struct {
	pool *clientPool

	// chatID scopes generation to a specific chat's client for File API
	// access isolation. Empty means the shared default client.
	chatID string
}

// compile-time interface check.
var _ llm.Provider = (*Provider)(nil)

// New returns a Provider using the given Gemini API key. Calls made through it
// use the shared default client (chatID ""). Use ForChat to bind a chat id.
func New(apiKey string) *Provider {
	return &Provider{pool: newClientPool(apiKey)}
}

// ForChat returns a shallow copy of the Provider bound to chatID, sharing the
// same underlying client pool. Generation for that chat uses a stable client so
// uploaded files remain accessible.
func (p *Provider) ForChat(chatID string) *Provider {
	return &Provider{pool: p.pool, chatID: chatID}
}

// Generate implements llm.Provider (non-streaming).
func (p *Provider) Generate(ctx context.Context, req llm.Request, opts llm.Opts) (llm.Response, error) {
	if opts.Model == "" {
		return llm.Response{}, fmt.Errorf("gemini: Opts.Model is required")
	}
	client, err := p.pool.get(ctx, p.chatID)
	if err != nil {
		return llm.Response{}, err
	}

	resp, err := client.Models.GenerateContent(ctx, opts.Model, buildContents(req), buildConfig(opts))
	if err != nil {
		return llm.Response{}, fmt.Errorf("gemini: generate: %w", err)
	}

	var answer, thoughts strings.Builder
	var grounding *llm.Grounding
	if len(resp.Candidates) > 0 {
		cand := resp.Candidates[0]
		if g := extractGrounding(cand.GroundingMetadata); !g.IsEmpty() {
			grounding = g
		}
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				if part.Text == "" {
					continue
				}
				if part.Thought {
					thoughts.WriteString(part.Text)
				} else {
					answer.WriteString(part.Text)
				}
			}
		}
	}

	return llm.Response{
		Text:      answer.String(),
		Thoughts:  thoughts.String(),
		Grounding: grounding,
	}, nil
}

// GenerateStream implements llm.Provider (streaming). It ranges over the genai
// stream and translates each response into llm.StreamChunk values: thought text,
// answer text, grounding metadata, and code-execution events.
func (p *Provider) GenerateStream(ctx context.Context, req llm.Request, opts llm.Opts) iter.Seq2[llm.StreamChunk, error] {
	return func(yield func(llm.StreamChunk, error) bool) {
		if opts.Model == "" {
			yield(llm.StreamChunk{}, fmt.Errorf("gemini: Opts.Model is required"))
			return
		}
		client, err := p.pool.get(ctx, p.chatID)
		if err != nil {
			yield(llm.StreamChunk{}, err)
			return
		}

		stream := client.Models.GenerateContentStream(ctx, opts.Model, buildContents(req), buildConfig(opts))
		for resp, err := range stream {
			if err != nil {
				yield(llm.StreamChunk{}, fmt.Errorf("gemini: stream: %w", err))
				return
			}
			if len(resp.Candidates) == 0 {
				continue
			}
			cand := resp.Candidates[0]

			// Grounding metadata (emitted as its own chunk when present).
			if g := extractGrounding(cand.GroundingMetadata); !g.IsEmpty() {
				if !yield(llm.StreamChunk{Grounding: g}, nil) {
					return
				}
			}

			if cand.Content == nil {
				continue
			}
			for _, part := range cand.Content.Parts {
				if !emitPart(part, yield) {
					return
				}
			}
		}
	}
}

// emitPart translates one genai.Part into zero or more StreamChunks. Returns
// false if the consumer stopped iterating.
func emitPart(part *genai.Part, yield func(llm.StreamChunk, error) bool) bool {
	if part.Text != "" {
		if !yield(llm.StreamChunk{Text: part.Text, Thought: part.Thought}, nil) {
			return false
		}
	}
	if ec := part.ExecutableCode; ec != nil {
		lang := strings.ToLower(string(ec.Language))
		if lang == "" || lang == "language_unspecified" {
			lang = "python"
		}
		if !yield(llm.StreamChunk{Code: &llm.CodeExecution{
			Stage:    "code",
			Code:     ec.Code,
			Language: lang,
		}}, nil) {
			return false
		}
	}
	if r := part.CodeExecutionResult; r != nil {
		if !yield(llm.StreamChunk{Code: &llm.CodeExecution{
			Stage:   "result",
			Outcome: string(r.Outcome),
			Result:  r.Output,
		}}, nil) {
			return false
		}
	}
	return true
}

// extractGrounding maps genai grounding metadata onto the provider-agnostic
// llm.Grounding. It always returns a non-nil pointer; use IsEmpty to test.
func extractGrounding(md *genai.GroundingMetadata) *llm.Grounding {
	g := &llm.Grounding{}
	if md == nil {
		return g
	}
	g.WebSearchQueries = md.WebSearchQueries

	for _, chunk := range md.GroundingChunks {
		if chunk == nil || chunk.Web == nil {
			continue
		}
		title := chunk.Web.Title
		if title == "" {
			title = "Unknown"
		}
		g.Sources = append(g.Sources, llm.GroundingSource{
			URI:   chunk.Web.URI,
			Title: title,
		})
	}

	for _, sup := range md.GroundingSupports {
		if sup == nil {
			continue
		}
		s := llm.GroundingSupport{ChunkIndices: sup.GroundingChunkIndices}
		if sup.Segment != nil {
			s.StartIndex = sup.Segment.StartIndex
			s.EndIndex = sup.Segment.EndIndex
			s.Text = sup.Segment.Text
		}
		g.Supports = append(g.Supports, s)
	}
	return g
}
