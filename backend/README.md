# SmartKrishi Go Backend

Go monolith replacing the Python FastAPI `backend/` and `Agentic-AI/` services.

## Status

**Milestone 0 (scaffold):** HTTP server, health check, CORS, structured logging, graceful shutdown.

Coming next: PostgreSQL (sqlc + pgx), auth, chat CRUD, inlined agent pipeline, SSE streaming.

## Layout

```
backend/
├── cmd/smartkrishi/     # Application entrypoint
├── internal/
│   ├── config/          # Environment configuration
│   ├── middleware/      # CORS, logging, recovery, request ID
│   ├── server/          # HTTP server and routes
│   ├── api/             # HTTP handlers (v1)
│   ├── service/         # Business logic
│   ├── agent/           # Gemini agent pipeline (from Agentic-AI)
│   └── repository/      # PostgreSQL (sqlc)
├── migrations/          # golang-migrate SQL
└── tests/
```

## Local development

```bash
# Start Postgres
docker compose up -d

# Copy env and configure secrets
cp .env.example .env

# Run API (default port 8000 — same as Python backend)
make run
# or: go run ./cmd/smartkrishi

# Frontend (unchanged, from repo root)
cd ../frontend && pnpm dev
```

## Build

```bash
make build
./bin/smartkrishi
```

## Branches

- `archive/python-v1` — frozen Python + Agentic-AI reference
- `feat/go-backend` — active Go migration (this code)

## Python reference

The previous Python backend is preserved on `archive/python-v1`. Do not delete that branch until the Go migration is production-ready.
