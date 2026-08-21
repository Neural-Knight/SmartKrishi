# SmartKrishi benchmark harness

A reproducible benchmark and measurement harness for the SmartKrishi Go backend.
It measures the current system as-is — it does **not** change application
architecture or optimize anything, and it never fabricates numbers. Every metric
in a results file comes from a real run; unmeasured fields stay zero.

All benchmark code lives under `backend/bench/`. The only production-package
touch is a set of **env-gated debug routes** that exist solely when
`SMARTKRISHI_BENCH=1` (see [Server hooks](#server-hooks)).

## Modes

Results of different modes are never mixed in one file and never compared to
each other.

| Mode | What it exercises | Determinism |
|------|-------------------|-------------|
| `controlled` | In-process server: real HTTP/DB/SSE/pipeline, **scripted LLM** + **stubbed** weather/market HTTP, **real Postgres**. Isolates our own overhead from external variance. | Deterministic; the mode to use for regression tracking. |
| `e2e` | A live server with **real Gemini**, real Postgres, and real tool APIs. User-perceived latency. | High variance; **indicative only**, not a tight regression gate. |
| `e2e_no_gemini` | A live server, **non-agent scenarios only** (health, chat CRUD). No Gemini key needed. | Deterministic-ish (DB only). |

## Scenarios

Defined in [`scenarios/fixtures.json`](scenarios/fixtures.json) (versioned,
deterministic inputs):

- **A `simple_query`**, **B `single_tool_weather`**, **B2 `single_tool_market`**,
  **B3 `single_tool_soil`**, **C `multi_tool`** — `send-stream` agent scenarios.
- **D `file_analyze`** — `upload-and-analyze-stream`; optional (marked skipped if
  the file subsystem / Gemini File API is unavailable).
- **E `streaming_only`** — like A with `include_logs:false`.
- **F `concurrent_mixed`** — rotates A/B/C prompts by worker index for load.
- **G `http_health`**, **H `http_chat_crud`** — non-agent HTTP baselines.

Planner tool selection is **non-deterministic** in `e2e`: the harness reports the
`tool_call` events actually observed; a scenario name is an intent, not a
guarantee.

## Prerequisites

- Go (see `go.mod`), Postgres reachable via `DATABASE_URL`, migrations applied.
- For `e2e`: a `GEMINI_API_KEY` and a server started with `SMARTKRISHI_BENCH=1`.

> **CGO note (Apple Silicon):** gopsutil pulls in `go-m1cpu`, whose cgo `init`
> can segfault under some Go/macOS-arm64 combinations. The `make bench-*` targets
> therefore run the CLI with `CGO_ENABLED=0`; invoke the CLI the same way if you
> run it directly (`CGO_ENABLED=0 go run ./bench/cmd/smartkrishi-bench ...`). With
> CGO off, RSS and goroutine sampling work; CPU% may report 0 on affected hosts
> (it is never fabricated).

```bash
cd backend
docker compose up -d
set -a && source .env && set +a && make migrate-up
```

## Running

```bash
cd backend

# Controlled baseline (deterministic, no Internet). Requires DATABASE_URL.
make bench-controlled
# → bench/results/baseline-controlled.json  (+ prints a report)

# End-to-end baseline. First start the server WITH the bench flag:
SMARTKRISHI_BENCH=1 make run          # terminal 1
make bench-e2e                        # terminal 2 (needs GEMINI_API_KEY)
# → bench/results/baseline-e2e.json

# Non-agent live baseline (no Gemini key required):
SMARTKRISHI_BENCH=1 make run
make bench-e2e-no-gemini
```

The CLI directly:

```bash
go run ./bench/cmd/smartkrishi-bench run \
  --mode controlled --scenario all --output bench/results/run.json
go run ./bench/cmd/smartkrishi-bench run \
  --mode e2e --scenario multi_tool --concurrency 1,5 --output bench/results/e2e-C.json
go run ./bench/cmd/smartkrishi-bench report  --input bench/results/run.json
go run ./bench/cmd/smartkrishi-bench compare \
  --baseline bench/results/baseline-controlled.json \
  --current  bench/results/current-controlled.json \
  --output   bench/results/comparison.txt
```

### `run` flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--mode` | (required) | `controlled` \| `e2e` \| `e2e_no_gemini` |
| `--scenario` | `all` | scenario id/name (e.g. `C`, `multi_tool`) or `all` |
| `--concurrency` | `auto` | `auto` (from fixtures per mode) or `1,5,10` |
| `--duration` | fixtures (60s) | measurement seconds (ignored for e2e agent scenarios, which use fixed-N) |
| `--warmup` | fixtures (30s) | warm-up seconds excluded from latency histograms |
| `--repetitions` | controlled 3, e2e 2 | repetitions per (scenario, concurrency) |
| `--output` | (required) | results JSON path |
| `--url` | `$SMARTKRISHI_BENCH_URL` or `http://127.0.0.1:8000` | e2e target |
| `--database-url` | `$DATABASE_URL` | controlled Postgres |

**Concurrency ladders** (from fixtures): `e2e` → 1,5; `controlled` → 1,5,10,25;
HTTP-only scenarios → up to 100. Expensive `e2e` agent scenarios run a **fixed
number of requests** at concurrency 1 (not duration-based) to bound Gemini cost.

## Server hooks

When — and only when — `SMARTKRISHI_BENCH=1`, the server registers:

- `GET /_bench/pool` — `pgxpool.Stat()` JSON (cumulative counters; the harness
  deltas them across the run window).
- `GET /_bench/runtime` — goroutine count + a small mem snapshot.
- `/_debug/pprof/*` — standard pprof, **restricted to loopback clients** so it is
  never reachable off-host even with the flag on.

When the env var is unset these routes do not exist and behavior is unchanged.
`SMARTKRISHI_BENCH` must never be set in production.

## Output

Each run writes a `schema.Suite` JSON (`schema_version`, git + environment
metadata, `mode`, and a `Result[]`). Each `Result` carries HTTP metrics, agent
SSE metrics (for agent scenarios), sampled resources, DB pool deltas, the
`mode`/scenario/concurrency/repetition, and a `limitations[]` list. Results and
pprof profiles live under `results/` and `profiles/`, both gitignored.

## Metrics

- **HTTP**: requests/successful/failed, error_rate, RPS, latency p50/p95/p99/max/stddev.
- **Agent** (SSE, client-side): time-to-first-event, time-to-plan, time-to-first-token,
  stream duration, tool-call count + observed names, events/sec, streams-completed rate.
- **Resources** (1s sampler): CPU%, RSS MB, goroutines (min/avg/max).
- **DB**: pool max/acquired-max/idle-min/total-max + `empty_acquire` deltas.

## Reproducing a comparison

```bash
# baseline once, current after a future change, then diff (same mode only)
go run ./bench/cmd/smartkrishi-bench compare \
  --baseline bench/results/baseline-controlled.json \
  --current  bench/results/current-controlled.json
```

`compare` flags a regression when p95 latency rises >10%, error_rate rises >1pp,
or RPS drops >10%. It refuses to compare across modes.

## Limitations (documented on every result)

1. **Planner non-determinism** — tool selection varies in `e2e`; scenarios are intent.
2. **Gemini variance** — `e2e` p95 can swing 2–5× run-to-run; unsuitable for tight gates.
3. **Single instance** — no multi-node / load-balancer behavior.
4. **Auth DB lookup** on every chat request is included in stream latency.
5. **Environment-specific** — a laptop baseline is not comparable to Render; the
   result records host + git metadata so runs are attributable.
6. **Scenario D** depends on Gemini File API availability (and its 48h file TTL).
7. **Pool stats** are cumulative; the harness computes per-run deltas.
8. **Observer effect** — 1s resource sampling is light; running pprof under load
   skews CPU, so profiles are opt-in.

## What this harness deliberately does not do

No Redis, queues, caching, parallel tools, or any architecture change. No
application optimization. No fabricated numbers in code, docs, or committed
JSON. No silent mock substitution in `e2e` mode. No production pprof exposure
without `SMARTKRISHI_BENCH=1`.
