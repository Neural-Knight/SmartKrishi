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

# Apply migrations (requires golang-migrate CLI)
export DATABASE_URL=postgresql://smartkrishi_user:smartkrishi_password@127.0.0.1:5432/smartkrishi_db?sslmode=disable
migrate -path migrations -database "$DATABASE_URL" up

# Copy env and configure secrets
cp .env.example .env

# Run API (default port 8000 — same as Python backend)
make run
```

### Auth endpoints (Milestone 1)

- `POST /api/v1/auth/signup` — `{name, email, password}`
- `POST /api/v1/auth/login` — `{email, password}`
- `POST /api/v1/auth/token` — OAuth2 form (`username`, `password`)
- `GET /api/v1/auth/me` — Bearer JWT

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
