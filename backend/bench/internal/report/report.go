// Package report renders a human-readable text summary of a benchmark Suite.
// It only prints values already present in the Suite (a 0 stays 0) and never
// derives or fabricates metrics.
package report

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/smartkrishi/backend/bench/internal/schema"
)

// Render returns a human-readable text report for the suite.
func Render(s *schema.Suite) string {
	var b strings.Builder

	fmt.Fprintf(&b, "SmartKrishi benchmark report\n")
	fmt.Fprintf(&b, "============================\n")
	fmt.Fprintf(&b, "mode:        %s\n", s.Mode)
	fmt.Fprintf(&b, "timestamp:   %s\n", s.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "git:         %s (%s)%s\n", s.Git.Commit, s.Git.Branch, dirtySuffix(s.Git.Dirty))
	fmt.Fprintf(&b, "go/os/arch:  %s %s/%s\n", s.Environment.GoVersion, s.Environment.OS, s.Environment.Arch)
	fmt.Fprintf(&b, "cpu:         %s (%d cores)\n", s.Environment.CPUModel, s.Environment.NumCPU)
	fmt.Fprintf(&b, "agent_model: %s\n", s.Environment.Config.AgentModel)
	fmt.Fprintf(&b, "max_db_conns:%d\n", s.Environment.Config.MaxDBConns)
	fmt.Fprintf(&b, "results:     %d\n\n", len(s.Results))

	for i := range s.Results {
		renderResult(&b, &s.Results[i])
	}
	return b.String()
}

func dirtySuffix(dirty bool) string {
	if dirty {
		return " [dirty]"
	}
	return ""
}

func renderResult(b *strings.Builder, r *schema.Result) {
	fmt.Fprintf(b, "--- %s / %s / c=%d / rep=%d ---\n",
		r.Run.Mode, r.Run.Scenario, r.Run.Concurrency, r.Run.Repetition)

	if r.Skipped {
		fmt.Fprintf(b, "SKIPPED: %s\n", nonEmpty(r.SkippedReason, "(no reason given)"))
	}

	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)

	// HTTP metrics.
	fmt.Fprintf(tw, "http.requests\t%d\n", r.HTTP.Requests)
	fmt.Fprintf(tw, "http.successful\t%d\n", r.HTTP.Successful)
	fmt.Fprintf(tw, "http.failed\t%d\n", r.HTTP.Failed)
	fmt.Fprintf(tw, "http.error_rate\t%.4f\n", r.HTTP.ErrorRate)
	fmt.Fprintf(tw, "http.rps\t%.2f\n", r.HTTP.RPS)
	fmt.Fprintf(tw, "http.latency_ms.p50\t%.2f\n", r.HTTP.LatencyMS.P50)
	fmt.Fprintf(tw, "http.latency_ms.p95\t%.2f\n", r.HTTP.LatencyMS.P95)
	fmt.Fprintf(tw, "http.latency_ms.p99\t%.2f\n", r.HTTP.LatencyMS.P99)
	fmt.Fprintf(tw, "http.latency_ms.max\t%.2f\n", r.HTTP.LatencyMS.Max)

	// Agent metrics (only when present).
	if r.Agent != nil {
		a := r.Agent
		fmt.Fprintf(tw, "agent.time_to_first_event_ms.p50\t%.2f\n", a.TimeToFirstEventMS.P50)
		fmt.Fprintf(tw, "agent.time_to_first_event_ms.p95\t%.2f\n", a.TimeToFirstEventMS.P95)
		fmt.Fprintf(tw, "agent.time_to_first_token_ms.p50\t%.2f\n", a.TimeToFirstTokenMS.P50)
		fmt.Fprintf(tw, "agent.time_to_first_token_ms.p95\t%.2f\n", a.TimeToFirstTokenMS.P95)
		fmt.Fprintf(tw, "agent.stream_duration_ms.p50\t%.2f\n", a.StreamDurationMS.P50)
		fmt.Fprintf(tw, "agent.stream_duration_ms.p95\t%.2f\n", a.StreamDurationMS.P95)
		fmt.Fprintf(tw, "agent.streams_completed\t%d\n", a.StreamsCompleted)
		fmt.Fprintf(tw, "agent.streams_completed_rate\t%.4f\n", a.StreamsCompletedRate)
		fmt.Fprintf(tw, "agent.tool_call_names\t%s\n", formatToolCalls(a.ToolCallNames))
	}

	// Resources.
	fmt.Fprintf(tw, "resources.cpu_percent avg/max\t%.2f / %.2f\n", r.Resources.CPUPercent.Avg, r.Resources.CPUPercent.Max)
	fmt.Fprintf(tw, "resources.memory_rss_mb avg/max\t%.2f / %.2f\n", r.Resources.MemoryRSSMB.Avg, r.Resources.MemoryRSSMB.Max)
	fmt.Fprintf(tw, "resources.goroutines avg/max\t%.2f / %.2f\n", r.Resources.Goroutines.Avg, r.Resources.Goroutines.Max)
	fmt.Fprintf(tw, "resources.samples\t%d\n", r.Resources.Samples)

	// DB pool.
	if r.DB != nil {
		fmt.Fprintf(tw, "db.acquired_conns_max\t%d\n", r.DB.AcquiredConnsMax)
		fmt.Fprintf(tw, "db.total_conns_max\t%d\n", r.DB.TotalConnsMax)
		fmt.Fprintf(tw, "db.idle_conns_min\t%d\n", r.DB.IdleConnsMin)
		fmt.Fprintf(tw, "db.max_conns\t%d\n", r.DB.MaxConns)
		fmt.Fprintf(tw, "db.empty_acquire_count_delta\t%d\n", r.DB.EmptyAcquireCountDelta)
		fmt.Fprintf(tw, "db.empty_acquire_wait_ms_delta\t%d\n", r.DB.EmptyAcquireWaitMSDelta)
	}
	tw.Flush()

	// Limitations are always printed.
	fmt.Fprintf(b, "Limitations:\n")
	if len(r.Limitations) == 0 {
		fmt.Fprintf(b, "  (none)\n")
	} else {
		for _, l := range r.Limitations {
			fmt.Fprintf(b, "  - %s\n", l)
		}
	}
	fmt.Fprintf(b, "\n")
}

// formatToolCalls renders the tool-call name counts in a stable order.
func formatToolCalls(m map[string]int) string {
	if len(m) == 0 {
		return "(none)"
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", n, m[n]))
	}
	return strings.Join(parts, ", ")
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// WriteFile renders the suite and writes it to path.
func WriteFile(path string, s *schema.Suite) error {
	return os.WriteFile(path, []byte(Render(s)), 0o644)
}
