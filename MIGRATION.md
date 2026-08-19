# SmartKrishi Python → Go Migration Plan

# Synced from migration planning session (2026-08-19). See **Migration Progress (Handoff)** for current status.




## Repository & Branch Strategy

### Target layout on the migration branch

Only two top-level application folders — no `Agentic-AI/`, no parallel `smartkrishi-go/`:

```
SmartKrishi/                    # feat/go-backend branch
├── frontend/                   # UNCHANGED — same React/Vite app
├── backend/                    # REPLACED — Go monolith (not Python)
│   ├── cmd/smartkrishi/main.go
│   ├── internal/               # api, service, agent, repository, ...
│   ├── migrations/
│   ├── go.mod
│   ├── go.sum
│   ├── sqlc.yaml
│   ├── .env.example
│   ├── docker-compose.yml      # Postgres (+ Redis in M2)
│   └── README.md
├── render.yaml                 # Updated startCommand → Go binary
└── README.md                   # Updated root docs
```

**What happens to existing folders:**

| Path | On migration branch |
|------|---------------------|
| `frontend/` | Kept as-is (no changes required for Milestone 1) |
| `backend/` | Python FastAPI **removed**; Go structure written in its place |
| `Agentic-AI/` | **Removed** from branch — logic ported into `backend/internal/agent/` |
| `render.yaml` | Updated to build/run Go from `backend/` |

Python reference code remains available on the **legacy branch** (see below), not in the migration branch working tree.

### Branch workflow

```mermaid
flowchart LR
    mainNow[main today\nPython backend +\nAgentic-AI] --> legacyBranch[archive/python-v1\nfrozen archive]
    mainNow --> migBranch[feat/go-backend\nfrontend + Go backend]
    migBranch --> mainFuture[main future\nafter promotion]
    legacyBranch -.->|reference only| migBranch
```

**Step 0 — Before any Go code (do this first):**

1. From current `main`, create archive branch: `archive/python-v1`
   - Preserves full Python `backend/` at current `main` commit (already tracked)
2. On `archive/python-v1`: commit untracked `Agentic-AI/` (one-time archive commit — **user runs commit manually**)
3. Create working branch: `feat/go-backend`
4. On `feat/go-backend`:
   - Delete `Agentic-AI/` (after archive commit confirmed)
   - Replace `backend/` contents with Go scaffold (folder name unchanged)
   - Leave `frontend/` untouched
   - Update `render.yaml` for Go build/start
5. All Milestone 1–5 work happens on `feat/go-backend`

**Commit workflow:** Agent does not run `git commit`. At each checkpoint, agent provides the commit message and waits for user confirmation via question UI before continuing destructive steps.

**Step N — When Go backend is production-ready:**

1. Tag legacy: `git tag v1-python-final` on `archive/python-v1`
2. Promote: merge `feat/go-backend` → `main`
3. Result:
   - `main` → Go backend + frontend (new default)
   - `archive/python-v1` → rollback reference, diff source, benchmark baseline

### Why this approach

| Benefit | Explanation |
|---------|-------------|
| **Frontend unchanged** | `VITE_API_BASE_URL` still points to `/api/v1` on `backend/` — no path changes |
| **Deploy config unchanged** | `render.yaml` still `cd backend && ...` — only start command changes |
| **Clean working tree** | No confusion between Python `backend/app/` and Go `backend/internal/` |
| **Safe rollback** | Legacy branch keeps entire Python stack including Agentic-AI |
| **No folder rename churn** | CI, docs, and mental model stay "frontend + backend" |

### Deployment change (render.yaml)

Current:

```yaml
buildCommand: cd backend && chmod +x render-build.sh && ./render-build.sh
startCommand: cd backend && uvicorn app.main:app --host 0.0.0.0 --port $PORT
```

Target (on migration branch):

```yaml
buildCommand: cd backend && go build -o bin/smartkrishi ./cmd/smartkrishi
startCommand: cd backend && ./bin/smartkrishi
env: go
```

Remove `AGENT_API_BASE_URL` from env — no longer needed after consolidation.

### Local development on migration branch

```bash
# Terminal 1 — Postgres
cd backend && docker compose up -d

# Terminal 2 — Go API (same port 8000 as today)
cd backend && go run ./cmd/smartkrishi

# Terminal 3 — Frontend (unchanged)
cd frontend && pnpm dev
```

Frontend `.env` stays `VITE_API_BASE_URL=http://localhost:8000`.

---

## Migration Progress (Handoff — updated 2026-08-19)

**Active branch:** `feat/go-backend`  
**Archive branch:** `archive/python-v1` (Python `backend/` + `Agentic-AI/` preserved)  
**Python reference:** use `git show archive/python-v1:backend/...` or `git show archive/python-v1:Agentic-AI/...`

### Commits on `feat/go-backend`

| Commit | Summary |
|--------|---------|
| `3c21082` (on `archive/python-v1`) | Archive untracked `Agentic-AI/` + `.gitignore` |
| `b9a3f92` | Go scaffold replaces Python `backend/`; `render.yaml` → Go build; `Agentic-AI/` removed from this branch |
| `75952a6` | Postgres migrations (8 tables), pgxpool, email JWT auth (`signup`, `login`, `token`, `me`), real `/health` DB ping |

### Completed work

#### Repository & branches (Step 0) — DONE
- [x] `archive/python-v1` created; full Python stack + Agentic-AI snapshotted
- [x] `feat/go-backend` created; repo slimmed to `frontend/` + `backend/` only
- [x] `frontend/` untouched
- [x] `render.yaml` updated: `env: go`, `go build -o bin/smartkrishi ./cmd/smartkrishi`
- [x] `AGENT_API_BASE_URL` removed from deploy config (proxy eliminated by design)

#### Go scaffold (Step 1) — DONE
- [x] `backend/cmd/smartkrishi/main.go` — entrypoint, godotenv, graceful shutdown
- [x] `backend/internal/config/` — env loading (DATABASE_URL, SECRET_KEY, CORS origins, etc.)
- [x] `backend/internal/server/` — chi router, `/`, `/health`, `/api/v1/status`
- [x] `backend/internal/middleware/` — CORS (matches Python origins), request ID, slog logging, panic recovery
- [x] `backend/go.mod`, `Makefile`, `docker-compose.yml`, `README.md`, `sqlc.yaml` (config only)
- [x] `go build ./cmd/smartkrishi` passes

#### PostgreSQL (Step 2) — PARTIAL
- [x] `migrations/000001_init.up.sql` — all 8 tables (`users`, `chats`, `chat_messages`, `uploaded_files`, `reasoning_steps`, `agent_api_configs`, `fallback_sessions`, `fallback_messages`)
- [x] `migrations/000001_init.down.sql`
- [x] `internal/database/postgres.go` — pgxpool (MaxConns 20, pre-ping)
- [x] `internal/repository/postgres/user.go` — hand-written pgx queries (Create, GetByEmail, GetByID, GetByPhone)
- [ ] **sqlc not yet used** — `sqlc.yaml` exists; no generated code; chat/message/file repos still TODO
- [x] Local migrations applied: `migrate -path migrations -database "$DATABASE_URL" up` → `1/u init`
- [x] Local `DATABASE_URL` must include `?sslmode=disable` for Docker Postgres

