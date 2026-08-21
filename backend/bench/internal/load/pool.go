package load

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/smartkrishi/backend/bench/internal/schema"
	"github.com/smartkrishi/backend/bench/internal/sse"
)

// Target describes the server under test and auth for the load run.
type Target struct {
	BaseURL string // e.g. http://127.0.0.1:8000
	APIV1   string // e.g. /api/v1
	Token   string // Bearer JWT for authenticated endpoints
	Client  *http.Client
}

func (t Target) url(path string) string { return t.BaseURL + t.APIV1 + path }

// RunConfig controls a single load run.
type RunConfig struct {
	Scenario    Scenario
	Concurrency int
	// Exactly one of Duration or FixedRequests drives the run. FixedRequests>0
	// runs a fixed number of requests total (used for expensive e2e agent
	// scenarios); otherwise Warmup+Measure durations apply.
	Warmup        time.Duration
	Measure       time.Duration
	FixedRequests int
	ScenarioDir   string // dir that fixtures.json 'file' paths are relative to
}

// reqResult is one request's outcome.
type reqResult struct {
	latency   time.Duration
	ok        bool
	warmup    bool
	agent     *sse.EventMetrics
	agentSecs float64 // stream wall seconds for events_per_sec
}

// Outcome aggregates a run into schema metrics.
type Outcome struct {
	HTTP  schema.HTTP
	Agent *schema.Agent // nil for non-agent scenarios
}

