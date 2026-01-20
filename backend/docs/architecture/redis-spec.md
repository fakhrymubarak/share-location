# Redis Specification

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Redis supports pub/sub fan-out for chat and live location, ephemeral live-location state, lightweight presence/connection hints, and optional cross-node coordination. Keys are namespaced to avoid collisions and use TTLs to prevent unbounded growth.

## Goals
- Provide channels and key schemas for cross-node WebSocket delivery.
- Store ephemeral live-location points with TTL.
- Offer optional presence/connection mapping to target online users.

## Non-goals
- Durable messaging; Postgres remains the source of truth.
- Complex presence semantics beyond simple connection tracking and TTLs.
- Long-term analytics storage.

## Keyspaces and channels
- Pub/Sub channels
  - `conv:{conversation_id}:messages`: broadcast persisted chat messages to subscribers on all nodes.
  - `live:{conversation_id}`: broadcast live location updates per conversation.

- Hashes
  - `live:{stream_id}`: latest live-location point `{lat,lng,accuracy,speed,heading,updated_at,conversation_id,user_id}` with TTL matching session expiry.

- Sets (optional, for multi-node presence)
  - `ws:user:{user_id}`: set of active connection ids; TTL refreshed on heartbeat; cleaned on disconnect.

- Strings (optional)
  - `presence:{user_id}`: simple presence marker with short TTL for coarse online checks.

## TTL strategy
- `live:{stream_id}` TTL set to session expiry but capped by a server-enforced max (default 15 minutes) configured via `LIVE_LOCATION_MAX_TTL_SECONDS`; cleanup worker can extend/trim as needed.
- Presence/connection sets expire quickly (default 90s) and are refreshed by heartbeats from the connection layer (ping every 30s, timeout 60s). TTL is configurable (e.g., `WS_PRESENCE_TTL_SECONDS`); disconnect timeout should remove/expire connection ids proactively.
- Pub/Sub channels rely on subscription only; no TTL.

## Access patterns
- Publish chat: `PUBLISH conv:{conversation_id}:messages <payload>`.
- Subscribe chat: long-lived subscription per node; deliver to in-process registry.
- Publish live location: `HSET live:{stream_id} ...` + `EXPIRE` + `PUBLISH live:{conversation_id} <payload>`.
- Subscribe live location: attach to `live:{conversation_id}` and fetch cached `HGETALL live:{stream_id}` for replay.
- Presence: add/remove connection ids in `ws:user:{user_id}`; optionally read membership to find online targets.

## Performance considerations
- Keep payloads small; avoid large hashes or channel fan-out beyond expected member counts.
- Use pipeline for `HSET` + `EXPIRE` during publish to reduce RTT.
- Avoid heavy key scanning; keys are deterministic by id.

## Reliability and cleanup
- On subscriber restart, resubscribe to both chat and live-location channels and replay cached hashes where applicable.
- If Redis is unavailable, degrade gracefully: continue local delivery, surface metrics/alerts for cross-node gaps, and rely on history endpoints until Redis reconnects.
- Cleanup worker: prune stale `ws:user:*` sets and check `live_location_sessions` vs Redis TTL drift.

## Security/Privacy
- No sensitive data stored; still avoid logging live-location payloads.
- Use AUTH/TLS to Redis; limit commands to needed primitives (no scripting required).
- Input validation before publishing to avoid injecting unexpected keys.

## Observability
- Metrics: publish/subscribe counts, failures, reconnects, TTL expirations, rate-limit hits.
- Logs: subscription drops, Redis errors with request ids; optional tracing tags on publish latency.

## Decisions
- Stick with pub/sub for v1; Redis Streams can be enabled behind a feature flag if durability requirements rise.
- Max TTL for `live:{stream_id}` is enforced server-side (default 15 minutes) regardless of client request.
