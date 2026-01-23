# Live Location

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Live location publish/subscribe over WebSocket with ephemeral storage in Redis. Publishers create a session (`stream_id`), send `PUBLISH_LIVE_LOCATION` updates that are cached with TTL and broadcast over `live:{conversation_id}` channels. Subscribers attach via `SUBSCRIBE_LIVE_LOCATION` and receive updates plus the latest cached point.

## Goals
- Allow clients to start/close live-location sessions and obtain a `stream_id`.
- Deliver low-latency location updates to conversation members with sensible TTLs (server-enforced max; default 15 minutes).
- Enforce rate limits and membership authorization for publish/subscribe.

## Non-goals
- Long-term location history or geo-fencing logic.
- Client-side map rendering guidance.
- Background tracking when the app is killed.

## User flow
1) Client starts session with `POST /v1/live-locations` and receives `stream_id` and TTL.
2) Client publishes via `PUBLISH_LIVE_LOCATION` with coordinates, `stream_id`, `conversation_id`, and `expires_at`.
3) Server validates membership/rate limits, stores in Redis hash `live:{stream_id}` with TTL, and publishes to `live:{conversation_id}`.
4) Subscriber sends `SUBSCRIBE_LIVE_LOCATION` with `conversation_id` or `stream_id`; server attaches to Redis subscription and replays cached point if present.
5) Updates continue until TTL expiry or explicit session close; cleanup worker removes stale streams.

## Data sources
- Remote: `POST /v1/live-locations`, `PUBLISH_LIVE_LOCATION`, `SUBSCRIBE_LIVE_LOCATION`.
- Local: Redis hash `live:{stream_id}` + channel `live:{conversation_id}`, Postgres `live_location_sessions`, optional presence data for connected members.

## Architecture
- Presentation: WebSocket handlers for publish/subscribe; HTTP handler for session create/close.
- Domain: Live location service handling validation, rate limiting, TTL management, and subscription orchestration.
- Data: Redis storage/publishers, Postgres session repository, connection registry for targeted fan-out.

## Dependencies
- Redis client with pub/sub and TTL, Fiber WebSocket/HTTP, UUID/time utilities, rate limiter middleware/util.

## State management
- Store latest point per `stream_id` with TTL; clamp TTL to a server-defined maximum (default 15 minutes) via config like `LIVE_LOCATION_MAX_TTL_SECONDS` even if client requests longer.
- Track publishers per `user_id` and active subscriptions per connection.
- Optional heartbeat/cleanup worker to trim expired sessions and drop subscriptions.

## Error handling
- Reject publishes from non-members or expired sessions (`ERROR` codes like `not_member`, `session_expired`).
- Validate coordinate bounds, accuracy, and payload size; rate-limit violators receive `ERROR rate_limited`.
- If client-supplied `expires_at` exceeds the max TTL, clamp to the configured max (default 15 minutes) and return the effective expiry, or error with `ttl_exceeded` if strict mode is enabled.
- Redis/DB failures return `ERROR` and may force subscription teardown to avoid stale streams.

## Performance considerations
- Cap publish rate per connection; coalesce updates server-side if a connection is slow.
- Use Redis TTL to free memory automatically; avoid writing every update to Postgres.
- Keep payload small (lat/lng/accuracy/speed/heading only).

## Security/Privacy
- Enforce membership for both publish and subscribe; do not leak stream data across conversations.
- Require TLS; ensure location data is not logged beyond coarse metrics.

## Analytics/telemetry
- Counters: active sessions, publishes, subscribes, rate-limit hits.
- Timers: publish-to-deliver latency; TTL expiry counts.

## Testing plan
- Unit: coordinate validation, TTL handling, rate limiting, membership checks.
- Integration: start session, publish points, subscribe and verify replay/broadcast across two clients.

## Rollout/flags
- Configurable publish rate limits and TTL defaults; flag to disable live location globally.
- Optional switch to Redis Streams if stronger delivery guarantees are needed later.

## Decisions
- Subscriptions remain explicit to a conversation or stream id; no auto-subscribe by `user_id` presence in v1 to avoid noisy fan-out.
- One active session per user per conversation (and one global session per user if `conversation_id` is omitted), aligned with DB partial unique constraints; renew by ending and recreating.
