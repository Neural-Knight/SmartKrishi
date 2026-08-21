package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/smartkrishi/backend/bench/internal/collect"
	"github.com/smartkrishi/backend/bench/internal/harness"
	"github.com/smartkrishi/backend/bench/internal/load"
	"github.com/smartkrishi/backend/bench/internal/report"
	"github.com/smartkrishi/backend/bench/internal/schema"
)

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// runOptions holds the parsed flags for `run`.
type runOptions struct {
	mode        string
	scenario    string // scenario id/name or "all"
	concurrency string // "auto" (from fixtures) or comma-separated ints
	durationSec int
	warmupSec   int
	repetitions int
	output      string
	fixturesDir string
	repoDir     string
	agentModel  string
	// e2e target
	baseURL string
	apiV1   string
	// controlled deps
	databaseURL string
	secret      string
}

// runCmd parses flags and dispatches to the controlled or e2e orchestrator.
func runCmd(ctx context.Context, args []string) error {
	fs := newFlagSet("run")
	var o runOptions
	fs.StringVar(&o.mode, "mode", "", "controlled | e2e | e2e_no_gemini (required)")
	fs.StringVar(&o.scenario, "scenario", "all", "scenario id/name, or 'all'")
	fs.StringVar(&o.concurrency, "concurrency", "auto", "'auto' (from fixtures per mode) or comma-separated ints e.g. 1,5,10")
	fs.IntVar(&o.durationSec, "duration", 0, "measurement seconds (0 = fixtures default)")
	fs.IntVar(&o.warmupSec, "warmup", -1, "warm-up seconds (-1 = fixtures default)")
	fs.IntVar(&o.repetitions, "repetitions", 0, "repetitions per (scenario,concurrency); 0 = mode default (controlled 3, e2e 2)")
	fs.StringVar(&o.output, "output", "", "output results JSON file (required)")
	fs.StringVar(&o.fixturesDir, "fixtures-dir", "bench/scenarios", "directory containing fixtures.json")
	fs.StringVar(&o.repoDir, "repo-dir", ".", "repo dir for git metadata")
	fs.StringVar(&o.agentModel, "agent-model", envOr("AGENT_MODEL", "gemini-2.5-flash"), "agent model name recorded in results")
	fs.StringVar(&o.baseURL, "url", envOr("SMARTKRISHI_BENCH_URL", "http://127.0.0.1:8000"), "e2e: base URL of the running server")
	fs.StringVar(&o.apiV1, "api-v1", "/api/v1", "API v1 path prefix")
	fs.StringVar(&o.databaseURL, "database-url", os.Getenv("DATABASE_URL"), "controlled: Postgres URL (defaults to $DATABASE_URL)")
	fs.StringVar(&o.secret, "secret", envOr("SECRET_KEY", "bench-secret-key-at-least-32-characters-long"), "controlled: JWT signing secret")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if o.output == "" {
		return fmt.Errorf("run requires --output")
	}

	mode := schema.Mode(o.mode)
	switch mode {
	case schema.ModeControlled, schema.ModeE2E, schema.ModeE2ENoGemini:
	default:
		return fmt.Errorf("invalid --mode %q (want controlled|e2e|e2e_no_gemini)", o.mode)
	}

	fixturesPath := filepath.Join(o.fixturesDir, "fixtures.json")
	fixtures, err := load.LoadFixtures(fixturesPath)
	if err != nil {
		return err
	}

	if o.repetitions == 0 {
		o.repetitions = defaultReps(mode)
	}

	switch mode {
	case schema.ModeControlled:
		return runControlled(ctx, o, fixtures)
	default:
		return runE2E(ctx, o, fixtures, mode)
	}
}

// runControlled starts an in-process server (scripted LLM + stubbed tools, real
// Postgres) and drives the load against it.
func runControlled(ctx context.Context, o runOptions, fixtures *load.Fixtures) error {
	if o.databaseURL == "" {
		return fmt.Errorf("controlled mode requires --database-url or $DATABASE_URL")
	}
	ctrl, err := harness.NewControlled(ctx, o.databaseURL, o.secret, o.agentModel)
	if err != nil {
		return fmt.Errorf("start controlled server: %w", err)
	}
	defer ctrl.Close()

	client := &http.Client{Timeout: 5 * time.Minute}
	if _, err := harness.WaitHealthy(ctx, client, ctrl.BaseURL(), 30*time.Second); err != nil {
		return err
	}
	token, err := harness.BootstrapToken(ctx, client, ctrl.BaseURL(), o.apiV1)
	if err != nil {
		return fmt.Errorf("bootstrap token: %w", err)
	}

	tgt := load.Target{BaseURL: ctrl.BaseURL(), APIV1: o.apiV1, Token: token, Client: client}
	// Controlled mode has no /_bench hooks (in-process httptest), so DB pool is
	// read directly from the harness; resource sampling covers this process.
	suite := newSuite(o, schema.ModeControlled, ctrl.MaxConns())
	// benchBaseURL "" → sampler skips HTTP hooks; it samples this process by PID.
	if err := executeSuite(ctx, &suite, o, fixtures, tgt, "", int32(os.Getpid())); err != nil {
		return err
	}
	return finish(&suite, o)
}

