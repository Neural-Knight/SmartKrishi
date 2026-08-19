// Package gemini is the single production implementation of llm.Provider,
// backed by google.golang.org/genai. Gemini-specific concerns (client pool,
// native tool wiring, genai<->llm type mapping) stay explicit here; the agent
// nodes never import this package directly, only llm.Provider.
package gemini

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/genai"
)

// clientPool manages one *genai.Client per chat id.
//
// This mirrors the Python client_manager's per-chat client isolation: files
// uploaded through a client are only accessible via that same client, so
// keeping a stable client per chat is a prerequisite for the File API work in
// Step 9. For text-only generation (Step 6) the isolation is harmless — every
// chat simply shares the same API key.
//
// The Python file registry (JSON persistence of uploads) is intentionally NOT
// ported here; it belongs with the file tools in Step 9.
type clientPool struct {
	apiKey string

	mu      sync.Mutex
	clients map[string]*genai.Client
}

func newClientPool(apiKey string) *clientPool {
	return &clientPool{
		apiKey:  apiKey,
		clients: make(map[string]*genai.Client),
	}
}

// get returns the client for chatID, creating it on first use. An empty chatID
// is valid and maps to a shared default client (used by stateless calls such as
// the planner). The returned client must not be closed by callers.
func (p *clientPool) get(ctx context.Context, chatID string) (*genai.Client, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("gemini: no API key configured (set GEMINI_API_KEY)")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if c, ok := p.clients[chatID]; ok {
		return c, nil
	}

	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  p.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("gemini: create client for chat %q: %w", chatID, err)
	}
	p.clients[chatID] = c
	return c, nil
}
