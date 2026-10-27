# Project Chat

A simple self-hostable chat website: username/password accounts, profile pictures
and status, one shared lobby plus 1-on-1 DMs, and text (emoji), image and video
messages. Nothing else.

Everything is open source and runs on free resources: a single self-hosted VM
(Docker), self-hosted PostgreSQL, self-hosted MinIO and Caddy for automatic HTTPS
via a free DuckDNS subdomain.

## Stack

| Layer    | Choice                                            |
|----------|---------------------------------------------------|
| Frontend | React 18 + Vite, served (embedded) by the Go binary |
| Backend  | Go 1.22, stdlib `net/http` + `gorilla/websocket`  |
| Database | PostgreSQL 16 (self-hosted, Docker)               |
| Files    | MinIO (self-hosted, Docker), served auth-gated    |
| Auth     | bcrypt + JWT (30-day), cookie + Bearer            |
| Proxy/TLS| Caddy 2 (auto-HTTPS, DuckDNS HTTP-01)             |

## How it works

- One Go binary serves everything: the built React app (via `go:embed`), the
  REST API and the WebSocket endpoint. No CORS, no second deployment.
- **REST**: register/login, profile (name, picture, status), user search,
  conversations (lobby + DMs), paginated message history, file upload
  (25 MB cap) into MinIO, authenticated file streaming.
- **WebSocket** (`/ws`, JWT-gated): live messages, room join/leave, typing
  indicators, and online/offline presence.
- **Files**: uploads go to a private MinIO bucket; every fetch streams through
  the Go server with JWT auth and HTTP `Range` support so `<video>` seeking works.
- **Lobby**: a single conversation every user is auto-joined to at registration.
  DMs are conversations with exactly two members. Presence updates broadcast via
  the lobby room (every connected client is there).

## Database

Four tables (created automatically from `migrations.sql` on boot):

```
users                 id, username (unique), password_hash, profile_pic,
                      status, last_online, created_at
conversations         id, kind ('lobby'|'dm'), name, created_at
conversation_members  conversation_id, user_id
messages              id, conversation_id, sender_id, type ('text'|'image'|'video'),
                      content, file_url, created_at
```

Message `content` stores raw UTF-8 text — emoji are plain Unicode, no special
handling needed.

## Project layout

```
├── main.go            entry point, routing, embedded frontend
├── config.go          env config
├── db.go              pgx pool, migrations, lobby seed
├── models.go          User / Conversation / Message
├── auth.go            bcrypt + JWT + cookie helpers
├── store.go           all SQL
├── obj.go             MinIO client + bucket bootstrap
├── files.go           authenticated, range-aware file proxy
├── handlers.go        REST handlers
├── ws.go              WebSocket hub (rooms, presence, typing)
├── migrations.sql     schema
├── web/               React + Vite frontend
└── deploy/            Docker, compose, Caddy, setup script
```

## Local development (optional)

```bash
# 1. start Postgres + MinIO locally, e.g.:
#    docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=chat postgres:16-alpine
#    docker run -d -p 9000:9000 -p 9001:9001 minio/minio server /data --console-address ":9001"

# 2. backend (defaults: postgres://chat:chat@localhost:5432/chatdb, minio at minio:9000)
#    adjust via env if needed, then:
DATABASE_URL=postgres://chat:chat@localhost:5432/chatdb go run .

# 3. frontend dev server with API proxy:
cd web && npm install && npm run dev     # http://localhost:5173
```

## Deployment

Full walkthrough for the Oracle Always Free VM (or any Ubuntu 22.04+ VPS) in
[`DEPLOY.md`](DEPLOY.md). In short:

```bash
git clone <repo> && cd project-chat
vi deploy/.env   # or just run the setup script
docker compose -f deploy/docker-compose.yml up -d --build
```

`deploy/setup.sh` generates secrets, writes `.env`, and starts everything.

## Production notes / tradeoffs

- JWT is kept in `localStorage` too (needed for WebSocket and `<img>/<video>`
  URLs). That's a known XSS tradeoff; for a hobby chat it's acceptable. Fixing
  it means serving media through signed short-lived URLs or service workers.
- Presence is in-memory (per Go process) + `last_online` in Postgres. Fine for
  a single instance.
- No rate limiting / moderation — open signup as requested. Add later if needed.
- Everything lives in one VM, so backups = `pg_dump` + MinIO `mc mirror` (see
  DEPLOY.md for commands).