// runE2E drives the load against a live server (already running with real
// dependencies). The server should be started with SMARTKRISHI_BENCH=1 so the
// sampler can read /_bench/pool and /_bench/runtime.
func runE2E(ctx context.Context, o runOptions, fixtures *load.Fixtures, mode schema.Mode) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	if _, err := harness.WaitHealthy(ctx, client, o.baseURL, 60*time.Second); err != nil {
		return fmt.Errorf("waiting for server at %s: %w", o.baseURL, err)
	}
	token, err := harness.BootstrapToken(ctx, client, o.baseURL, o.apiV1)
	if err != nil {
		return fmt.Errorf("bootstrap token: %w", err)
	}
	maxConns := harness.FetchPoolMaxConns(ctx, client, o.baseURL) // 0 if hooks off

	tgt := load.Target{BaseURL: o.baseURL, APIV1: o.apiV1, Token: token, Client: client}
	suite := newSuite(o, mode, maxConns)
	// benchBaseURL = server URL → sampler polls /_bench hooks (goroutines, pool).
	// PID is unknown for a remote/subprocess server; process CPU/RSS series stay
	// empty rather than sampling the wrong process.
	if err := executeSuite(ctx, &suite, o, fixtures, tgt, o.baseURL, 0); err != nil {
		return err
	}
	return finish(&suite, o)
}

// executeSuite runs the selected scenarios × concurrency levels × repetitions,
// appending a Result per run to the suite.
func executeSuite(ctx context.Context, suite *schema.Suite, o runOptions, fixtures *load.Fixtures, tgt load.Target, benchBaseURL string, samplePID int32) error {
	scenarios, err := selectScenarios(fixtures, o.scenario, suite.Mode)
	if err != nil {
		return err
	}
	scenarioDir := o.fixturesDir

	for _, sc := range scenarios {
		levels := concurrencyLevels(o, fixtures, suite.Mode, sc)
		for _, conc := range levels {
			for rep := 1; rep <= o.repetitions; rep++ {
				res := runOne(ctx, o, suite.Mode, tgt, sc, conc, rep, scenarioDir, benchBaseURL, samplePID, suite)
				suite.Results = append(suite.Results, res)
				fmt.Printf("  %-18s mode=%s conc=%-3d rep=%d  reqs=%d ok=%d p95=%.0fms rps=%.2f%s\n",
					sc.Name, suite.Mode, conc, rep, res.HTTP.Requests, res.HTTP.Successful,
					res.HTTP.LatencyMS.P95, res.HTTP.RPS, skippedNote(res))
			}
		}
	}
	return nil
}

// runOne executes a single (scenario, concurrency, repetition) load run with a
// resource sampler running concurrently, and assembles a schema.Result.
func runOne(ctx context.Context, o runOptions, mode schema.Mode, tgt load.Target, sc load.Scenario, conc, rep int, scenarioDir, benchBaseURL string, samplePID int32, suite *schema.Suite) schema.Result {
	warmup, measure, fixed := timing(o, mode, sc)

	res := schema.Result{
		SchemaVersion: "1",
		Timestamp:     time.Now().UTC(),
		Git:           suite.Git,
		Environment:   suite.Environment,
		Run: schema.Run{
			Mode:           mode,
			Scenario:       sc.Name,
			Concurrency:    conc,
			WarmupSeconds:  int(warmup.Seconds()),
			MeasureSeconds: int(measure.Seconds()),
			FixedRequests:  fixed,
			Repetition:     rep,
		},
		Limitations: standardLimitations(mode),
	}

	// Sampler runs for the whole load window (warmup + measure). It samples the
	// process by PID when known, and the /_bench hooks when a URL is given.
	sampler := collect.NewSampler(benchBaseURL, time.Second)
	if samplePID > 0 {
		sampler.SetPID(samplePID)
	}
	sampCtx, sampCancel := context.WithCancel(ctx)
	sampler.Start(sampCtx)

	outcome, err := load.Run(ctx, tgt, load.RunConfig{
		Scenario:      sc,
		Concurrency:   conc,
		Warmup:        warmup,
		Measure:       measure,
		FixedRequests: fixed,
		ScenarioDir:   scenarioDir,
	})
	sampCancel()

	if err != nil {
		res.Skipped = true
		res.SkippedReason = err.Error()
		return res
	}

	res.HTTP = outcome.HTTP
	res.Agent = outcome.Agent
	res.Resources = sampler.Resources()
	if db := sampler.DBDelta(); db != nil {
		res.DB = db
	} else if suite.Environment.Config.MaxDBConns > 0 {
		// Controlled mode reads max conns from the harness even without hooks.
		res.DB = &schema.DB{MaxConns: suite.Environment.Config.MaxDBConns}
	}

	// An optional scenario that produced zero successful agent streams is flagged
	// (e.g. file upload unavailable), without fabricating metrics.
	if sc.Optional && sc.IsAgent() && res.HTTP.Successful == 0 && res.HTTP.Requests > 0 {
		res.Skipped = true
		res.SkippedReason = "optional scenario produced no successful requests (dependency unavailable)"
	}
	return res
}

