package compare

import (
	"strings"
	"testing"

	"github.com/smartkrishi/backend/bench/internal/schema"
)

func suiteWith(mode schema.Mode, r schema.Result) *schema.Suite {
	return &schema.Suite{Mode: mode, Results: []schema.Result{r}}
}

func result(scenario string, conc int, mode schema.Mode) schema.Result {
	return schema.Result{
		Run: schema.Run{Mode: mode, Scenario: scenario, Concurrency: conc},
	}
}

func TestCompareFlagsLatencyRegression(t *testing.T) {
	base := result("chat", 4, schema.ModeControlled)
	base.HTTP = schema.HTTP{RPS: 100, ErrorRate: 0.0, LatencyMS: schema.Stats{P95: 100}}

	cur := result("chat", 4, schema.ModeControlled)
	cur.HTTP = schema.HTTP{RPS: 100, ErrorRate: 0.0, LatencyMS: schema.Stats{P95: 130}} // +30%

	out, err := Compare(suiteWith(schema.ModeControlled, base), suiteWith(schema.ModeControlled, cur))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "REGRESSION") {
		t.Errorf("expected REGRESSION flag for +30%% p95, got:\n%s", out)
	}
	if !strings.Contains(out, "+30.0%") {
		t.Errorf("expected +30.0%% change, got:\n%s", out)
	}
}

func TestCompareMismatchedModesError(t *testing.T) {
	base := result("chat", 4, schema.ModeControlled)
	cur := result("chat", 4, schema.ModeE2E)
	_, err := Compare(suiteWith(schema.ModeControlled, base), suiteWith(schema.ModeE2E, cur))
	if err == nil {
		t.Fatal("expected error comparing controlled to e2e, got nil")
	}
}

func TestCompareZeroBaselinePrintsNA(t *testing.T) {
	base := result("chat", 4, schema.ModeControlled)
	base.HTTP = schema.HTTP{RPS: 0, LatencyMS: schema.Stats{P95: 0}} // zero baseline

	cur := result("chat", 4, schema.ModeControlled)
	cur.HTTP = schema.HTTP{RPS: 50, LatencyMS: schema.Stats{P95: 20}}

	out, err := Compare(suiteWith(schema.ModeControlled, base), suiteWith(schema.ModeControlled, cur))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "n/a") {
		t.Errorf("expected n/a for zero-baseline metric, got:\n%s", out)
	}
	// A zero baseline must never be flagged as a regression.
	if strings.Contains(out, "REGRESSION") {
		t.Errorf("zero baseline should not be flagged REGRESSION, got:\n%s", out)
	}
}

func TestCompareErrorRateRegression(t *testing.T) {
	base := result("chat", 2, schema.ModeE2E)
	base.HTTP = schema.HTTP{RPS: 10, ErrorRate: 0.00, LatencyMS: schema.Stats{P95: 10}}

	cur := result("chat", 2, schema.ModeE2E)
	cur.HTTP = schema.HTTP{RPS: 10, ErrorRate: 0.05, LatencyMS: schema.Stats{P95: 10}} // +5pp

	out, err := Compare(suiteWith(schema.ModeE2E, base), suiteWith(schema.ModeE2E, cur))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "REGRESSION") {
		t.Errorf("expected REGRESSION for error_rate +5pp, got:\n%s", out)
	}
}
