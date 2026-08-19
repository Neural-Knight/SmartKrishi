# SmartKrishi

Web application that helps farmers get crop advice through an AI chat interface. Users can send messages, upload photos and documents, and receive streamed responses with visible reasoning steps.

## Project structure

```
SmartKrishi/
├── frontend/     React + Vite + TypeScript
├── backend/      Go API and agent service
└── render.yaml     Backend deployment (Render)
```

## Stack

| Component | Technology |
|-----------|------------|
| Frontend | React, TypeScript, Vite, Tailwind, Firebase Auth |
| Backend | Go, chi, pgx, Google Gemini |
| Database | PostgreSQL |
| Hosting | Vercel (frontend), Render (API) |

The backend runs as one process: REST API, authentication, chat storage, file handling, and a Gemini-based agent pipeline with server-sent events (SSE) for streaming replies.

## Local setup

**Requirements:** Go 1.22+, Node 18+, Docker, pnpm

### Backend (port 8000)

```bash
cd backend
docker compose up -d
cp .env.example .env
# Edit .env: DATABASE_URL, SECRET_KEY, GEMINI_API_KEY, Firebase credentials

set -a && source .env && set +a
make migrate-up
make run
```

Verify: `curl http://localhost:8000/health`

### Frontend (port 5173)

```bash
cd frontend
cp .env.example .env
# Set VITE_API_BASE_URL=http://localhost:8000 and Firebase client keys

pnpm install
pnpm dev
```

Open http://localhost:5173

## Configuration

Environment templates:

- [`backend/.env.example`](backend/.env.example) — database, JWT, Gemini, Firebase, optional tool API keys
- [`frontend/.env.example`](frontend/.env.example) — API base URL and Firebase client config

`GEMINI_API_KEY` is required for chat and file analysis. Weather and market price tools use optional keys documented in the backend README.

## Deployment

- **API:** Render via [`render.yaml`](render.yaml). Apply database migrations against the production `DATABASE_URL` after deploy.
- **Frontend:** Set `VITE_API_BASE_URL` to the Render service URL. Backend CORS uses `FRONTEND_URL`.

## Documentation

API layout, endpoints, and development commands: [`backend/README.md`](backend/README.md)
