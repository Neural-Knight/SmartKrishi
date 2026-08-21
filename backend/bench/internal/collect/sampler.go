package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/smartkrishi/backend/bench/internal/schema"
)

// Sampler polls host + server resources during a measurement window. It samples
// three series (goroutines, CPU%, RSS MB) plus DB pool snapshots on a fixed
// interval and reduces them to min/avg/max at the end of the run. It never
// fabricates values: a series with no samples yields a zero MinAvgMax, and the
// DB delta is nil when no pool reads succeeded.
type Sampler struct {
	baseURL  string
	interval time.Duration
	client   *http.Client

	mu sync.Mutex

	// series accumulators
	goroutines []float64
	cpuPercent []float64
	rssMB      []float64

	// DB pool tracking (min/max/first/last).
	firstPool *poolStat
	lastPool  *poolStat
	poolCount int
	acqMax    int32
	idleMin   int32
	totalMax  int32
	maxConns  int32

	// process handle for CPU/RSS; nil when no PID set.
	pid  int32
	proc *process.Process
}

// runtimeStat mirrors the JSON returned by GET {base}/_bench/runtime.
type runtimeStat struct {
	Goroutines int `json:"goroutines"`
}

// poolStat mirrors the JSON returned by GET {base}/_bench/pool. All fields are
// numbers; missing fields decode to zero.
type poolStat struct {
	MaxConns          int32 `json:"max_conns"`
	TotalConns        int32 `json:"total_conns"`
	AcquiredConns     int32 `json:"acquired_conns"`
	IdleConns         int32 `json:"idle_conns"`
	EmptyAcquireCount int64 `json:"empty_acquire_count"`
}

// NewSampler creates a sampler. benchBaseURL is the server base URL used to poll
// the env-gated /_bench/runtime and /_bench/pool hooks; pass "" to skip all
// HTTP-based sampling (the local process is still sampled if a PID is set). A
// non-positive interval defaults to 1s.
func NewSampler(benchBaseURL string, interval time.Duration) *Sampler {
	if interval <= 0 {
		interval = time.Second
	}
	return &Sampler{
		baseURL:  benchBaseURL,
		interval: interval,
		client:   &http.Client{Timeout: 2 * time.Second},
	}
}

// SetPID selects the OS process to sample for CPU% and RSS. When unset, those
// series stay empty and their MinAvgMax remain zero.
func (s *Sampler) SetPID(pid int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pid = pid
	if p, err := process.NewProcess(pid); err == nil {
		s.proc = p
	}
}

// Start launches a background sampling goroutine that ticks every interval until
// ctx is cancelled.
//
// CPU% approach: gopsutil's Percent(0) reports the CPU usage since the *previous*
// Percent call for that Process handle. We therefore establish a baseline with an
// initial Percent(0) at Start (its return value is discarded) and then call
// Percent(0) on each tick, which yields the busy fraction over the elapsed
// interval. This avoids blocking the tick with Percent(interval).
func (s *Sampler) Start(ctx context.Context) {
	// Establish CPU baseline and take an initial pool read so DBDelta has a
	// "first" reference even for very short windows.
	s.mu.Lock()
	proc := s.proc
	s.mu.Unlock()
	if proc != nil {
		_, _ = proc.Percent(0)
	}
	s.samplePool()

	go func() {
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.tick()
			}
		}
	}()
}

func (s *Sampler) tick() {
	s.sampleRuntime()
	s.sampleProc()
	s.samplePool()
}

// sampleRuntime polls goroutine count via the server hook (skipped when no base
// URL is configured).
func (s *Sampler) sampleRuntime() {
	if s.baseURL == "" {
		return
	}
	var rt runtimeStat
	if !s.getJSON(s.baseURL+"/_bench/runtime", &rt) {
		return
	}
	s.mu.Lock()
	s.goroutines = append(s.goroutines, float64(rt.Goroutines))
	s.mu.Unlock()
}

// sampleProc records CPU% and RSS MB for the target PID (skipped when unset).
func (s *Sampler) sampleProc() {
	s.mu.Lock()
	proc := s.proc
	s.mu.Unlock()
	if proc == nil {
		return
	}
	if pct, err := proc.Percent(0); err == nil {
		s.mu.Lock()
		s.cpuPercent = append(s.cpuPercent, pct)
		s.mu.Unlock()
	}
	if mi, err := proc.MemoryInfo(); err == nil && mi != nil {
		s.mu.Lock()
		s.rssMB = append(s.rssMB, float64(mi.RSS)/(1024*1024))
		s.mu.Unlock()
	}
}

// samplePool reads the DB pool hook and updates first/last/min/max trackers.
func (s *Sampler) samplePool() {
	if s.baseURL == "" {
		return
	}
	var p poolStat
	if !s.getJSON(s.baseURL+"/_bench/pool", &p) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poolCount == 0 {
		first := p
		s.firstPool = &first
		s.acqMax = p.AcquiredConns
		s.idleMin = p.IdleConns
		s.totalMax = p.TotalConns
		s.maxConns = p.MaxConns
	} else {
		if p.AcquiredConns > s.acqMax {
			s.acqMax = p.AcquiredConns
		}
		if p.IdleConns < s.idleMin {
			s.idleMin = p.IdleConns
		}
		if p.TotalConns > s.totalMax {
			s.totalMax = p.TotalConns
		}
		if p.MaxConns > s.maxConns {
			s.maxConns = p.MaxConns
		}
	}
	last := p
	s.lastPool = &last
	s.poolCount++
}

func (s *Sampler) getJSON(url string, dst any) bool {
	resp, err := s.client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	return json.NewDecoder(resp.Body).Decode(dst) == nil
}

// Resources reduces the sampled series to min/avg/max. Series with no samples
// yield a zero MinAvgMax. Samples reflects the largest series length observed.
func (s *Sampler) Resources() schema.Resources {
	s.mu.Lock()
	defer s.mu.Unlock()
	samples := len(s.goroutines)
	if len(s.cpuPercent) > samples {
		samples = len(s.cpuPercent)
	}
	if len(s.rssMB) > samples {
		samples = len(s.rssMB)
	}
	return schema.Resources{
		CPUPercent:  minAvgMax(s.cpuPercent),
		MemoryRSSMB: minAvgMax(s.rssMB),
		Goroutines:  minAvgMax(s.goroutines),
		Samples:     samples,
	}
}

// DBDelta returns the DB pool summary from first/last/min/max reads, or nil if
// no pool sample was obtained.
func (s *Sampler) DBDelta() *schema.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poolCount == 0 || s.firstPool == nil || s.lastPool == nil {
		return nil
	}
	return &schema.DB{
		MaxConns:               s.maxConns,
		AcquiredConnsMax:       s.acqMax,
		IdleConnsMin:           s.idleMin,
		TotalConnsMax:          s.totalMax,
		EmptyAcquireCountDelta: s.lastPool.EmptyAcquireCount - s.firstPool.EmptyAcquireCount,
	}
}

// minAvgMax reduces a series to min/avg/max, returning a zero value for empty
// input (never fabricates numbers).
func minAvgMax(vals []float64) schema.MinAvgMax {
	if len(vals) == 0 {
		return schema.MinAvgMax{}
	}
	mn, mx, sum := vals[0], vals[0], 0.0
	for _, v := range vals {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
		sum += v
	}
	return schema.MinAvgMax{
		Min: mn,
		Avg: sum / float64(len(vals)),
		Max: mx,
	}
}
