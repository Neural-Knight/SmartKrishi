// Package schema defines the benchmark result JSON structure and helpers.
// All benchmark result files use these types so run/compare/report share one
// contract. Numeric fields are populated only from real runs; this package
// never fabricates measurements.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// SchemaVersion is bumped when the result JSON shape changes incompatibly.
const SchemaVersion = "1"

// Mode labels how externals were handled during a run. Results of different
// modes are never compared to each other.
type Mode string

const (
	// ModeE2E runs against a live server with real Gemini/Postgres/tool APIs.
	ModeE2E Mode = "e2e"
	// ModeControlled runs against an in-process server with a scripted LLM and
	// stubbed tool HTTP endpoints (real Postgres), isolating our own overhead.
	ModeControlled Mode = "controlled"
	// ModeE2ENoGemini runs only non-agent scenarios (health, chat CRUD) with no
	// Gemini key configured.
	ModeE2ENoGemini Mode = "e2e_no_gemini"
)

// Result is one benchmark run for a single (mode, scenario, concurrency,
// repetition). It is the top-level object written to results/*.json.
type Result struct {
	SchemaVersion string      `json:"schema_version"`
	Timestamp     time.Time   `json:"timestamp"`
	Git           Git         `json:"git"`
	Environment   Environment `json:"environment"`
	Run           Run         `json:"run"`
	HTTP          HTTP        `json:"http"`
	Agent         *Agent      `json:"agent,omitempty"`
	Resources     Resources   `json:"resources"`
	DB            *DB         `json:"db,omitempty"`
	Limitations   []string    `json:"limitations"`
	Skipped       bool        `json:"skipped"`
	SkippedReason string      `json:"skipped_reason,omitempty"`
}

// Git captures the repository state at run time.
type Git struct {
	Commit string `json:"commit"`
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
}

// Environment captures the host and relevant server config.
type Environment struct {
	GoVersion  string    `json:"go_version"`
	OS         string    `json:"os"`
	Arch       string    `json:"arch"`
	CPUModel   string    `json:"cpu_model"`
	NumCPU     int       `json:"num_cpu"`
	MemTotalMB int64     `json:"mem_total_mb"`
	Config     EnvConfig `json:"config"`
}

// EnvConfig records the server settings that affect performance.
type EnvConfig struct {
	AgentModel string `json:"agent_model"`
	MaxDBConns int32  `json:"max_db_conns"`
}

// Run identifies the scenario/mode/load parameters of this result.
type Run struct {
	Mode           Mode   `json:"mode"`
	Scenario       string `json:"scenario"`
	Concurrency    int    `json:"concurrency"`
	WarmupSeconds  int    `json:"warmup_seconds"`
	MeasureSeconds int    `json:"measure_seconds"`
	FixedRequests  int    `json:"fixed_requests,omitempty"` // set instead of MeasureSeconds for N-request runs
	Repetition     int    `json:"repetition"`
}

// Stats is a latency/percentile summary in milliseconds (or the metric's unit).
type Stats struct {
	P50    float64 `json:"p50"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
	Max    float64 `json:"max"`
	StdDev float64 `json:"stddev"`
}

// HTTP holds request-level outcome and latency metrics.
type HTTP struct {
	Requests   int     `json:"requests"`
	Successful int     `json:"successful"`
	Failed     int     `json:"failed"`
	ErrorRate  float64 `json:"error_rate"`
	RPS        float64 `json:"rps"`
	LatencyMS  Stats   `json:"latency_ms"`
}

// Agent holds SSE/agent-phase metrics derived client-side from the event stream.
type Agent struct {
	TimeToFirstEventMS   Stats          `json:"time_to_first_event_ms"`
	TimeToPlanMS         Stats          `json:"time_to_plan_ms"`
	TimeToFirstTokenMS   Stats          `json:"time_to_first_token_ms"`
	StreamDurationMS     Stats          `json:"stream_duration_ms"`
	ToolCallsPerRequest  Stats          `json:"tool_calls_per_request"`
	ToolCallNames        map[string]int `json:"tool_call_names"`
	EventsPerSec         Stats          `json:"events_per_sec"`
	StreamsCompleted     int            `json:"streams_completed"`
	StreamsCompletedRate float64        `json:"streams_completed_rate"`
}

// Resources holds sampled host resource usage over the measurement window.
type Resources struct {
	CPUPercent  MinAvgMax `json:"cpu_percent"`
	MemoryRSSMB MinAvgMax `json:"memory_rss_mb"`
	Goroutines  MinAvgMax `json:"goroutines"`
	Samples     int       `json:"samples"`
}

// MinAvgMax summarizes a sampled series.
type MinAvgMax struct {
	Min float64 `json:"min"`
	Avg float64 `json:"avg"`
	Max float64 `json:"max"`
}

// DB holds pool statistics sampled from the /_bench/pool hook.
type DB struct {
	MaxConns                int32 `json:"max_conns"`
	AcquiredConnsMax        int32 `json:"acquired_conns_max"`
	IdleConnsMin            int32 `json:"idle_conns_min"`
	TotalConnsMax           int32 `json:"total_conns_max"`
	EmptyAcquireCountDelta  int64 `json:"empty_acquire_count_delta"`
	EmptyAcquireWaitMSDelta int64 `json:"empty_acquire_wait_ms_delta"`
}

// --- percentile helpers -----------------------------------------------------

// StatsFromMillis computes percentile stats from a slice of millisecond
// durations. Returns a zero Stats for an empty input (no fabricated values).
func StatsFromMillis(vals []float64) Stats {
	if len(vals) == 0 {
		return Stats{}
	}
	sorted := append([]float64(nil), vals...)
	sort.Float64s(sorted)
	return Stats{
		P50:    percentile(sorted, 50),
		P95:    percentile(sorted, 95),
		P99:    percentile(sorted, 99),
		Max:    sorted[len(sorted)-1],
		StdDev: stddev(vals),
	}
}

// percentile returns the p-th percentile (0-100) of a pre-sorted slice using
// nearest-rank.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := int(float64(len(sorted))*p/100.0 + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var sq float64
	for _, v := range vals {
		d := v - mean
		sq += d * d
	}
	return sqrt(sq / float64(len(vals)-1))
}

// sqrt avoids importing math for a single call while staying correct.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 40; i++ {
		z = 0.5 * (z + x/z)
	}
	return z
}

// Median returns the median of a value set (used to aggregate repetitions).
func Median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// --- IO ---------------------------------------------------------------------

// Write marshals a value as indented JSON to path.
func Write(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadResult loads a single Result from a JSON file.
func ReadResult(path string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &r, nil
}

// Suite is a collection of results written by a single `run` invocation.
type Suite struct {
	SchemaVersion string      `json:"schema_version"`
	Timestamp     time.Time   `json:"timestamp"`
	Git           Git         `json:"git"`
	Environment   Environment `json:"environment"`
	Mode          Mode        `json:"mode"`
	Results       []Result    `json:"results"`
}

// ReadSuite loads a Suite from a JSON file.
func ReadSuite(path string) (*Suite, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Suite
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("decode suite %s: %w", path, err)
	}
	return &s, nil
}