// finish writes the suite JSON and prints where it landed plus a short report.
func finish(suite *schema.Suite, o runOptions) error {
	if err := os.MkdirAll(filepath.Dir(o.output), 0o755); err != nil {
		return err
	}
	if err := schema.Write(o.output, suite); err != nil {
		return err
	}
	fmt.Printf("\nwrote %d result(s) to %s\n\n", len(suite.Results), o.output)
	fmt.Print(report.Render(suite))
	return nil
}

// --- helpers ---------------------------------------------------------------

func newSuite(o runOptions, mode schema.Mode, maxConns int32) schema.Suite {
	return schema.Suite{
		SchemaVersion: "1",
		Timestamp:     time.Now().UTC(),
		Git:           collect.GitMeta(o.repoDir),
		Environment:   collect.EnvMeta(o.agentModel, maxConns),
		Mode:          mode,
	}
}

// selectScenarios resolves --scenario to a concrete list, filtering to non-agent
// scenarios in e2e_no_gemini mode.
func selectScenarios(f *load.Fixtures, sel string, mode schema.Mode) ([]load.Scenario, error) {
	var out []load.Scenario
	if sel == "all" {
		for _, s := range f.Scenarios {
			if mode == schema.ModeE2ENoGemini && s.IsAgent() {
				continue
			}
			out = append(out, s)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no scenarios selected for mode %s", mode)
		}
		return out, nil
	}
	s, ok := f.Scenario(sel)
	if !ok {
		return nil, fmt.Errorf("scenario %q not found in fixtures", sel)
	}
	if mode == schema.ModeE2ENoGemini && s.IsAgent() {
		return nil, fmt.Errorf("scenario %q is an agent scenario, not allowed in e2e_no_gemini mode", sel)
	}
	return []load.Scenario{s}, nil
}

// concurrencyLevels resolves the concurrency levels for a scenario+mode. HTTP-
// only scenarios (health, chat CRUD) use the http_only ladder.
func concurrencyLevels(o runOptions, f *load.Fixtures, mode schema.Mode, sc load.Scenario) []int {
	if o.concurrency != "auto" {
		return parseInts(o.concurrency)
	}
	key := string(mode)
	if !sc.IsAgent() {
		key = "http_only"
	} else if mode == schema.ModeE2ENoGemini {
		key = "e2e"
	}
	return load.ExpandConcurrency(f, key)
}

// timing resolves (warmup, measure, fixedRequests) for a run. Expensive e2e
// agent scenarios use fixed-N requests at low concurrency instead of a duration
// to bound external API cost.
func timing(o runOptions, mode schema.Mode, sc load.Scenario) (warmup, measure time.Duration, fixed int) {
	if mode == schema.ModeE2E && sc.IsAgent() {
		return 0, 0, 10 // fixed-N; e2e_fixed_requests default
	}
	warmSecs := 30
	measSecs := 60
	if o.warmupSec >= 0 {
		warmSecs = o.warmupSec
	}
	if o.durationSec > 0 {
		measSecs = o.durationSec
	}
	return time.Duration(warmSecs) * time.Second, time.Duration(measSecs) * time.Second, 0
}

func defaultReps(mode schema.Mode) int {
	if mode == schema.ModeControlled {
		return 3
	}
	return 2
}

// standardLimitations records the documented caveats on every result so no
// consumer treats a mode's numbers as more precise than they are.
func standardLimitations(mode schema.Mode) []string {
	base := []string{
		"single instance; no load-balancer behavior",
		"auth DB lookup on every chat request is included in stream latency",
		"environment-specific; do not compare across hosts",
	}
	switch mode {
	case schema.ModeE2E:
		return append(base,
			"e2e: Gemini/tool variance dominates agent latency; indicative only, not a regression gate",
			"e2e: planner tool selection is non-deterministic; observed tool_call names are reported, not guaranteed")
	case schema.ModeControlled:
		return append(base,
			"controlled: scripted LLM + stubbed tools; measures our HTTP/DB/SSE/pipeline overhead only",
			"controlled results are not comparable to e2e")
	default:
		return append(base, "e2e_no_gemini: non-agent scenarios only")
	}
}

func skippedNote(r schema.Result) string {
	if r.Skipped {
		return "  SKIPPED: " + r.SkippedReason
	}
	return ""
}

func parseInts(csv string) []int {
	var out []int
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		out = []int{1}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