#### Email auth (Step 3) — PARTIAL (email only)
- [x] `internal/service/auth/` — bcrypt, HS256 JWT (`sub` + `auth_provider` claims, matches Python)
- [x] `internal/api/auth/handler.go` — routes mounted at `/api/v1/auth/`
- [x] `POST /api/v1/auth/signup` — `{name, email, password}` → `{access_token, token_type}`
- [x] `POST /api/v1/auth/login` — `{email, password}` → token
- [x] `POST /api/v1/auth/token` — OAuth2 form (`username`, `password`) → token
- [x] `GET /api/v1/auth/me` — Bearer JWT → user JSON (FastAPI-compatible `detail` errors)
- [x] `internal/service/auth/jwt_test.go` — token round-trip tests
- [x] Signup smoke-tested locally against Docker Postgres
- [ ] **Firebase mobile auth** — NOT started (`mobile-init`, `mobile-verify`, `mobile-signup`)

#### Dev environment — DONE (local)
- [x] `backend/.env` created (gitignored) with local DATABASE_URL + SECRET_KEY
- [x] `make migrate-up` installs golang-migrate via `go install` if missing
- [x] Docker Postgres running via `docker compose up -d`
- [x] `make run` serves on `:8000` (same port as Python backend)

### Not started (Milestone 1 remainder)

| Step | Item | Python reference |
|------|------|------------------|
| 4 | Firebase mobile auth | `archive/python-v1:backend/app/routers/mobile_auth.py` |
| 5 | Chat CRUD | `archive/python-v1:backend/app/routers/chat.py`, `chat_service.py` |
| 6 | Agent pipeline (planner, tools, executor) | `archive/python-v1:Agentic-AI/app/` |
| 7 | SSE `send-stream` + `upload-and-analyze-stream` | `integrated_chat_service.py`, frontend `chatService.ts` |
| 8 | Reasoning persistence | `reasoning_service.py` |
| 9 | File upload + processing | `file_service.py`, `Agentic-AI/app/media.py` |
| 10 | Legacy endpoints (`/ask`, `/send`, `/analyze-image`) | `backend/app/ai/` |
| — | Frontend E2E validation | `frontend/` against Go API |
| — | Push branches to remote / Render staging deploy | — |
| 14 | Promote `feat/go-backend` → `main` | After Milestone 1 exit criteria |

### Current Go file map

```
backend/
├── cmd/smartkrishi/main.go
├── internal/
│   ├── api/auth/handler.go      ✅ email auth HTTP
│   ├── api/response.go          ✅ JSON + FastAPI-style errors
│   ├── config/config.go         ✅
│   ├── database/postgres.go     ✅ pgxpool
│   ├── domain/user.go           ✅
│   ├── middleware/              ✅ cors, auth, logging, recovery, requestid
│   ├── repository/postgres/user.go  ✅ users only
│   ├── server/server.go         ✅ wires auth when DB + SECRET_KEY set
│   └── service/auth/            ✅ jwt + signup/login
├── migrations/000001_init.*.sql ✅
└── (no internal/agent/, internal/service/chat/, streaming yet)
```

### Milestone 1 exit criteria — progress

| Criterion | Status |
|-----------|--------|
| Email signup/login | ✅ Done |
| Mobile login (Firebase) | ❌ Not started |
| Create chat, list chats | ❌ Not started |
| Streaming message (`send-stream`) | ❌ Not started |
| Upload file + analysis stream | ❌ Not started |
| SSE events match frontend | ❌ Not started |
| Data in PostgreSQL schema | ✅ Schema + auth users; chat data N/A yet |

### Local dev quick reference (for Claude)

```bash
cd backend
docker compose up -d
export PATH="$PATH:$(go env GOPATH)/bin"
export DATABASE_URL='postgresql://smartkrishi_user:smartkrishi_password@127.0.0.1:5432/smartkrishi_db?sslmode=disable'
make migrate-up          # first time only
make run                 # uses backend/.env if present

# Test auth
curl -s http://localhost:8000/health
curl -s -X POST http://localhost:8000/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test","email":"test@example.com","password":"secret123"}'
```

**Known local gotchas:**
- `migrate: command not found` → `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` or `make migrate-up`
- Postgres crash loop → `docker compose down -v && docker compose up -d` (wipes local volume)
- `SSL is not enabled` → add `?sslmode=disable` to DATABASE_URL

### Recommended next steps for Claude (in order)

1. **Firebase mobile auth** — Step 4; port `mobile_auth.py` + `firebase_service.py`; use `firebase.google.com/go/v4`
2. **Chat CRUD** — Step 5; repos for `chats` + `chat_messages`; mount `/api/v1/chat/chats/*`; match frontend `chatService.ts` shapes
3. **Agent pipeline** — Step 6; create `internal/agent/` from `Agentic-AI` (planner, tools, executor); use `google.golang.org/genai`
4. **SSE streaming** — Step 7; `POST /api/v1/chat/send-stream`; SSE `data: {json}\n\n`; match event types in `useStreamingChatNew.ts`
5. **Reasoning + files** — Steps 8–9
6. **Legacy endpoints** — Step 10
7. **Integration tests + frontend E2E** — validate against unchanged `frontend/`
8. **Milestone 2+** — Redis workers, fallback, observability, benchmarks (see Phase 15 below)

---

## Phase 2 — Current Architecture

### Service/module structure

```
SmartKrishi/
├── frontend/          React/Vite — Bearer JWT, SSE consumer
├── backend/           Main FastAPI (port 8000)
│   ├── app/main.py           App bootstrap, CORS, routers
│   ├── app/routers/          auth, mobile_auth, chat, fallback
│   ├── app/services/         chat, integrated_chat, agent_api, file, fallback, sms, telegram
│   ├── app/ai/               Legacy direct Gemini (non-agent paths)
│   ├── app/models/           SQLAlchemy: users, chats, messages, files, reasoning, fallback
│   └── app/db/database.py    PostgreSQL pool (SQLAlchemy)
├── Agentic-AI/        Agent FastAPI (port 8080)
│   ├── app/main.py           All routes in one file (~1050 lines)
│   ├── app/nodes/            planner, main_agent, checker (/ask only)
│   ├── app/tools/            weather, market, soil, file_tools, chat_history
│   ├── app/history.py        SQLite chats/messages
│   ├── app/file_manager.py   SQLite file metadata
│   ├── app/media.py          Disk uploads + ChromaDB + Gemini processing
│   └── app/client_manager.py Per-chat Gemini client + file registry JSON
└── render.yaml        Deploys backend only + managed Postgres
```

### ASCII architecture diagram (actual)

```
React Frontend (Vercel)
  |  Bearer JWT + SSE (text/event-stream)
  v
backend/ FastAPI :8000
  |
  +-- CORS Middleware (explicit origins)
  +-- JWT Auth (deps.get_current_user)
  |
  +-- /api/v1/auth/* ---------> PostgreSQL (users)
  +-- /api/v1/chat/* --------> PostgreSQL (chats, messages, files, reasoning_steps)
  |       |
  |       +-- Legacy paths --> backend/app/ai/gemini_service.py --> Gemini API (google-generativeai)
  |       |
  |       +-- Streaming paths -> IntegratedChatService
  |               |
  |               +--> AgentAPIService (httpx) --HTTP NDJSON-->
  |               |         Agentic-AI :8080 /ask_stream
  |               +--> ReasoningService --> PostgreSQL (reasoning_steps)
  |               +--> ChatService --> PostgreSQL (chat_messages)
  |
  +-- /api/v1/fallback/* ----> FallbackService (in-process asyncio tasks)
          |
          +--> SMSService (httpx) --> External SMS microservice (ngrok URL default)
          +--> TelegramClient (httpx) --> External Telegram bot service
          +--> PostgreSQL (fallback_sessions, fallback_messages)

Agentic-AI FastAPI :8080  (no auth — trusts caller-supplied user_id)
  |
  +-- SQLite (data/chat_history.db, data/files.db)
  +-- ChromaDB (data/vectors/pdfs)
  +-- Disk (uploads/{user_id}/)
  +-- Gemini API (google-genai) — planner/agent/checker + file processing
  +-- External: WeatherAPI, data.gov.in (AgMarkNet), mock soil_api
```

