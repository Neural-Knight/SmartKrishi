// Package gemini is the single production implementation of llm.Provider,
// backed by google.golang.org/genai. Gemini-specific concerns (client pool,
// native tool wiring, genai<->llm type mapping) stay explicit here; the agent
// nodes never import this package directly, only llm.Provider.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"google.golang.org/genai"
)

// clientPool manages one *genai.Client per chat id.
//
// Per-chat client isolation exists because files uploaded through a client are
// only accessible via that same client, so keeping a stable client per chat is
// required for file access. For text-only generation the isolation is harmless
// — every chat simply shares the same API key.
//
// The pool also holds a file registry: a JSON record of uploads (chat -> Gemini
// file id + local path) so an expired uploaded file (48h TTL) can be
// re-uploaded from the local copy. The registry path defaults to
// data/gemini_files_registry.json and is best-effort — a failure to load or
// save never blocks generation.
type clientPool struct {
	apiKey string

	mu      sync.Mutex
	clients map[string]*genai.Client

	regMu    sync.Mutex
	regPath  string
	registry map[string]fileRegistryEntry // keyed by Gemini file id
}

// fileRegistryEntry records enough to re-upload an expired uploaded file.
type fileRegistryEntry struct {
	FileID    string `json:"file_id"`
	ChatID    string `json:"chat_id"`
	LocalPath string `json:"local_path"`
	MIMEType  string `json:"mime_type"`
	URI       string `json:"uri"`
}

func newClientPool(apiKey string) *clientPool {
	p := &clientPool{
		apiKey:   apiKey,
		clients:  make(map[string]*genai.Client),
		regPath:  "data/gemini_files_registry.json",
		registry: make(map[string]fileRegistryEntry),
	}
	p.loadRegistry()
	return p
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

// recordUpload adds/updates a registry entry and persists it (best-effort).
func (p *clientPool) recordUpload(e fileRegistryEntry) {
	p.regMu.Lock()
	p.registry[e.FileID] = e
	p.regMu.Unlock()
	p.saveRegistry()
}

// lookup returns the registry entry for a Gemini file id, if known.
func (p *clientPool) lookup(fileID string) (fileRegistryEntry, bool) {
	p.regMu.Lock()
	defer p.regMu.Unlock()
	e, ok := p.registry[fileID]
	return e, ok
}

func (p *clientPool) loadRegistry() {
	b, err := os.ReadFile(p.regPath)
	if err != nil {
		return // no registry yet — fine
	}
	var reg map[string]fileRegistryEntry
	if err := json.Unmarshal(b, &reg); err != nil {
		return
	}
	p.regMu.Lock()
	p.registry = reg
	p.regMu.Unlock()
}

func (p *clientPool) saveRegistry() {
	p.regMu.Lock()
	b, err := json.MarshalIndent(p.registry, "", "  ")
	p.regMu.Unlock()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p.regPath), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(p.regPath, b, 0o644)
}
