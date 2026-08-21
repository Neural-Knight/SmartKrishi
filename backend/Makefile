.PHONY: run build test tidy fmt migrate-up migrate-down sqlc \
	bench-build bench-controlled bench-e2e bench-e2e-no-gemini bench-report

run:
	go run ./cmd/smartkrishi

build:
	go build -o bin/smartkrishi ./cmd/smartkrishi

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	go fmt ./...

MIGRATE ?= $(shell go env GOPATH)/bin/migrate

migrate-install:
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate-up: migrate-install
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" up

migrate-down: migrate-install
	$(MIGRATE) -path migrations -database "$$DATABASE_URL" down 1

sqlc:
	sqlc generate

# --- Benchmark harness (backend/bench) --------------------------------------
# Results are written under bench/results/ (gitignored). Numbers are only ever
# produced by a real run; nothing here fabricates values.

# CGO is disabled for the bench CLI: gopsutil's transitive go-m1cpu dependency
# has a cgo init that segfaults under some Go/Apple-Silicon combinations. With
# CGO off, gopsutil uses its pure-Go paths for RSS/goroutines; CPU% may be
# reported as 0 on affected hosts (documented in bench/README.md) rather than
# crashing the harness.
BENCH := CGO_ENABLED=0 go run ./bench/cmd/smartkrishi-bench

bench-build:
	go build ./bench/...

# Controlled mode: in-process server (scripted LLM + stubbed tool HTTP, real
# Postgres). Deterministic; requires DATABASE_URL. 3 repetitions by default.
bench-controlled:
	$(BENCH) run --mode controlled --scenario all \
		--output bench/results/baseline-controlled.json

# End-to-end mode: drives a live server that must already be running with
# SMARTKRISHI_BENCH=1 (so /_bench hooks are available) and a real GEMINI_API_KEY.
# Set SMARTKRISHI_BENCH_URL if the server is not at http://127.0.0.1:8000.
bench-e2e:
	$(BENCH) run --mode e2e --scenario all \
		--output bench/results/baseline-e2e.json

# End-to-end without Gemini: only non-agent scenarios (health, chat CRUD).
bench-e2e-no-gemini:
	$(BENCH) run --mode e2e_no_gemini --scenario all \
		--output bench/results/baseline-e2e-no-gemini.json

# Render a human-readable report from a results file: make bench-report IN=bench/results/x.json
bench-report:
	$(BENCH) report --input $(IN)
