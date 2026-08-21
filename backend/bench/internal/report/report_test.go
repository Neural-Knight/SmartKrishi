package report

import (
	"strings"
	"testing"
	"time"

	"github.com/smartkrishi/backend/bench/internal/schema"
)

func TestRenderContainsLabels(t *testing.T) {
	s := &schema.Suite{
		SchemaVersion: schema.SchemaVersion,
		Timestamp:     time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC),
		Git:           schema.Git{Commit: "abc1234", Branch: "main", Dirty: true},
		Environment: schema.Environment{
			GoVersion: "go1.25.0", OS: "darwin", Arch: "arm64",
			CPUModel: "Apple M3", NumCPU: 8,
			Config: schema.EnvConfig{AgentModel: "gemini-2.0", MaxDBConns: 10},
		},
		Mode: schema.ModeControlled,
		Results: []schema.Result{
			{
				Run: schema.Run{
					Mode: schema.ModeControlled, Scenario: "chat_stream",
					Concurrency: 4, Repetition: 1,
				},
				HTTP: schema.HTTP{
					Requests: 100, Successful: 99, Failed: 1,
					ErrorRate: 0.01, RPS: 42.5,
					LatencyMS: schema.Stats{P50: 10, P95: 20, P99: 30, Max: 40},
				},
				Agent: &schema.Agent{
					TimeToFirstEventMS: schema.Stats{P50: 5, P95: 9},
					TimeToFirstTokenMS: schema.Stats{P50: 6, P95: 11},
					StreamDurationMS:   schema.Stats{P50: 100, P95: 200},
					StreamsCompleted:   50, StreamsCompletedRate: 0.98,
					ToolCallNames: map[string]int{"weather": 3, "market": 2},
				},
				Resources: schema.Resources{
					CPUPercent:  schema.MinAvgMax{Min: 1, Avg: 5, Max: 12},
					MemoryRSSMB: schema.MinAvgMax{Min: 50, Avg: 60, Max: 80},
					Goroutines:  schema.MinAvgMax{Min: 10, Avg: 20, Max: 30},
					Samples:     15,
				},
				DB: &schema.DB{
					MaxConns: 10, AcquiredConnsMax: 6, IdleConnsMin: 1,
					TotalConnsMax: 8, EmptyAcquireCountDelta: 2,
				},
				Limitations: []string{"scripted LLM", "stubbed tools"},
			},
		},
	}

	out := Render(s)
	for _, want := range []string{
		"mode:", "controlled", "abc1234", "main", "[dirty]",
		"go1.25.0", "gemini-2.0", "max_db_conns",
		"chat_stream", "http.rps", "http.latency_ms.p95",
		"agent.time_to_first_event_ms.p95", "agent.stream_duration_ms.p95",
		"agent.streams_completed_rate", "market=2, weather=3",
		"resources.cpu_percent", "resources.memory_rss_mb",
		"db.acquired_conns_max", "Limitations:", "scripted LLM",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Render output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderSkippedAndNoLimitations(t *testing.T) {
	s := &schema.Suite{
		Mode: schema.ModeE2E,
		Results: []schema.Result{
			{
				Run:           schema.Run{Mode: schema.ModeE2E, Scenario: "agent_chat"},
				Skipped:       true,
				SkippedReason: "no gemini key",
			},
		},
	}
	out := Render(s)
	if !strings.Contains(out, "SKIPPED: no gemini key") {
		t.Errorf("expected skipped reason, got:\n%s", out)
	}
	if !strings.Contains(out, "(none)") {
		t.Errorf("expected empty limitations placeholder, got:\n%s", out)
	}
}