### Main request flows

**Flow A — Primary streaming chat (frontend path)**

```
POST /api/v1/chat/send-stream (JSON: message, chat_id?, model?, tools?, include_logs?)
  → get_current_user (JWT)
  → ChatService.get/create chat (Postgres)
  → ChatService.add_message (user)          [router]
  → IntegratedChatService.send_message_stream
      → ChatService.ensure_agent_chat → AgentAPIService.create_chat (HTTP to Agentic-AI)
      → AgentAPIService.stream_message → POST Agentic-AI /ask_stream (NDJSON)
      → For each event: ReasoningService.create_reasoning_step (Postgres commit per event!)
      → Accumulate response_chunk → update assistant ChatMessage (Postgres commit per chunk!)
  → Router wraps events as SSE: data: {...}\n\n
  → Final: data: {"type":"end",...}\n\n
```

**Flow B — File upload + analyze stream**

```
POST /api/v1/chat/upload-and-analyze-stream (multipart: file, message, chat_id?)
  → IntegratedChatService.upload_file_and_analyze
      → AgentAPIService.upload_file → Agentic-AI /upload/{type}
      → FileService.save_file_to_database (Postgres)
      → send_message_stream (same as Flow A)
```

**Flow C — Legacy non-agent chat**

```
POST /api/v1/chat/send or /ask
  → backend/app/ai/chat_service.py → GeminiService (gemini-2.5-flash, simple prompt)
  → No agent pipeline, no reasoning steps
```

**Flow D — SMS fallback (deferred in Milestone 1)**

```
PUT /api/v1/fallback/settings → Postgres users table
POST /api/v1/fallback/activate → FallbackService.activate_fallback
  → sms_service.register_phone + send_sms
  → asyncio.create_task(start_sms_listener) — long-poll loop in-process
```

### Database architecture (PostgreSQL — main backend)

8 tables via SQLAlchemy ([`backend/app/models/`](backend/app/models/)):

| Table | Key fields | Notes |
|-------|-----------|-------|
| `users` | email, phone_number, hashed_password, auth_provider, fallback_* | Unique email + phone |
| `chats` | UUID PK, user_id, title, agent_chat_id, is_fallback_chat | Soft delete via is_deleted |
| `chat_messages` | UUID PK, chat_id, role, content, message_type, file_url | reasoning_steps FK |
| `uploaded_files` | UUID PK, agent_file_id, processing_status, summary | Soft delete |
| `reasoning_steps` | step_type, step_order, step_metadata (JSON) | One row per stream event |
| `agent_api_configs` | preferred_model, default_tools (JSON), include_logs | Per user |
| `fallback_sessions` | phone_number, is_active, activation_trigger | |
| `fallback_messages` | inbound/outbound, sms_id | |

**Indexes:** PK indexes only; no explicit secondary indexes beyond `users.email`, `users.phone_number`.

**Transactions:** Per-request SQLAlchemy session; streaming path commits on **every chunk** (performance concern).

### Redis architecture

**None implemented.** Documented aspiration only.

### External dependencies

| Service | Used by | Env var | Timeout |
|---------|---------|---------|---------|
| Google Gemini | Both backends | `GEMINI_API_KEY` / `GOOGLE_API_KEY` | 60–300s streaming |
| Firebase Admin | mobile_auth | `FIREBASE_CREDENTIALS` | — |
| WeatherAPI | Agentic-AI tools | `WEATHERAPI_KEY` | 5s |
| data.gov.in AgMarkNet | Agentic-AI tools | `DATA_GOV_KEY`, `AGMARKNET_ID` | default requests |
| External SMS API | fallback | `SMS_API_BASE_URL` | 35s long-poll |
| Telegram bot service | fallback | `TELEGRAM_SERVICE_URL` | httpx default |

### Authentication flow

1. **Email:** `POST /auth/signup|login` → bcrypt hash → JWT (`sub=email`, `auth_provider=email`)
2. **Mobile:** Frontend Firebase OTP → `POST /auth/mobile-verify` → Firebase Admin `verify_id_token` → JWT (`sub=phone`, `auth_provider=mobile`)
3. **Protected routes:** `HTTPBearer` → `verify_token` → DB lookup by email or phone
4. **Agentic-AI:** No auth — `user_id` is caller-supplied string

### AI request flow (agent path)

```
User message
  → Planner (gemini-2.5-flash) → JSON plan {tools, location, ...}
  → Tool execution (weather_api, market_api, soil_api, file_tools, chat_history)
  → Main agent (gemini-2.5-flash in stream, gemini-2.5-pro in /ask)
      → Google Search grounding, URL context, code execution (conditional)
  → Checker (gemini-2.5-flash) — /ask only, SKIPPED in /ask_stream
  → Persist assistant message
```

**Known bugs to fix in Go (not replicate blindly):**
- Tools invoked as `fn(location_string)` but file tools expect `(file_id, user_id, ...)` ([`Agentic-AI/app/main.py`](Agentic-AI/app/main.py) ~753)
- `market_api(crop, region)` receives location as crop
- Duplicate user message creation (router + IntegratedChatService)
- XLSX upload uses `file_id` as `chat_id` in client_manager ([`Agentic-AI/app/media.py:467`](Agentic-AI/app/media.py))

### Streaming flow

| Layer | Format | Content-Type |
|-------|--------|--------------|
| Agentic-AI `/ask_stream` | NDJSON lines | `application/json` |
| backend `/send-stream` | SSE `data: {...}\n\n` | `text/event-stream` |
| Frontend parser | Splits on `data: ` prefix | 5min timeout |

**Event types frontend handles:** `log`, `plan`, `tool_call`, `thinking`, `response_chunk`, `response`, `code_execution`, `grounding_*`, `file_uploaded`, `error`, `end`

### File-processing flow

```
Upload → save to disk (uploads/{user_id}/)
      → SQLite metadata insert
      → SYNC processing (despite "background" response message):
          PDF: PyPDF → ChromaDB embed + Gemini File API + summary
          Image: Gemini File API + vision analysis
          DOCX: docx2pdf → PDF pipeline
          XLSX: pandas → temp CSV → Gemini code execution
          CSV: Gemini File API + code execution
```

### Background/async work (current)

| Work | Mechanism | Problem |
|------|-----------|---------|
| File processing | Sync in request | Blocks HTTP; mislabeled as "background" |
| SMS listener | `asyncio.create_task` per session | Lost on restart; not multi-instance safe |
| Network monitor | `asyncio.create_task` per user | Uses SMS health as proxy for network |
| Gemini file cleanup | `cleanup_expired_files()` exists | Never called from main |

### Failure/error handling

- Generic `HTTPException(500)` with logged traceback
- Streaming: error events yielded + `end` event (frontend can recover UI)
- Agent API timeout: 300s total, yields `{"type":"error"}`
- Health check: `/health` tests Postgres; does not check Agent API, Redis, or Gemini
- No circuit breakers, no retry with backoff (except implicit httpx)

### Configuration/secrets

From [`backend/.env.example`](backend/.env.example) + [`Agentic-AI/.env.example`](Agentic-AI/.env.example):

