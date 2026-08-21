// Package load defines benchmark scenarios and the worker pool that drives them
// against a running server, collecting HTTP and agent-stream metrics.
package load

import (
	"encoding/json"
	"fmt"
	"os"
)

// Endpoint identifies which server endpoint a scenario exercises.
type Endpoint string

const (
	EndpointSendStream Endpoint = "send-stream"
	EndpointUpload     Endpoint = "upload-and-analyze-stream"
	EndpointHealth     Endpoint = "health"
	EndpointChatCRUD   Endpoint = "chat-crud"
)

// Scenario is one fixed benchmark input, loaded from fixtures.json.
type Scenario struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Endpoint       Endpoint `json:"endpoint"`
	Message        string   `json:"message"`
	IncludeLogs    bool     `json:"include_logs"`
	File           string   `json:"file"`
	ExpectedTools  []string `json:"expected_tools"`
	RotateMessages []string `json:"rotate_messages"`
	Optional       bool     `json:"optional"`
	Notes          string   `json:"notes"`
}

// MessageForWorker returns the prompt for a given worker index. Scenarios with
// rotate_messages cycle by worker index; otherwise the fixed Message is used.
func (s Scenario) MessageForWorker(worker int) string {
	if len(s.RotateMessages) > 0 {
		return s.RotateMessages[worker%len(s.RotateMessages)]
	}
	return s.Message
}

// IsAgent reports whether the scenario exercises the agent/SSE stream (vs a
// plain HTTP endpoint like health or chat CRUD).
func (s Scenario) IsAgent() bool {
	return s.Endpoint == EndpointSendStream || s.Endpoint == EndpointUpload
}

// Defaults holds the default timing knobs from fixtures.json.
type Defaults struct {
	WarmupSeconds   int `json:"warmup_seconds"`
	MeasureSeconds  int `json:"measure_seconds"`
	CooldownSeconds int `json:"cooldown_seconds"`
}

// Fixtures is the parsed fixtures.json.
type Fixtures struct {
	Version           string           `json:"version"`
	Scenarios         []Scenario       `json:"scenarios"`
	ConcurrencyLevels map[string][]int `json:"concurrency_levels"`
	Defaults          Defaults         `json:"defaults"`
	E2EFixedRequests  E2EFixedRequests `json:"e2e_fixed_requests"`
}

// E2EFixedRequests configures the fixed-N request mode for expensive e2e
// agent scenarios (to avoid burning quota with duration-based load).
type E2EFixedRequests struct {
	Requests    int `json:"requests"`
	Concurrency int `json:"concurrency"`
}

// LoadFixtures reads and parses fixtures.json.
func LoadFixtures(path string) (*Fixtures, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f Fixtures
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse fixtures %s: %w", path, err)
	}
	return &f, nil
}

// Scenario looks up a scenario by id or name; returns false if not found.
func (f *Fixtures) Scenario(idOrName string) (Scenario, bool) {
	for _, s := range f.Scenarios {
		if s.ID == idOrName || s.Name == idOrName {
			return s, true
		}
	}
	return Scenario{}, false
}
