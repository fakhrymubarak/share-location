# Messaging

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Durable messaging over WebSocket with `SEND_CHAT` and `RECEIVE_CHAT`, backed by Postgres persistence and Redis pub/sub for cross-node fan-out. Includes HTTP history retrieval and delivery receipts so offline clients can catch up.

## Goals
- Persist messages transactionally with idempotency via `(conversation_id, sender_id, client_msg_id)`.
- Broadcast `RECEIVE_CHAT` to all conversation members across nodes with ordering guarantees.
- Provide paginated history and delivery/read receipts over HTTP as a fallback.

## Non-goals
- End-to-end encryption or message search.
- Media upload/processing beyond message metadata.
- Rich threading/reactions; only flat messages.

## User flow
1) Client connects via `/ws` (handshake feature).
2) Client sends `SEND_CHAT` with `conversation_id`, payload, `message_type`, `client_msg_id`.
3) Server validates membership, writes to Postgres, publishes Redis `conv:{conversation_id}:messages`.
4) Sender receives `RECEIVE_CHAT` ack with `message_id` and `created_at`; other members receive the same event.
5) Clients fetch paginated history via `GET /v1/conversations/:id/messages` as needed.
6) Clients set receipts via `POST /v1/messages/:id/receipt` for delivered/read.

## Data sources
- Remote: `SEND_CHAT`/`RECEIVE_CHAT` WebSocket events, `GET /v1/conversations/:id/messages`, `POST /v1/messages/:id/receipt`.
- Local: Postgres tables `messages`, `conversation_members`, `delivery_receipts`; Redis channel `conv:{conversation_id}:messages`; connection registry for local fan-out.

## Architecture
- Presentation: WebSocket event handlers for send/receive; HTTP handlers for history and receipts.
- Domain: Message service (validation, idempotency, fan-out orchestration); receipt service.
- Data: Message repository (Postgres), Redis pub/sub publisher + subscriber, repository for receipts and membership lookups.

## Dependencies
- Fiber WebSocket + HTTP, Redis client (pub/sub), Postgres driver, UUID/time sources.

## State management
- In-memory connection registry for targeting; optional in-flight map for `client_msg_id` dedupe caching (scoped per conversation).
- Pagination cursors rely on persisted order: `ORDER BY created_at, id` from the database; fan-out uses the persisted row, not receive order.

## Error handling
- Validation failures (missing membership, bad payload) return `ERROR` with codes like `not_member` or `invalid_message`.
- DB or pub/sub failures return `ERROR` and log request id; if Redis publish fails, deliver locally and surface an alert/metric for cross-node delivery gap.
- Duplicate `(conversation_id, sender_id, client_msg_id)` returns the existing `message_id` and `created_at` for idempotent ack.

## Performance considerations
- DB indexes on `(conversation_id, created_at, id)`; limit page size.
- Backpressure: cap outstanding send requests per connection and enforce payload size limits.
- Redis fan-out to reduce n^2 delivery; local broadcast only to connected members.
- Ordering guarantee is derived from the DB write (`created_at`, `id`), not subscription receive order; clocks should be server-side sourced.

## Security/Privacy
- Enforce membership authorization for send/history/receipts.
- Strip/validate message body to avoid script injection in downstream clients.
- Use TLS for WebSocket/HTTP transport.

## Analytics/telemetry
- Counters: messages persisted, fan-out successes/failures, receipt updates.
- Timers: send-to-ack latency, DB write latency, Redis publish-to-delivery lag.

## Testing plan
- Unit: membership validation, idempotency logic, message serialization.
- Integration: WebSocket send/receive across two simulated users; pagination ordering; receipt updates.

## Rollout/flags
- Optional flag to enable Redis Streams instead of pub/sub if durability requirements rise.
- Configurable limits for max message size and per-connection in-flight sends.

## Decisions
- No server-side filtering by `message_type`; clients should ignore unsupported types. If needed later, add a client-declared capability flag.
- History pagination stays on `created_at`/`id` ordering; no `before_message_id` cursor in v1. Can add compatible cursor later without breaking contract.
