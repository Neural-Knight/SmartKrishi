# SmartKrishi Backend

HTTP API and agent service for SmartKrishi. Handles authentication, chat persistence, file uploads, and streaming AI responses through an in-process Gemini agent pipeline.

## Layout

```
backend/
├── cmd/smartkrishi/       Entry point
├── internal/
│   ├── api/               HTTP handlers (auth, chat)
│   ├── service/           Business logic
│   ├── repository/        PostgreSQL access
│   ├── agent/             Planner, tools, executor, SSE events
│   ├── middleware/        CORS, auth, logging, recovery
│   ├── config/            Environment configuration
│   └── server/            Router and wiring
├── migrations/            SQL migrations (golang-migrate)
├── docker-compose.yml     Local PostgreSQL
└── Makefile
```

## Requirements

- Go 1.22+
- Docker (local Postgres)
- [`golang-migrate`](https://github.com/golang-migrate/migrate) (installed automatically by `make migrate-up`)

## Local development

```bash
docker compose up -d
cp .env.example .env
# Configure DATABASE_URL, SECRET_KEY, GEMINI_API_KEY, FIREBASE_CREDENTIALS

set -a && source .env && set +a   # migrate reads DATABASE_URL from the shell
make migrate-up
make run
```

Default listen address: `:8000`

### Makefile targets

| Command | Description |
|---------|-------------|
| `make run` | Start the server |
| `make build` | Output binary to `bin/smartkrishi` |
| `make test` | Run all tests |
| `make migrate-up` | Apply migrations |
| `make migrate-down` | Roll back one migration |
| `make fmt` | Format Go source |

## Environment variables

See [`.env.example`](.env.example). Minimum for a working dev environment:

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | PostgreSQL connection string (use `?sslmode=disable` locally) |
| `SECRET_KEY` | JWT signing key (32+ characters) |
| `GEMINI_API_KEY` | Google Gemini API access |
| `FIREBASE_CREDENTIALS` | Service account file path or inline JSON |
| `FIREBASE_PROJECT_ID` | Firebase project ID |

Optional agent tool keys (tools degrade gracefully when unset):

| Variable | Purpose |
|----------|---------|
| `WEATHERAPI_KEY` | Current weather for a location |
| `DATA_GOV_KEY` | Indian open-data API key |
| `AGMARKNET_ID` | Resource ID for mandi price data (e.g. `9ef84268-d588-465a-a308-a864a43d0070`) |

Model names default to `gemini-2.5-flash` and can be overridden with `AGENT_PLANNER_MODEL`, `AGENT_MODEL`, and `AGENT_CHECKER_MODEL`.

## API overview

Base path: `/api/v1` (configurable via `API_V1_STR`)

### Health

- `GET /health` — service and database status
- `GET /` — welcome payload

### Auth

- `POST /auth/signup`, `/auth/login`, `/auth/token`
- `GET /auth/me`
- `POST /auth/mobile-init`, `/auth/mobile-verify`, `/auth/mobile-signup`

### Chat

- `GET/POST /chat/chats`, `GET/PUT/DELETE /chat/chats/{id}`
- `POST /chat/send-stream` — primary chat path (SSE)
- `POST /chat/upload-file`, `/chat/upload-and-analyze-stream`
- `GET /chat/chats/{id}/files`
- `GET /chat/suggestions`
- `POST /chat/ask`, `/chat/send`, `/chat/analyze-image`, `/chat/analyze-image-persistent`

Streaming responses use `Content-Type: text/event-stream` with JSON events (`plan`, `tool_call`, `response_chunk`, `end`, etc.).

## Production

Build and run:

```bash
make build
./bin/smartkrishi
```

Render deployment is defined in the repo root [`render.yaml`](../render.yaml). Run migrations against the production database before serving traffic:

```bash
export DATABASE_URL='postgresql://...?sslmode=require'
make migrate-up
```

Set `FRONTEND_URL` to the deployed frontend origin for CORS.
