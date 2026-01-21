# Share Location Backend

Backend service for chat, conversations, and live-location sharing. Supports WebSocket messaging with Redis fan-out, Postgres persistence for core data, and optional in-memory mode for quick starts.

## Requirements
- Go 1.24+
- (Optional) Docker + Docker Compose (for Postgres + Redis)
- Copy `.env.example` to `.env` and set real values (DB/Redis URLs, logging prefs). Compose and local runs will read these.

## Build & Run
- Local (in-memory defaults; ignores DB/Redis):
  ```bash
  go test ./...
  go run ./cmd/server
  ```
- With Docker Compose (Postgres + Redis):
  ```bash
  cp .env.example .env  # edit passwords/urls if needed
  docker compose up --build
  ```
  Compose reads `.env` for `POSTGRES_*`, `DB_URL`, `REDIS_URL`, and logging flags.
- Config loading: YAML under `config/` (see `docs/architecture/environment-config.md`) plus env vars like `DB_URL`, `REDIS_URL`, `LOG_LEVEL`, `LOG_JSON`. Env vars override YAML.

## HTTP API
- `POST /v1/login` — body `{"username":"alice"}`; returns `user_id`, `username`.
- `POST /v1/conversations` — body `{"type":"direct|group","title":"...","usernames":["alice","bob"]}`; returns conversation data and member IDs.
- `GET /v1/conversations/:id/messages?limit=50` — paginated history ordered by `created_at,id`.
- `POST /v1/messages/:id/receipt` — body `{"user_id":"<uuid>","status":"delivered|read"}`.
- `POST /v1/live-locations` — body `{"user_id":"<uuid>","conversation_id":"<uuid>","ttl_seconds":900}`; returns `stream_id` and `expires_at`.
- `GET /health` — liveness check.

## WebSocket API (`/ws?username=<name>`)
- On connect: server upserts user and sends `{"event":"CONNECTED","data":{"user_id","username"}}`.
- `SEND_CHAT` — client frame:
  ```json
  {"event":"SEND_CHAT","req_id":"r1","data":{"conversation_id":"<uuid>","body":{"text":"hi"},"message_type":"text","client_msg_id":"c1"}}
  ```
  Server replies with `RECEIVE_CHAT` echoing stored fields. Broadcasts to other members locally or via Redis pub/sub.
- `PUBLISH_LIVE_LOCATION` — client frame:
  ```json
  {"event":"PUBLISH_LIVE_LOCATION","req_id":"r2","data":{"stream_id":"<uuid>","conversation_id":"<uuid>","lat":1.23,"lng":2.34,"accuracy":3,"speed":0,"heading":0,"expires_at":"<RFC3339>" }}
  ```
  Server replies with `SUBSCRIBE_LIVE_LOCATION` containing latest point and broadcasts to other members.
- `SUBSCRIBE_LIVE_LOCATION` — client frame:
  ```json
  {"event":"SUBSCRIBE_LIVE_LOCATION","req_id":"r3","data":{"stream_id":"<uuid>","conversation_id":"<uuid>"}}
  ```
  Server responds with cached point when available.
- Errors: server sends `{"event":"ERROR","req_id":"rX","data":{"code":"invalid_input|forbidden|not_found|internal","message":"..."}}`.

## Docs
- Architecture: `docs/architecture/*.md` (DB schema, Redis, Docker, environment config).
- Feature specs: `docs/features/*.md` (handshake, messaging, conversations, live location).
