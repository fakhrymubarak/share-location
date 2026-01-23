# Technical Architecture – Messaging Backend

- Status: Approved
- Owner: Backend Team
- Last updated: 23/01/2026

Role: Technical Architect. Goal is a simple, scalable backend for chat and live location built with Go Fiber, PostgreSQL, and Redis. Auth is intentionally simple: users identify by username only (no JWT/password). The server creates or looks up the user on login/connect.

## Goals
- Provide a WebSocket endpoint that supports `SEND_CHAT`, `RECEIVE_CHAT`, `PUBLISH_LIVE_LOCATION`, and `SUBSCRIBE_LIVE_LOCATION`.
- Keep message ordering and durability while enabling fan-out to multiple clients.
- Deliver low-latency live location updates with sensible TTLs and backpressure.
- Stay horizontally scalable with minimal complexity.

## High-Level Architecture
- **API + WebSocket Gateway (Fiber):** Handles username-based login/upsert, HTTP endpoints, and `/ws` upgrade. Maintains in-process connection registry keyed by user id; subscribes to Redis for cross-node fan-out.
- **Message Service:** Persists chat messages to Postgres in a transactional flow. Publishes delivery events to Redis channels for other nodes.
- **Live Location Service:** Receives location updates over WebSocket, validates rate limits, stores ephemeral state in Redis, and publishes updates.
- **PostgreSQL:** Source of truth for users, conversations, membership, messages, delivery receipts.
- **Redis:** Pub/Sub for cross-node WebSocket broadcast, caches for session/presence, and ephemeral live-location streams.
- **Background Workers:** Optional little workers for read-repair (e.g., retry failed deliveries) and cleanup (expire live-location streams).

## Data Model (PostgreSQL)
- `users(id, username unique, display_name, created_at)`
- `conversations(id, type enum[direct, group], title, created_at)`
- `conversation_members(conversation_id, user_id, role, joined_at, last_read_message_id)`
- `messages(id uuid, conversation_id, sender_id, body jsonb, message_type enum[text, location, media], client_msg_id, created_at)`
- `delivery_receipts(message_id, user_id, status enum[delivered, read], updated_at)`
- `live_location_sessions(id uuid, user_id, conversation_id nullable, expires_at)` — tracks active publishers/subscribers.

## WebSocket Contract
Single endpoint: `GET /ws` with query `?username=<string>` (or header `X-Username`). Server upserts the user by username if missing; missing username returns an `ERROR`. Subprotocol: `json`. All frames use `{ "event": "<NAME>", "data": {...}, "req_id": "<uuid>" }`.

- `CONNECTED` (server -> client, on handshake)  
  - Sent after the server upserts the user. Payload includes `user_id` and `username` so the client knows its resolved id.

- `SEND_CHAT` (client -> server)  
  - `conversation_id`, `body` (text or payload), `message_type`, `client_msg_id` (for idempotency).  
  - Server validates membership, writes message in Postgres, publishes to Redis channel `conv:{conversation_id}:messages`.  
  - Ack: `RECEIVE_CHAT` to sender with persisted `message_id`, `created_at`, and echoed `client_msg_id`.
- `RECEIVE_CHAT` (server -> clients)  
  - Broadcast to all conversation members (online) across nodes using Redis pub/sub; local node fans out over in-memory connection registry.
- `PUBLISH_LIVE_LOCATION` (client -> server)  
  - `stream_id` (uuid, stable for session), `conversation_id`, `lat`, `lng`, `accuracy`, `speed`, `heading`, `expires_at`.  
  - Stored in Redis hash `live:{stream_id}` with TTL; published to channel `live:{conversation_id}`. Rate limited to prevent abuse.
- `SUBSCRIBE_LIVE_LOCATION` (client -> server)  
  - `conversation_id` or `stream_id`. Server attaches client to Redis subscription and replays latest cached point if present.  
  - Updates sent as `SUBSCRIBE_LIVE_LOCATION` events with payload `{stream_id, conversation_id, point, updated_at}`.

Errors return `{ "event": "ERROR", "req_id": "<req>", "code": "<string>", "message": "<details>" }`.

## HTTP Endpoints (Fiber)
- `POST /v1/login` {username} — idempotently create or fetch the user and return `user_id`.
- `POST /v1/conversations` create direct/group conversations; payload can include participant usernames so users can be added by username (server upserts users).
- `GET /v1/conversations` list conversations (paginated) for the authenticated user.
- `GET /v1/conversations/:id/messages` paginated history (uses message ordering by `created_at` and `id`).
- `POST /v1/messages/:id/receipt` set delivered/read.
- `POST /v1/live-locations` start/end publisher session (creates `stream_id`, sets TTL).
These support WebSocket flows and provide fallbacks for clients that reconnect.

## Connection and Delivery Flow
- On `SEND_CHAT`, server writes to Postgres within a transaction and emits Redis pub/sub event. To avoid duplicates, dedupe by `(conversation_id, sender_id, client_msg_id)`.
- Delivery ordering derives from the persisted row (`created_at`, `id`), not subscription receive order; fan-out uses the stored message payload.
- Each node subscribes to Redis and forwards events to connected users. If Redis publish/subscription fails, deliver locally, alert/metric the gap, and rely on history pulls for cross-node recovery until Redis reconnects.
- Live location uses Redis TTL to auto-expire; a cleanup worker trims stale sessions and stops broadcasting expired streams.

## Caching and Redis Keys
- `ws:user:{user_id}` set of connection ids for tracing and broadcast targeting.
- `presence:{user_id}` presence state with TTL refresh heartbeats.
- `conv:{conversation_id}:messages` Redis channel for chat fan-out.
- `live:{conversation_id}` channel and `live:{stream_id}` hash (latest point) with TTL capped by server-configured max (default 15 minutes).

## Scalability and Reliability
- Stateless API nodes; scale horizontally behind a load balancer with sticky sessions optional (connection registry handles either).
- Redis pub/sub keeps WebSocket fan-out in sync across nodes; consider Redis Stream for guaranteed delivery if requirements rise.
- Use DB indexes on `messages(conversation_id, created_at, id)` and `conversation_members(conversation_id, user_id)`.
- Backpressure: limit concurrent in-flight requests per connection and cap live-location publish rate.
- Observability: request/connection logs, message delivery latency metrics, and alert on Redis/DB saturation.

## Security Basics
- No auth beyond username; avoid storing secrets. Enforce conversation membership checks with user ids resolved from username.
- Validate payload sizes and types; cap message length and strip HTML/JS.
- Use TLS for transport even without auth to avoid sniffing.

## Minimal Implementation Order
1) Wire `/ws` with username handshake (upsert user) and JSON frame parsing.  
2) Implement message persistence + Redis pub/sub broadcast.  
3) Add history and receipt HTTP endpoints.  
4) Add live-location publish/subscribe with Redis TTL.  
5) Add metrics/logging and rate limits.

This architecture keeps the system small while supporting chat and live location in a horizontally scalable way on Fiber, PostgreSQL, and Redis.