`DATABASE_URL`, `SECRET_KEY`, `ALGORITHM`, `ACCESS_TOKEN_EXPIRE_MINUTES`, `API_V1_STR`, `FRONTEND_URL`, `FIREBASE_CREDENTIALS`, `GEMINI_API_KEY`, `SMS_API_BASE_URL`, `AGENT_API_BASE_URL`, `TELEGRAM_SERVICE_URL`, `WEATHERAPI_KEY`, `DATA_GOV_KEY`, `AGMARKNET_ID`

### Deployment architecture

- **Render:** Python web service + managed Postgres ([`render.yaml`](render.yaml))
- **Docker:** Postgres only ([`backend/docker-compose.yml`](backend/docker-compose.yml)) — no app container
- **Agentic-AI:** Manual `uvicorn` — not in Render config
- **Frontend:** Vercel, `VITE_API_BASE_URL` points to Render backend

---

## Phase 3 — Complete Functional Inventory

| Current Python Component | Responsibility | API/Interface | Dependencies | State | Proposed Go Component | Difficulty |
|--------------------------|---------------|---------------|--------------|-------|----------------------|------------|
| `backend/app/main.py` | App bootstrap, CORS, health | `GET /`, `GET /health` | Postgres | Stateless | `backend/cmd/smartkrishi` + `backend/internal/server` | Low |
| `backend/app/routers/auth.py` | Email signup/login/JWT | `POST /api/v1/auth/{signup,login,token}`, `GET /me` | Postgres, bcrypt, JWT | Stateless | `internal/api/auth` + `internal/service/auth` | Low |
| `backend/app/routers/mobile_auth.py` | Firebase mobile OTP verify | `POST /api/v1/auth/mobile-{init,verify,signup}` | Postgres, Firebase Admin | Stateless | `internal/api/auth/mobile` + `internal/ai/firebase` | Medium |
| `backend/app/routers/chat.py` | All chat/file/streaming endpoints | 20+ `/api/v1/chat/*` routes | Postgres, Agent API, Gemini | Per-request DB session | `internal/api/chat` | High |
| `backend/app/services/chat_service.py` | Chat CRUD, agent_chat_id sync | Internal | Postgres, Agent API HTTP | Stateless | `internal/service/chat` | Medium |
| `backend/app/services/integrated_chat_service.py` | Stream orchestration + persistence | Internal async generator | Agent API, Postgres | Accumulated response in memory | `internal/service/chat/stream` | High |
| `backend/app/services/agent_api_service.py` | HTTP client to Agentic-AI | Internal httpx | Agentic-AI HTTP | Stateless | **Eliminated** — inlined into `internal/agent` | N/A |
| `backend/app/services/reasoning_service.py` | Persist stream events | Internal | Postgres JSON columns | Stateless | `internal/service/reasoning` | Medium |
| `backend/app/services/file_service.py` | File metadata + agent upload proxy | Internal | Postgres, Agent API | Stateless | `internal/service/file` | Medium |
| `backend/app/ai/chat_service.py` | Legacy direct Gemini chat | `/chat/ask`, `/send` (legacy) | Gemini (generativeai SDK) | Stateless | `internal/ai/gemini` (simple path) | Low |
| `backend/app/ai/gemini_service.py` | Text + image Gemini calls | Internal | Gemini API | Stateless | `internal/ai/gemini` | Low |
| `backend/app/routers/fallback.py` | SMS/Telegram fallback REST | 15+ `/api/v1/fallback/*` | Postgres, SMS, Telegram | **In-memory listeners** | `internal/api/fallback` (Milestone 3+) | High |
| `backend/app/services/fallback_service.py` | Fallback orchestration | Internal | SMS, Postgres, asyncio tasks | **Global active_listeners** | `internal/service/fallback` + Redis locks | High |
| `backend/app/services/sms_service.py` | External SMS HTTP client | Internal | SMS microservice | Stateless | `internal/client/sms` | Medium |
| `backend/app/services/telegram_client.py` | External Telegram HTTP client | Internal | Telegram service | Stateless | `internal/client/telegram` | Low |
| `backend/app/services/firebase_service.py` | Firebase token verify | Internal | Firebase Admin | Stateless | `internal/client/firebase` | Medium |
| `Agentic-AI/app/main.py` | Agent HTTP API (all routes) | `/ask`, `/ask_stream`, `/upload/*`, chat CRUD | SQLite, Gemini, ChromaDB | USER_TOOL_PREFS dict | `internal/agent/api` (internal only) or inlined | High |
| `Agentic-AI/app/nodes/planner.py` | JSON plan generation | Internal | Gemini flash | Stateless | `internal/agent/planner` | Medium |
| `Agentic-AI/app/nodes/main_agent.py` | Tool exec + answer gen | Internal | Gemini, tools | Per-chat client | `internal/agent/executor` | High |
| `Agentic-AI/app/nodes/checker.py` | Answer validation | Internal | Gemini flash | Stateless | `internal/agent/checker` | Low |
| `Agentic-AI/app/tools/*` | Weather, market, soil, files | Tool registry | External APIs, SQLite, Chroma | Stateless | `internal/agent/tools/*` | Medium |
| `Agentic-AI/app/client_manager.py` | Per-chat Gemini client + file registry | Internal | Gemini File API, JSON file | **Unbounded clients map** | `internal/agent/gemini/clientpool` | Medium |
| `Agentic-AI/app/media.py` | File save + processing | `/upload/*` | Disk, ChromaDB, Gemini, pandas | ChromaDB collection | `internal/service/file/processor` + worker | High |
| `Agentic-AI/app/history.py` | SQLite chat/message store | Internal + some HTTP | SQLite | **Global conn** | **Merged into Postgres** via `internal/repository` | Medium |
| `Agentic-AI/app/file_manager.py` | SQLite file metadata | Internal | SQLite | Global conn | **Merged into Postgres** `uploaded_files` | Low |
| ChromaDB `PDF_COL` | PDF text embeddings | Internal file_tools | Chroma persistent | Disk vectors | `pgvector` table OR Gemini-only (redesign) | High |
| `frontend.html` | Standalone demo UI | `GET /` on Agentic-AI | Static file | — | **Remove** (not used by React frontend) | — |

**Classification legend:**
- **Must preserve exactly:** All `/api/v1/*` routes consumed by [`frontend/src/services/`](frontend/src/services/)
- **Can be redesigned:** Agentic-AI HTTP surface (internal after consolidation), ChromaDB → pgvector, per-chunk DB commits → batched, checker in streaming path
- **Can be removed:** Agentic-AI `frontend.html`, `graph.py` stub, duplicate SQLite stores, HTTP proxy layer
- **Unclear / verify manually:** Whether legacy `/chat/ask` and `/chat/send` are still used in production UI (hooks favor `send-stream`)

---

## Phase 4 — Proposed Go Architecture

### Design principles

1. **Single binary, modular packages** — not microservices
2. **Preserve `/api/v1` contracts** — frontend unchanged
3. **Unify persistence in PostgreSQL** — eliminate Agentic-AI SQLite duplication
4. **Inline agent pipeline** — delete `AGENT_API_BASE_URL` proxy hop
5. **Add Redis only where it solves real problems** (jobs, distributed locks) — not for show

### Target directory structure

**Root:** `backend/` (replaces Python FastAPI contents on the `go-migration` branch)