// Run executes the load and returns aggregated metrics. It does not sample
// resources/DB — the caller runs a Sampler concurrently around Run.
func Run(ctx context.Context, tgt Target, cfg RunConfig) (Outcome, error) {
	if tgt.Client == nil {
		tgt.Client = &http.Client{Timeout: 5 * time.Minute}
	}

	var (
		mu           sync.Mutex
		results      []reqResult
		measureStart time.Time
		measureEnd   time.Time
	)

	fixed := cfg.FixedRequests > 0

	// Warm-up boundary: requests started before warmEnd are marked warmup and
	// excluded from latency histograms (duration mode only).
	start := time.Now()
	warmEnd := start.Add(cfg.Warmup)
	if fixed {
		warmEnd = start // no separate warm-up phase in fixed-N mode
	}
	measureStart = warmEnd
	deadline := warmEnd.Add(cfg.Measure)

	// Fixed-N mode: distribute N requests across workers via a counter.
	var remaining int
	if fixed {
		remaining = cfg.FixedRequests
	}

	worker := func(idx int) func() {
		return func() {
			for {
				if fixed {
					mu.Lock()
					if remaining <= 0 {
						mu.Unlock()
						return
					}
					remaining--
					mu.Unlock()
				} else if time.Now().After(deadline) {
					return
				}
				if ctx.Err() != nil {
					return
				}
				res := doRequest(ctx, tgt, cfg, idx)
				if !fixed {
					res.warmup = time.Now().Before(warmEnd)
				}
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(idx int) { defer wg.Done(); worker(idx)() }(i)
	}
	wg.Wait()
	measureEnd = time.Now()
	if measureEnd.After(deadline) && !fixed {
		measureEnd = deadline
	}

	window := measureEnd.Sub(measureStart).Seconds()
	if fixed {
		window = measureEnd.Sub(start).Seconds()
	}
	return aggregate(results, window, cfg.Scenario.IsAgent()), nil
}

// aggregate turns raw results into schema metrics, excluding warm-up requests
// from latency/agent histograms.
func aggregate(results []reqResult, windowSecs float64, isAgent bool) Outcome {
	var (
		latencies                                        []float64
		requests                                         int
		successful                                       int
		measured                                         int
		ttfe, ttp, ttft, streamDur, evPerSec, toolCounts []float64
		toolNames                                        = map[string]int{}
		streamsDone                                      int
	)
	for _, r := range results {
		if r.warmup {
			continue
		}
		measured++
		requests++
		if r.ok {
			successful++
			latencies = append(latencies, float64(r.latency.Milliseconds()))
		}
		if isAgent && r.agent != nil {
			a := r.agent
			ttfe = append(ttfe, ms(a.TimeToFirstEvent))
			if a.TimeToPlan > 0 {
				ttp = append(ttp, ms(a.TimeToPlan))
			}
			if a.TimeToFirstToken > 0 {
				ttft = append(ttft, ms(a.TimeToFirstToken))
			}
			streamDur = append(streamDur, ms(a.StreamDuration))
			toolCounts = append(toolCounts, float64(a.ToolCallCount))
			for _, n := range a.ToolCallNames {
				toolNames[n]++
			}
			if r.agentSecs > 0 {
				evPerSec = append(evPerSec, float64(a.EventsTotal)/r.agentSecs)
			}
			if a.StreamCompleted {
				streamsDone++
			}
		}
	}

	failed := requests - successful
	var errRate, rps float64
	if requests > 0 {
		errRate = float64(failed) / float64(requests)
	}
	if windowSecs > 0 {
		rps = float64(successful) / windowSecs
	}

	out := Outcome{
		HTTP: schema.HTTP{
			Requests:   requests,
			Successful: successful,
			Failed:     failed,
			ErrorRate:  errRate,
			RPS:        rps,
			LatencyMS:  schema.StatsFromMillis(latencies),
		},
	}
	if isAgent {
		completedRate := 0.0
		if measured > 0 {
			completedRate = float64(streamsDone) / float64(measured)
		}
		out.Agent = &schema.Agent{
			TimeToFirstEventMS:   schema.StatsFromMillis(ttfe),
			TimeToPlanMS:         schema.StatsFromMillis(ttp),
			TimeToFirstTokenMS:   schema.StatsFromMillis(ttft),
			StreamDurationMS:     schema.StatsFromMillis(streamDur),
			ToolCallsPerRequest:  schema.StatsFromMillis(toolCounts),
			ToolCallNames:        toolNames,
			EventsPerSec:         schema.StatsFromMillis(evPerSec),
			StreamsCompleted:     streamsDone,
			StreamsCompletedRate: completedRate,
		}
	}
	return out
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000.0 }

// doRequest performs one request for the scenario and measures it.
func doRequest(ctx context.Context, tgt Target, cfg RunConfig, worker int) reqResult {
	switch cfg.Scenario.Endpoint {
	case EndpointHealth:
		return doHealth(ctx, tgt)
	case EndpointChatCRUD:
		return doChatCRUD(ctx, tgt)
	case EndpointUpload:
		return doUpload(ctx, tgt, cfg, worker)
	default:
		return doSendStream(ctx, tgt, cfg, worker)
	}
}

func doHealth(ctx context.Context, tgt Target) reqResult {
	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, tgt.BaseURL+"/health", nil)
	resp, err := tgt.Client.Do(req)
	if err != nil {
		return reqResult{latency: time.Since(start)}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	return reqResult{latency: time.Since(start), ok: resp.StatusCode == http.StatusOK}
}

func doChatCRUD(ctx context.Context, tgt Target) reqResult {
	start := time.Now()
	// Create a chat, then list chats. Success requires both 200.
	body := bytes.NewBufferString(`{"title":"bench"}`)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tgt.url("/chat/chats"), body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tgt.Token)
	resp, err := tgt.Client.Do(req)
	if err != nil {
		return reqResult{latency: time.Since(start)}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return reqResult{latency: time.Since(start)}
	}
	lreq, _ := http.NewRequestWithContext(ctx, http.MethodGet, tgt.url("/chat/chats?skip=0&limit=10"), nil)
	lreq.Header.Set("Authorization", "Bearer "+tgt.Token)
	lresp, err := tgt.Client.Do(lreq)
	if err != nil {
		return reqResult{latency: time.Since(start)}
	}
	io.Copy(io.Discard, lresp.Body)
	lresp.Body.Close()
	return reqResult{latency: time.Since(start), ok: lresp.StatusCode == http.StatusOK}
}

func doSendStream(ctx context.Context, tgt Target, cfg RunConfig, worker int) reqResult {
	payload := map[string]any{
		"message":      cfg.Scenario.MessageForWorker(worker),
		"include_logs": cfg.Scenario.IncludeLogs,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tgt.url("/chat/send-stream"), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+tgt.Token)
	return streamRequest(tgt, req)
}

func doUpload(ctx context.Context, tgt Target, cfg RunConfig, worker int) reqResult {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("message", cfg.Scenario.MessageForWorker(worker))
	filePath := filepath.Join(cfg.ScenarioDir, cfg.Scenario.File)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return reqResult{}
	}
	fw, _ := mw.CreateFormFile("file", filepath.Base(filePath))
	fw.Write(data)
	mw.Close()

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tgt.url("/chat/upload-and-analyze-stream"), &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+tgt.Token)
	return streamRequest(tgt, req)
}

// streamRequest sends an SSE request and parses the event stream for metrics.
func streamRequest(tgt Target, req *http.Request) reqResult {
	start := time.Now()
	resp, err := tgt.Client.Do(req)
	if err != nil {
		return reqResult{latency: time.Since(start)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return reqResult{latency: time.Since(start)}
	}
	m, perr := sse.Parse(resp.Body, start)
	lat := time.Since(start)
	res := reqResult{
		latency:   lat,
		agent:     &m,
		agentSecs: m.StreamDuration.Seconds(),
	}
	// Success = a completed stream with no error event and no read error.
	res.ok = perr == nil && m.StreamCompleted && !m.SawError
	return res
}

// ExpandConcurrency returns the concurrency levels for a mode, from fixtures.
func ExpandConcurrency(f *Fixtures, key string) []int {
	if lv, ok := f.ConcurrencyLevels[key]; ok {
		return lv
	}
	return []int{1}
}
