package agent

import "github.com/smartkrishi/backend/internal/agent/llm"

// mergeGrounding accumulates streamed grounding deltas into dst. The Gemini
// stream re-sends grounding metadata as it refines; we keep the latest
// non-empty values for each sub-part (queries/sources/supports), matching the
// Python code which overwrites grounding_metadata on each chunk that carries it.
func mergeGrounding(dst, src *llm.Grounding) {
	if src == nil {
		return
	}
	if len(src.WebSearchQueries) > 0 {
		dst.WebSearchQueries = src.WebSearchQueries
	}
	if len(src.Sources) > 0 {
		dst.Sources = src.Sources
	}
	if len(src.Supports) > 0 {
		dst.Supports = src.Supports
	}
}

// groundingStreamEvents converts a grounding delta into the ordered discrete
// grounding_* events the frontend consumes (queries, chunks, supports). Empty
// sub-parts are skipped.
func groundingStreamEvents(g *llm.Grounding) []Event {
	if g == nil || g.IsEmpty() {
		return nil
	}
	var evs []Event
	if len(g.WebSearchQueries) > 0 {
		evs = append(evs, Event{Type: EventGroundingWebSearchQuery, Queries: g.WebSearchQueries})
	}
	if len(g.Sources) > 0 {
		evs = append(evs, Event{Type: EventGroundingChunks, Sources: sourceEvents(g.Sources)})
	}
	if len(g.Supports) > 0 {
		evs = append(evs, Event{Type: EventGroundingSupports, Supports: supportEvents(g.Supports)})
	}
	return evs
}

// serializeGrounding builds the grounding_metadata block for the final response
// event. Returns nil when there is no grounding.
func serializeGrounding(g *llm.Grounding) *GroundingMetadata {
	if g == nil || g.IsEmpty() {
		return nil
	}
	return &GroundingMetadata{
		WebSearchQueries: g.WebSearchQueries,
		GroundingChunks:  sourceEvents(g.Sources),
		GroundingSupport: supportEvents(g.Supports),
	}
}

func sourceEvents(in []llm.GroundingSource) []GroundingSourceEvent {
	if len(in) == 0 {
		return nil
	}
	out := make([]GroundingSourceEvent, 0, len(in))
	for _, s := range in {
		out = append(out, GroundingSourceEvent{URI: s.URI, Title: s.Title})
	}
	return out
}

func supportEvents(in []llm.GroundingSupport) []GroundingSupportEvent {
	if len(in) == 0 {
		return nil
	}
	out := make([]GroundingSupportEvent, 0, len(in))
	for _, s := range in {
		out = append(out, GroundingSupportEvent{
			Segment:             GroundingSegment{StartIndex: s.StartIndex, EndIndex: s.EndIndex, Text: s.Text},
			GroundingChunkIndex: s.ChunkIndices,
		})
	}
	return out
}