```
backend/
├── cmd/smartkrishi/main.go              # Entry: config load, DI wiring, graceful shutdown
├── internal/
│   ├── config/config.go                 # Env parsing (caarlos0/env)
│   ├── server/
│   │   ├── server.go                    # http.Server, routes, lifecycle
│   │   └── router.go                    # chi router mounting
│   ├── middleware/
│   │   ├── cors.go                      # Match current origin list
│   │   ├── auth.go                      # JWT Bearer → user context
│   │   ├── requestid.go
│   │   ├── logging.go                   # slog structured
│   │   └── recovery.go
│   ├── api/v1/
│   │   ├── auth/handler.go
│   │   ├── chat/handler.go              # All /chat/* endpoints
│   │   └── fallback/handler.go        # Milestone 3+
│   ├── domain/                          # Pure structs, no DB tags
│   │   ├── user.go, chat.go, file.go, reasoning.go, fallback.go
│   ├── repository/postgres/             # sqlc-generated queries
│   │   ├── queries/*.sql
│   │   └── *.sql.go                     # generated
│   ├── service/
│   │   ├── auth/service.go              # bcrypt, JWT issue/verify
│   │   ├── chat/service.go              # CRUD, title generation
│   │   ├── chat/stream.go               # SSE orchestration
│   │   ├── reasoning/service.go
│   │   ├── file/service.go
│   │   └── fallback/service.go          # Milestone 3+
│   ├── agent/                           # Former Agentic-AI (internal only)
│   │   ├── pipeline.go                  # planner → tools → agent → checker
│   │   ├── planner/planner.go
│   │   ├── executor/executor.go
│   │   ├── checker/checker.go
│   │   ├── stream/emitter.go            # NDJSON/SSE event types
│   │   ├── tools/
│   │   │   ├── registry.go
│   │   │   ├── weather.go, market.go, soil.go
│   │   │   └── files.go
│   │   └── gemini/
│   │       ├── client.go                # google.golang.org/genai
│   │       └── clientpool.go            # Per-chat client with eviction
│   ├── client/                          # External HTTP clients
│   │   ├── sms/client.go
│   │   ├── telegram/client.go
│   │   └── weather/client.go
│   ├── worker/
│   │   ├── pool.go                      # Bounded worker pool
│   │   ├── file_processor.go            # PDF/DOCX/XLSX jobs
│   │   └── gemini_cleanup.go
│   ├── streaming/sse.go                 # SSE writer with flush
│   └── storage/
│       ├── local/fs.go                  # uploads/{user_id}/
│       └── vector/pgvector.go           # Optional; Milestone 2+
├── migrations/                          # golang-migrate SQL files
├── sqlc.yaml
├── go.mod
├── go.sum
├── .env.example                         # Merged from backend + Agentic-AI env vars
├── docker-compose.yml
├── Makefile                             # test, migrate, sqlc generate, run
└── tests/
    ├── integration/
    ├── api/
    └── load/
```

**Repo root** (migration branch): only `frontend/`, `backend/`, `render.yaml`, `README.md` — no `Agentic-AI/`.

### Why these decisions

| Decision | Rationale (from codebase) |
|----------|--------------------------|
| **chi router** over raw `net/http` | Clean middleware chain; matches existing router-per-domain structure |
| **sqlc + pgx** over ORM | 8 well-defined tables; no complex inheritance; Alembic unused anyway; type-safe queries for reasoning JSON |
| **Single Gemini SDK** (`google.golang.org/genai`) | Agentic-AI already uses `google-genai`; backend's `google-generativeai` is legacy/simple path only |
| **Eliminate SQLite + proxy** | Duplicate chat state between Postgres `agent_chat_id` and Agentic-AI SQLite `chat_id` causes sync bugs |
| **Keep `agent_chat_id` column initially** | Can store internal agent session ID; avoids frontend migration |
| **SSE via `http.Flusher`** | Frontend expects `text/event-stream` with `data:` prefix — must match exactly |
| **Defer fallback** | In-process asyncio listeners don't survive restarts; needs Redis redesign anyway |

---

## Phase 5 — Concurrency Design

| Candidate | Current behavior | Bound | Safe? | Shared state | Proposed Go design |
|-----------|-----------------|-------|-------|--------------|-------------------|
| **AI streaming** | Sync Gemini iterator inside `async def` generator | I/O | Yes per-request | Per-chat Gemini client | 1 goroutine reads Gemini stream → `chan Event` → SSE writer goroutine; `context.Cancel` on client disconnect |
| **Tool parallelization** | Sequential tool calls in plan | I/O | Yes if tools independent | None | `errgroup.Group` with per-tool timeout (5s weather, 10s market); cap concurrency at 3 |
| **File processing** | Sync in upload handler (blocks 5–60s) | CPU+I/O | Per-file | ChromaDB, disk | Milestone 2: enqueue job → worker pool (bounded queue 100, workers 4); return `processing_status: processing` honestly |
| **DB during stream** | Commit per `response_chunk` | I/O | Risk: connection held long | SQLAlchemy session | Buffer chunks; flush to Postgres every 500ms or 2KB via dedicated goroutine; final commit on `response` |
| **Reasoning persistence** | Commit per event | I/O | Same session issues | Postgres | Batch insert via channel + `COPY` or multi-row insert every N events |
| **SMS listener** | `asyncio.create_task` infinite loop | I/O | No (multi-instance) | `active_listeners` dict | Milestone 3: Redis distributed lock per phone; single consumer; or webhook-only (no long-poll) |
| **Gemini client pool** | Unbounded `clients` dict | I/O | Memory leak risk | Per-chat clients | `sync.Map` + LRU eviction (max 500 clients); TTL 24h |
| **Health/network monitor** | Per-user asyncio task | I/O | Duplicates across instances | In-memory checks | Milestone 3: single scheduler goroutine OR Redis-backed heartbeat |

**Do NOT parallelize:** bcrypt password hashing (CPU-bound, fast enough), JWT verification, simple CRUD queries.

**Cancellation:** Propagate `r.Context()` from HTTP request through agent pipeline; on client disconnect, cancel Gemini stream and stop DB writes.

**Backpressure:** SSE writer: if `Flusher` blocks, drop intermediate `log` events before `response_chunk`; never drop `response`/`error`/`end`.

**Race conditions to test:** concurrent messages to same chat; duplicate user message insert (fix by idempotency key on message); file upload while streaming.

---

## Phase 6 — Redis Design

### Current Redis usage: **NONE**

### Proposed Redis usage (Milestone 2+, justified)

| Use case | Key pattern | Structure | TTL | Why Redis |
|----------|------------|-----------|-----|-----------|
| **File processing queue** | `queue:file_process` | LIST or Streams | — | Decouple upload HTTP from CPU-heavy PDF/DOCX; survive process restart |
| **Job status** | `job:{id}` | HASH (status, progress) | 24h | Frontend polls processing_status |
| **Weather/market cache** | `cache:weather:{loc}` | STRING (JSON) | 15min | WeatherAPI called repeatedly for same location in tool loop |
| **Rate limiting** | `rl:{user_id}:{endpoint}` | STRING counter | 1min window | Protect Gemini quota; `slowapi` in requirements.txt but never wired |
| **Fallback leader lock** | `lock:sms_listener:{phone}` | STRING SET NX | 60s renew | Only one instance long-polls per phone (Milestone 3) |
| **Dead letter queue** | `dlq:file_process` | LIST | 7d | Failed jobs after 3 retries |

**Do NOT use Redis for:** JWT sessions (stateless JWT works today), chat message storage (Postgres is source of truth), Gemini file registry (use Postgres table).

### Cache-aside pattern (weather example)

```
tool weather_api(location):
  1. GET cache:weather:{location}
  2. On miss: HTTP WeatherAPI → SET with 15min TTL
  3. On Redis down: call API directly (graceful degradation)
```

