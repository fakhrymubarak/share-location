# WebSocket Handshake

- Status: Approved
- Owner: Backend Team
- Last updated: 23/01/2026

## Overview
WebSocket upgrade and handshake flow that upserts a user by `username`, binds the connection to the resolved `user_id`, and sends a `CONNECTED` event. Maintains an in-process connection registry (plus optional Redis presence keys) to enable fan-out and observability across nodes.

## Goals
- Accept `GET /ws?username=` (or `X-Username`) and upsert user idempotently
- Establish connection registry keyed by `user_id` for targeted fan-out
- Emit `CONNECTED` payload with `user_id` and `username` once setup succeeds

## Non-goals
- Strong authentication beyond username-only identification
- Token/JWT issuance or session cookies
- Presence UX; only internal presence for routing/metrics

## User flow
1) Client opens `GET /ws?username=<name>` with JSON subprotocol.
2) Server validates non-empty username, upserts `users` row, and upgrades connection.
3) Connection id is stored under the caller’s `user_id` (local map + optional `ws:user:{user_id}` set in Redis).
4) Server sends `CONNECTED` with `user_id`, `username`; client proceeds with chat/location events.

## Data sources
- Remote: `GET /ws?username=`, optional fallback `POST /v1/login`.
- Local: Postgres `users`, Redis `ws:user:{user_id}` (optional), in-process connection registry.

## Architecture
- Presentation: Fiber `GET /ws` handler + JSON frame parser/serializer.
- Domain: Handshake/use-case service to upsert user and bind connection metadata.
- Data: User repository (Postgres), presence publisher (Redis), connection registry component.

## Dependencies
- Fiber WebSocket support, Redis client, Postgres driver, UUID generator.

## State management
- Connection registry keyed by `user_id -> []connectionIds`; refresh TTL for `presence:{user_id}` if enabled.
- Heartbeat: server pings every 30s; idle timeout at 60s (2x heartbeat). Both intervals are configurable (e.g., `WS_HEARTBEAT_INTERVAL_SECONDS`, `WS_IDLE_TIMEOUT_SECONDS`). Disconnects evict registry entries and expire `ws:user:{user_id}` / `presence:{user_id}` keys.
- Track subprotocol and client metadata (IP/user-agent) for tracing.

## Error handling
- Reject missing/blank username with `ERROR` event (code `invalid_username`).
- Handle upsert/DB errors with `ERROR` (code `user_upsert_failed`); close connection on fatal errors.
- Guard against duplicate connection ids; clean registry on disconnect/ping timeout.

## Performance considerations
- Limit concurrent handshakes and frame size during upgrade.
- Optional stickiness is not required; cross-node fan-out goes through Redis.
- Lightweight presence TTL updates to avoid Redis churn.

## Security/Privacy
- Enforce input validation (length caps, printable characters).
- Require TLS termination; drop unexpected subprotocols.
- Avoid storing secrets; log usernames minimally with request ids.

## Analytics/telemetry
- Counters: handshake success/failure, `ERROR` by code, concurrent connections.
- Timers: handshake duration, time-to-first-frame.

## Testing plan
- Unit: username validation, user upsert call, registry insert/remove.
- Integration: WebSocket upgrade end-to-end emitting `CONNECTED`; disconnect cleanup.

## Rollout/flags
- Flag to toggle Redis-backed presence vs in-process only.
- Configurable handshake rate limit per IP.

## Decisions
- Allow multi-tab connections; registry supports multiple connections per user id and cleans up via heartbeat/timeout.
- Presence remains internal-only; no client-facing presence exposure in the initial contract.
