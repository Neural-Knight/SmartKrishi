// Package compare diffs two benchmark suites and flags regressions. It only
// compares results of the same Mode (never e2e vs controlled) and only computes
// a percent change when both values are > 0.
package compare

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/smartkrishi/backend/bench/internal/schema"
)

// Regression thresholds.
const (
	latencyRegressPct   = 10.0 // p95 latency increase > 10%
	rpsRegressPct       = 10.0 // rps drop > 10%
	errorRateRegressAbs = 0.01 // error_rate increase > 1 percentage point
)

// Compare matches results by (mode, scenario, concurrency) present in both
// suites and renders a per-metric before/after/change table. It returns an error
// when the two suites have different modes.
func Compare(baseline, current *schema.Suite) (string, error) {
	if baseline.Mode != current.Mode {
		return "", fmt.Errorf("cannot compare suites of different modes: baseline=%q current=%q (never compare e2e to controlled)",
			baseline.Mode, current.Mode)
	}

	baseIdx := indexResults(baseline)
	curIdx := indexResults(current)

	// Deterministic ordering over the intersection of keys.
	keys := make([]resultKey, 0, len(baseIdx))
	for k := range baseIdx {
		if _, ok := curIdx[k]; ok {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].scenario != keys[j].scenario {
			return keys[i].scenario < keys[j].scenario
		}
		return keys[i].concurrency < keys[j].concurrency
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Benchmark comparison (mode=%s)\n", baseline.Mode)
	fmt.Fprintf(&b, "baseline: %s (%s)\n", baseline.Git.Commit, baseline.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "current:  %s (%s)\n", current.Git.Commit, current.Timestamp.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Fprintf(&b, "matched results: %d\n\n", len(keys))

	if len(keys) == 0 {
		fmt.Fprintf(&b, "(no matching (mode, scenario, concurrency) pairs)\n")
		return b.String(), nil
	}

	for _, k := range keys {
		before := baseIdx[k]
		after := curIdx[k]
		renderPair(&b, k, before, after)
	}
	return b.String(), nil
}

type resultKey struct {
	mode        schema.Mode
	scenario    string
	concurrency int
}

func indexResults(s *schema.Suite) map[resultKey]*schema.Result {
	m := make(map[resultKey]*schema.Result, len(s.Results))
	for i := range s.Results {
		r := &s.Results[i]
		k := resultKey{mode: r.Run.Mode, scenario: r.Run.Scenario, concurrency: r.Run.Concurrency}
		// First occurrence wins for a stable comparison.
		if _, ok := m[k]; !ok {
			m[k] = r
		}
	}
	return m
}

func renderPair(b *strings.Builder, k resultKey, before, after *schema.Result) {
	fmt.Fprintf(b, "--- %s / %s / c=%d ---\n", k.mode, k.scenario, k.concurrency)

	tw := tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "Metric\tBefore\tAfter\tChange\n")

	// http.latency_ms.p95 — regression when increase > 10%.
	writeRow(tw, "http.latency_ms.p95",
		before.HTTP.LatencyMS.P95, after.HTTP.LatencyMS.P95,
		regressLatency(before.HTTP.LatencyMS.P95, after.HTTP.LatencyMS.P95))

	// http.rps — regression when it drops > 10%.
	writeRow(tw, "http.rps",
		before.HTTP.RPS, after.HTTP.RPS,
		regressRPS(before.HTTP.RPS, after.HTTP.RPS))

	// http.error_rate — regression when it increases by > 0.01 absolute.
	writeRow(tw, "http.error_rate",
		before.HTTP.ErrorRate, after.HTTP.ErrorRate,
		after.HTTP.ErrorRate-before.HTTP.ErrorRate > errorRateRegressAbs)

	// agent.stream_duration_ms.p95 — regression when increase > 10%.
	bStream, aStream := agentStreamP95(before), agentStreamP95(after)
	writeRow(tw, "agent.stream_duration_ms.p95", bStream, aStream, regressLatency(bStream, aStream))

	// resources.memory_rss_mb.max — informational (no regression flag).
	writeRow(tw, "resources.memory_rss_mb.max",
		before.Resources.MemoryRSSMB.Max, after.Resources.MemoryRSSMB.Max, false)

	tw.Flush()
	fmt.Fprintf(b, "\n")
}

func agentStreamP95(r *schema.Result) float64 {
	if r.Agent == nil {
		return 0
	}
	return r.Agent.StreamDurationMS.P95
}

// writeRow prints one metric row with a percent change (or "n/a" when a percent
// cannot be computed) and an optional REGRESSION flag.
func writeRow(tw *tabwriter.Writer, name string, before, after float64, regression bool) {
	change := percentChange(before, after)
	flag := ""
	if regression {
		flag = "  REGRESSION"
	}
	fmt.Fprintf(tw, "%s\t%.4g\t%.4g\t%s%s\n", name, before, after, change, flag)
}

// percentChange returns a signed percent string only when both values are > 0,
// else "n/a".
func percentChange(before, after float64) string {
	if before <= 0 || after <= 0 {
		return "n/a"
	}
	pct := (after - before) / before * 100.0
	return fmt.Sprintf("%+.1f%%", pct)
}

// regressLatency reports whether after is > 10% higher than before (both > 0).
func regressLatency(before, after float64) bool {
	if before <= 0 || after <= 0 {
		return false
	}
	return (after-before)/before*100.0 > latencyRegressPct
}

// regressRPS reports whether after dropped > 10% below before (both > 0).
func regressRPS(before, after float64) bool {
	if before <= 0 || after <= 0 {
		return false
	}
	return (before-after)/before*100.0 > rpsRegressPct
}

// WriteFile renders the comparison and writes it to path.
func WriteFile(path string, baseline, current *schema.Suite) error {
	out, err := Compare(baseline, current)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}