---

## Phase 7 — Asynchronous Job System

### Operations that should become async

| Job | Currently | Priority | Worker concurrency |
|-----|-----------|----------|-------------------|
| PDF text extract + embed | Sync in upload | P1 | 2 workers (CPU) |
| DOCX → PDF convert | Sync (docx2pdf) | P1 | 1 worker (LibreOffice subprocess) |
| XLSX → analysis | Sync (pandas) | P1 | 2 workers |
| Image Gemini analysis | Sync | P2 | 4 workers (I/O) |
| Gemini file registry cleanup | Never runs | P2 | 1 scheduled/hour |
| Reasoning batch persist | Sync per event | P2 | Batched writer |
| SMS long-poll | In-process task | P3 | 1 per active session with Redis lock |

### Job structure

```go
type FileProcessJob struct {
    ID        uuid.UUID
    UserID    int
    ChatID    uuid.UUID
    FileID    uuid.UUID
    Path      string
    FileType  string
    Attempt   int
    CreatedAt time.Time
}
```

### Queue semantics

- **Enqueue:** `LPUSH queue:file_process {json}` on upload complete
- **Dequeue:** `BRPOP` with 5s timeout in worker loop
- **Retry:** exponential backoff `2^attempt * 5s`, max 3 attempts → DLQ
- **Idempotency:** `job:{file_id}` SET NX before processing
- **Timeout:** 10min per job via `context.WithTimeout`
- **Graceful shutdown:** stop accepting new jobs; drain queue with 30s deadline; mark in-flight jobs as `interrupted` for retry

**NOT appropriate for job queue:** streaming chat (must stay synchronous SSE), auth, chat CRUD.

---

## Phase 8 — API Compatibility

### Endpoint migration table (frontend-facing — must preserve)

| Python Endpoint | Method | Request | Response | Go Endpoint | Compatibility Notes |
|----------------|--------|---------|----------|-------------|---------------------|
| `/` | GET | — | `{message, version, docs, redoc}` | Same | Exact |
| `/health` | GET | — | `{status, service, database, version}` | Same | Add `ready` sub-check later |
| `/api/v1/auth/signup` | POST | `{name, email, password}` | `{access_token, token_type}` | Same | bcrypt + JWT identical claims |
| `/api/v1/auth/login` | POST | `{email, password}` | Token | Same | |
| `/api/v1/auth/token` | POST | OAuth2 form | Token | Same | Swagger compat |
| `/api/v1/auth/me` | GET | Bearer | User object | Same | Field names via json tags |
| `/api/v1/auth/mobile-init` | POST | `{phone_number, username?}` | `{message, is_new_user, phone_number, status}` | Same | |
| `/api/v1/auth/mobile-verify` | POST | `{phone_number, id_token}` | Token | Same | Firebase Admin verify |
| `/api/v1/auth/mobile-signup` | POST | `{phone_number, id_token, name}` | Token | Same | |
| `/api/v1/chat/send-stream` | POST | JSON `{message, chat_id?, model?, tools?, include_logs?}` | SSE stream | Same | **Critical** — event types must match |
| `/api/v1/chat/upload-and-analyze-stream` | POST | multipart `{file, message, chat_id?}` | SSE stream | Same | Include `file_uploaded` event |
| `/api/v1/chat/chats` | GET | `?skip&limit` | `[ChatSummary]` | Same | |
| `/api/v1/chat/chats` | POST | `{title}` | `Chat` | Same | |
| `/api/v1/chat/chats/{id}` | GET | — | `Chat` with messages | Same | |
| `/api/v1/chat/chats/{id}` | PUT | `{title}` | `Chat` | Same | |
| `/api/v1/chat/chats/{id}` | DELETE | — | `{message}` | Same | Soft delete |
| `/api/v1/chat/send` | POST | `{message, chat_id?}` | `{response, chat_id, message_id}` | Same | Legacy — keep |
| `/api/v1/chat/ask` | POST | `{message, chat_history?}` | `{response}` | Same | Legacy — keep |
| `/api/v1/chat/analyze-image` | POST | multipart image | `{response}` | Same | Legacy |
| `/api/v1/chat/analyze-image-persistent` | POST | multipart + chat_id | `SendMessageResponse` | Same | |
| `/api/v1/chat/suggestions` | GET | — | `{suggestions: [...]}` | Same | Static list |
| `/api/v1/chat/chats/{id}/reasoning` | GET | — | `{chat_id, reasoning_steps}` | Same | |
| `/api/v1/chat/messages/{id}/reasoning` | GET | — | `{message_id, reasoning_steps}` | Same | |
| `/api/v1/chat/agent-tools` | GET | — | `{tools: [...]}` | Same | From tool registry |
| `/api/v1/chat/agent-config` | GET/PUT | config fields | `AgentApiConfig` | Same | |
| `/api/v1/chat/upload-file` | POST | multipart | `FileUploadResponse` | Same | 10MB limit |
| `/api/v1/chat/chats/{id}/files` | GET | — | `ChatFilesResponse` | Same | |
| `/api/v1/chat/files/{id}` | GET | — | `UploadedFile` | Same | |
| `/api/v1/chat/files/{id}` | DELETE | — | `{message, file_id}` | Same | Soft delete |
| `/api/v1/fallback/*` (15 routes) | Various | Various | Various | Same paths | **Milestone 3** — deferred |

### Agentic-AI endpoints (internal — eliminated as HTTP)

Previously proxied via `AgentAPIService`. After consolidation, these become internal function calls in `internal/agent/`. **No external HTTP exposure needed.**

### Compatibility risks

| Risk | Mitigation |
|------|-----------|
| SSE flush buffering behind Render/nginx | Set `X-Accel-Buffering: no` header; flush after each event |
| UUID serialization format | Use RFC 4122 string format in JSON |
| `datetime` timezone | Emit ISO 8601 with timezone (Postgres `timestamptz`) |
| Error status codes | 401 for auth, 404 chat not found, 400 validation — match FastAPI |
| Duplicate `end` events | Router + service both emit `end` today; Go should emit exactly once from handler |

---

## Phase 9 — Database Migration

### Existing schema (preserve initially)

All 8 tables from SQLAlchemy models — no schema change required for Milestone 1.

### New tables (Milestone 2+)

```sql
-- Replace ChromaDB + gemini_files_registry.json
CREATE TABLE gemini_file_registry (
    id UUID PRIMARY KEY,
    chat_id UUID REFERENCES chats(id),
    gemini_file_name TEXT NOT NULL,
    local_path TEXT,
    mime_type TEXT,
    uploaded_at TIMESTAMPTZ DEFAULT now(),
    expires_at TIMESTAMPTZ
);

-- Optional pgvector (if keeping PDF semantic search)
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE document_embeddings (
    id UUID PRIMARY KEY,
    file_id UUID REFERENCES uploaded_files(id),
    chunk_index INT,
    content TEXT,
    embedding vector(768)
);
```

### Go database layer: **sqlc + pgx/v5**

| Approach | Verdict for SmartKrishi |
|----------|------------------------|
| `database/sql` + raw SQL | Too verbose for 8 tables + JSON columns |
| GORM | Python uses SQLAlchemy but adds magic; JSON reasoning metadata awkward |
| sqlc + pgx | **Chosen** — compile-time query safety, native Postgres types, JSON support, matches hand-written SQL style in Agentic-AI |
| ent | Overkill for this schema |

**Justification:** Fixed schema, JSON columns (`reasoning_steps.step_metadata`, `agent_api_configs.default_tools`), no polymorphic associations. Alembic was never used — adopt `golang-migrate` with versioned SQL from day one.

### Connection pool (pgxpool)

```go
MaxConns: 20, MinConns: 5, MaxConnLifetime: 30m, MaxConnIdleTime: 5m
```

Match current SQLAlchemy `pool_size=5, max_overflow=10` but increase for streaming concurrency.

### Data migration (SQLite → Postgres)

One-time script to import Agentic-AI SQLite data if production has orphaned agent state:
- Map Agentic-AI `chat_id` → `chats.agent_chat_id`
- Skip message import if Postgres already has messages (Postgres is authoritative)

---

## Phase 10 — Error Handling & Reliability

| Failure | Expected behavior |
|---------|-------------------|
| **PostgreSQL down** | `/health` returns `unhealthy`; API returns 503; no partial writes |
| **Redis down (M2+)** | File uploads return 200 with sync fallback processing; cache miss → direct API call |
| **Gemini API timeout** | SSE `error` event + `end`; assistant message saved with error text |
| **Gemini rate limit** | Retry 2x with exponential backoff (1s, 4s); then error event |
| **File processing failure** | `processing_status: failed` in DB; SSE error if during analyze stream |
| **SMS API down** | Fallback activation returns 503; health endpoint shows degraded |
| **Worker crash** | In-flight job returned to queue (BRPOPLPUSH pattern); idempotency key prevents duplicate processing |

### Timeouts

| Layer | Timeout |
|-------|---------|
| HTTP server read | 30s (except streaming endpoints: unlimited read) |
| HTTP write (streaming) | 5min (match frontend) |
| DB query | 5s |
| External APIs (weather) | 5s |
| External APIs (market) | 10s |
| Gemini non-stream | 60s |
| Gemini stream | 5min (context deadline) |

### Graceful shutdown

```
SIGTERM → stop accepting connections → wait in-flight streams (30s) → drain worker pool → close pgxpool → exit
```

### Health vs readiness

- `/health` — liveness (process up)
- `/ready` (new, P1) — Postgres ping + Redis ping (if enabled) + disk writable

---

## Phase 11 — Observability

### Structured logging (slog)

Every request: `request_id`, `user_id`, `method`, `path`, `duration_ms`, `status`

### Metrics (Prometheus + OpenTelemetry — Milestone 3)

| Metric | Type | Labels |
|--------|------|--------|
| `http_request_duration_seconds` | histogram | method, path, status |
| `gemini_request_duration_seconds` | histogram | model, operation |
| `gemini_tokens_total` | counter | model, direction |
| `stream_events_total` | counter | event_type |
| `file_processing_duration_seconds` | histogram | file_type |
| `queue_depth` | gauge | queue_name |
| `worker_active` | gauge | worker_type |
| `db_query_duration_seconds` | histogram | query_name |

**OpenTelemetry:** Recommended for Gemini + HTTP client spans in Milestone 3; start with slog + Prometheus to avoid overhead in MVP.

### Benchmark targets (to measure, not invent)

Track p50/p95/p99 for: `send-stream` TTFB, full stream duration, chat list, file upload.

---

## Phase 12 — Performance & Benchmarking

### Comparison matrix (Go vs Python)

| Scenario | Tool | Compare |
|----------|------|---------|
| Chat list (50 items) | `k6` or `hey` | 100 concurrent, 60s |
| `send-stream` TTFB | Custom script | 20 concurrent streams |
| `send-stream` full duration | Same | Dominated by Gemini — expect similar |
| File upload 5MB PDF | `k6` multipart | 10 concurrent |
| Auth login | `hey` | 200 rps |
| DB connection saturation | `pgbench` + app load | Find pool sweet spot |

### How to measure (post-implementation)

1. Run Python baseline on same hardware (MacBook or Render instance)
2. Run Go on identical hardware with same `DATABASE_URL` and `GEMINI_API_KEY`
3. Use `k6` scripts in `tests/load/` with shared config
4. Record: rps, p50/p95/p99 latency, CPU (`top`), memory (`ps`), Postgres `pg_stat_activity`
5. Document Gemini latency separately (external dependency dominates streaming)

**Expected Go wins:** concurrent connection handling, memory per connection, file upload throughput, DB query latency under load, elimination of HTTP proxy hop (~5-20ms per stream chunk).

**Expected similar:** end-to-end streaming duration (Gemini-bound).

---

## Phase 13 — Testing Strategy

| Layer | Tool | Focus |
|-------|------|-------|
| Unit | `go test` + testify | auth JWT, tool registry, event parsing, password hash |
| Repository | testcontainers-go (Postgres) | CRUD, soft delete, reasoning inserts |
| API | httptest + golden files | Every `/api/v1` endpoint response shape |
| Streaming | Integration test | Parse SSE output; assert event sequence `plan → tool_call → response_chunk → response → end` |
| Concurrency | `go test -race` | parallel streams same chat, concurrent file upload |
| Agent tools | httptest mock servers | Weather/market API responses |
| Failure | Toxiproxy or mock | Gemini timeout, DB disconnect mid-stream |
| Load | k6 | 50 concurrent streams, measure goroutine count |
| Compatibility | Contract tests against saved Python responses | JSON field parity |

**Priority tests for Milestone 1:**
- Auth round-trip (signup → login → me)
- Chat CRUD
- `send-stream` event contract (mock Gemini)
- Health check

---

## Phase 14 — Migration Strategy (Incremental)

**Legend:** ✅ Done · 🟡 Partial · ⬜ Not started

| Step | Status | Implement | Replaces | Depends on | Tests | Verify |
|------|--------|-----------|----------|------------|-------|--------|
| 0 | ✅ | Create `archive/python-v1` + `feat/go-backend`; remove `Agentic-AI/` on migration branch | — | — | — | Commits `3c21082`, `b9a3f92` |
| 1 | ✅ | Go scaffold inside `backend/` | Python `backend/` | Step 0 | `go build` | Commit `b9a3f92`; server on `:8000` |
| 2 | 🟡 | Postgres repos + migrations | SQLAlchemy models | Step 1 | JWT tests only | Migration `1/u init` applied locally; user repo only; sqlc not generated |
| 3 | 🟡 | Email JWT auth (`signup`, `login`, `token`, `me`) | `auth.py` | Step 2 | `jwt_test.go` | Signup curl OK; frontend email login not E2E tested |
| 4 | ⬜ | Firebase mobile auth | `mobile_auth.py` | Step 3 | Firebase mock | Mobile login in UI |
| 5 | ⬜ | Chat CRUD endpoints | `chat.py` CRUD | Step 2 | API tests | Chat list/create in UI |
| 6 | ⬜ | Agent pipeline (planner, tools, executor) | `Agentic-AI/` core | GEMINI_API_KEY | Mock Gemini | Tool calls return data |
| 7 | ⬜ | SSE `send-stream` + `upload-and-analyze-stream` | `integrated_chat_service` | Steps 5–6 | SSE contract test | Full chat in UI |
| 8 | ⬜ | Reasoning persistence | `reasoning_service.py` | Step 7 | DB assertions | Reasoning panel in UI |
| 9 | ⬜ | File upload + processing (sync OK for M1) | `file_service`, `media.py` | Steps 5–6 | Upload test | File analyze in UI |
| 10 | ⬜ | Legacy endpoints (`/ask`, `/send`, image) | `ai/chat_service.py` | Step 6 | API tests | Manual |
| 11 | ⬜ | Redis + async file workers | Sync processing | Step 9, Redis | Queue tests | M2 |
| 12 | ⬜ | Fallback/SMS/Telegram | `fallback/*` | Steps 3–7, Redis | SMS mocks | M3 |
| 13 | ⬜ | Observability (metrics, `/ready`) | logging only | — | — | M3 |
| 14 | ⬜ | Promote `feat/go-backend` → `main`; tag `v1-python-final` | Python on `main` | All M1 criteria | Full regression | Production smoke test |

**Traffic switching:** Not started. When M1 complete: deploy `feat/go-backend` to staging Render → validate frontend → merge to `main`.

**Branch promotion checklist:**
- [ ] All Milestone 1 exit criteria pass on `feat/go-backend`
- [ ] `archive/python-v1` pushed to remote and tagged `v1-python-final`
- [x] `render.yaml` on migration branch uses Go build
- [ ] Frontend E2E against Go staging API
- [ ] Merge `feat/go-backend` → `main`
- [x] `archive/python-v1` exists locally (Python + Agentic-AI reference)

---

## Phase 15 — Prioritized Roadmap

### P0 — Required for basic Go migration (Milestone 1)

- [x] Project scaffold (cmd, config, server, middleware)
- [x] PostgreSQL migrations (all 8 tables) + pgxpool
- [ ] PostgreSQL repos for chats/messages/files (sqlc or pgx — users repo done)
- [x] Email JWT auth (`signup`, `login`, `token`, `me`)
- [ ] Mobile Firebase auth
- [ ] Chat CRUD (Postgres, soft delete)
- [ ] Agent pipeline inlined (planner, tools, executor)
- [ ] SSE `send-stream` + `upload-and-analyze-stream`
- [ ] Basic file upload (sync processing acceptable)
- [x] `/health`, CORS, graceful shutdown
- [ ] Contract tests for streaming events

### P1 — Important architectural improvements (Milestone 2)

- Redis file processing queue + worker pool
- Reasoning step batch persistence
- `gemini_file_registry` Postgres table
- Checker in streaming path (optional, redesign)
- `/ready` endpoint
- golang-migrate versioned schema
- Fix tool invocation signatures (file tools)
- Eliminate duplicate message inserts

### P2 — Performance/concurrency (Milestone 4)

- Parallel tool execution (errgroup)
- Gemini client pool with eviction
- Weather/market Redis cache
- pgvector for PDF search (if needed)
- k6 benchmarks vs Python baseline
- DB connection pool tuning

### P3 — Advanced distributed (Milestone 3)

- SMS/Telegram fallback with Redis leader election
- Distributed rate limiting
- OpenTelemetry tracing
- Circuit breaker on Gemini (gobreaker)
- Dead letter queue for failed jobs

### P4 — Nice-to-have (Milestone 5)

- Admin metrics dashboard
- Webhook-based SMS (replace long-poll)
- WhatsApp integration (schema exists, no code)
- API versioning `/api/v2`

---

## Milestones

### MILESTONE 1: Minimum viable Go migration (target: fastest working backend)

**Status:** ~25% complete (scaffold + DB schema + email auth only)

**Scope:** P0 items. Fallback deferred. Sync file processing acceptable.

**Deliverable:** Single Go binary replaces `backend/` + `Agentic-AI/` for auth + chat + streaming. Frontend works unchanged.

**Exit criteria:**
- [x] User can sign up, log in (email) — **done locally**
- [ ] User can log in (mobile / Firebase)
- [ ] User can create chat, send streaming message, upload file and get analysis stream
- [ ] SSE events match frontend parser expectations
- [x] Data persists in existing PostgreSQL schema — **schema + users; chat data pending**

### MILESTONE 2: Concurrency + Redis + async processing

**Scope:** P1 + Redis queue from Phase 6/7.

**Exit criteria:** File upload returns immediately; processing completes via workers; queue survives process restart.

### MILESTONE 3: Reliability + observability + fallback

**Scope:** P3 + fallback routes + Prometheus + `/ready`.

**Exit criteria:** SMS fallback works with single leader; metrics dashboard; graceful degradation documented.

### MILESTONE 4: Benchmarking + performance optimization

**Scope:** P2 + k6 suite + documented comparison report.

### MILESTONE 5: Advanced improvements

**Scope:** P4 items.

---

## Migration Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Gemini Go SDK API differences | High | Spike on `google.golang.org/genai` streaming in week 1 |
| SSE buffering on Render | High | `X-Accel-Buffering: no`; early flush testing |
| ChromaDB feature loss | Medium | Start Gemini File API only; add pgvector if search quality insufficient |
| Firebase Admin in Go | Medium | Use `firebase.google.com/go/v4` — spike token verify |
| DOCX processing (docx2pdf) | Medium | Call LibreOffice headless or defer DOCX to Milestone 2 |
| Data in Agentic-AI SQLite orphaned | Low | Postgres is authoritative; migration script optional |
| Long-poll SMS incompatible with serverless | High | Defer fallback; redesign to webhooks in M3 |

---

## What NOT to Change

1. **Frontend API paths** — all `/api/v1/*` URLs and HTTP methods
2. **JWT token format** — `sub` + `auth_provider` claims, HS256, same expiry
3. **SSE wire format** — `data: {json}\n\n` with existing event `type` values
4. **PostgreSQL schema** (Milestone 1) — column names and types
5. **Firebase mobile OTP flow** — frontend sends `id_token`; backend verifies
6. **CORS allowed origins** — keep Vercel URLs + localhost + `FRONTEND_URL`
7. **Soft delete semantics** — `is_deleted` flag, not hard delete
8. **File size limits** — 10MB upload-file, 20MB upload-and-analyze-stream
9. **UUID primary keys** for chats, messages, files

---

## Recommended Implementation Order

**Completed (2026-08-19):**
- [x] Day 1: Branch setup (`archive/python-v1`, `feat/go-backend`), Go scaffold, `render.yaml`
- [x] Week 1 (partial): Migrations + pgxpool + email JWT auth

**Remaining (pick up here with Claude):**
1. **Next:** Firebase mobile auth (Step 4)
2. Chat CRUD repos + endpoints (Step 5)
3. Gemini client spike + agent pipeline in `internal/agent/` (Step 6)
4. SSE `send-stream` end-to-end (Step 7)
5. Reasoning persistence (Step 8)
6. File upload + `upload-and-analyze-stream` (Step 9)
7. Legacy endpoints (Step 10)
8. Integration tests + frontend E2E + staging deploy
9. Milestone 2 (Redis workers) → Milestone 3 (fallback) → Milestone 4 (benchmarks)
10. Promote `feat/go-backend` → `main` (Step 14)

```mermaid
flowchart TB
    subgraph done [Completed]
        A0[Branch setup] --> A1[Go scaffold]
        A1 --> A2[Postgres migrations]
        A2 --> A3[Email JWT auth]
    end
    subgraph milestone1 [Milestone 1 - remaining]
        A3 --> B1[Firebase mobile auth]
        B1 --> B2[Chat CRUD]
        B2 --> B3[Agent pipeline inline]
        B3 --> B4[SSE streaming]
        B4 --> B5[File upload sync]
        B5 --> B6[Legacy endpoints + E2E]
    end
    subgraph milestone2 [Milestone 2]
        B6 --> H[Redis job queue]
        H --> I[Async file workers]
    end
    subgraph milestone3 [Milestone 3]
        I --> J[Fallback SMS redesign]
        J --> K[Observability]
    end
    subgraph milestone4 [Milestone 4]
        K --> L[Benchmarks vs Python]
    end
```
