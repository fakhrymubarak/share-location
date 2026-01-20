# Database Specification

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Relational source of truth for users, conversations, memberships, durable chat messages, delivery receipts, and live-location session metadata. Built on PostgreSQL with UUID primary keys, strict foreign keys, and indexes optimized for messaging order and membership checks.

## Goals
- Provide normalized schema for chat and live location features with clear ownership.
- Enable efficient writes/reads for per-conversation history and membership validation.
- Keep migrations predictable and reversible for fast iteration.

## Non-goals
- Full-text search or analytics-grade queries.
- Media storage; only metadata is stored here.
- Long-term storage of live location points (only session metadata lives in Postgres).

## Tables
- `users`
  - Columns: `id uuid pk`, `username text unique not null`, `display_name text null`, `created_at timestamptz not null default now()`.
  - Notes: `username` uniqueness enforced; username is used for upsert on login/handshake.

- `conversations`
  - Columns: `id uuid pk`, `type text not null check in ('direct','group')`, `title text null`, `direct_pair_key text null`, `created_at timestamptz not null default now()`.
  - Notes: Direct conversations are unique per user pair. `direct_pair_key` stores a deterministic sorted pair of user ids (e.g., `least:greatest`) for direct convos and is `NULL` for groups. Enforce uniqueness via a partial unique index on `(type, direct_pair_key)` where `direct_pair_key IS NOT NULL`; creation returns the existing convo if it already exists.

- `conversation_members`
  - Columns: `conversation_id uuid fk -> conversations.id`, `user_id uuid fk -> users.id`, `role text not null default 'member'`, `joined_at timestamptz not null default now()`, `last_read_message_id uuid null fk -> messages.id`.
  - PK: `(conversation_id, user_id)`.
  - Indexes: `(conversation_id, user_id)` via PK; additional index on `(user_id)` for “my conversations” lookups.
  - Notes: `last_read_message_id` must belong to the same conversation (composite FK on `(conversation_id, last_read_message_id)` referencing `messages(conversation_id, id)` to prevent cross-conversation pointers). Used for membership validation and receipt rollups.

- `messages`
  - Columns: `id uuid pk`, `conversation_id uuid fk`, `sender_id uuid fk -> users.id`, `body jsonb not null`, `message_type text not null check in ('text','location','media')`, `client_msg_id text not null`, `created_at timestamptz not null default now()`.
  - Index: unique on `(conversation_id, sender_id, client_msg_id)` for idempotency within a conversation; query index on `(conversation_id, created_at, id)`; optional index on `(sender_id, created_at)` if “sent by me” queries are needed.
  - Notes: `body` stays small (validated size); `client_msg_id` from client for dedupe scoped to conversation.

- `delivery_receipts`
  - Columns: `message_id uuid fk -> messages.id`, `user_id uuid fk -> users.id`, `status text not null check in ('delivered','read')`, `updated_at timestamptz not null default now()`.
  - PK: `(message_id, user_id)`.
  - Index: on `(user_id, status)` for reporting; consider partial index on `status='read'`.

- `live_location_sessions`
  - Columns: `id uuid pk`, `user_id uuid fk -> users.id`, `conversation_id uuid null fk -> conversations.id`, `expires_at timestamptz not null`.
  - Index: on `(conversation_id, expires_at)` for cleanup; `(user_id)` for publisher lookup.
  - Constraints: unique `(user_id, conversation_id)` where `conversation_id IS NOT NULL` to allow only one active session per user per conversation; unique `(user_id)` where `conversation_id IS NULL` to prevent unlimited global sessions. Adjust partial uniques if global sessions are not allowed.
  - Notes: Stores session metadata only; actual points are in Redis.

## Constraints and integrity
- Foreign keys on all relationships; cascading deletes are avoided—cleanup is explicit.
- Check constraints for enumerated types; consider real Postgres enums if stability is preferred.
- Timestamps use `timestamptz`; always set defaults in migrations.

## Access patterns
- Message history: `WHERE conversation_id = $1 ORDER BY created_at, id LIMIT $page`.
- Membership check: `SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2`.
- Idempotent send: `INSERT ... ON CONFLICT (conversation_id, sender_id, client_msg_id) DO UPDATE RETURNING id, created_at`.
- Receipts: upsert on `(message_id, user_id)` to move from delivered->read.
- Live location session lookup: `SELECT * FROM live_location_sessions WHERE id=$1 AND expires_at>now()`.

## Migrations
- Use additive migrations with down steps; run within transactions.
- Default to `uuid_generate_v4()` if extension is available; otherwise generate in app layer.
- Seed indexes alongside table creation to avoid locking later.

## Performance considerations
- Keep `body` size bounded; add check constraint for max length if needed.
- Covering index `(conversation_id, created_at, id)` supports pagination without extra sorts.
- Avoid large transactions; batch inserts for membership creation.

## Security/Privacy
- Limit `username` and `title` length; store minimal PII (no passwords).
- Ensure application-level authorization for membership before message/receipt operations.
- Use TLS for DB connections; restrict DB user privileges to needed tables only.

## Observability
- Log query timings on slow thresholds; capture migration versions.
- Add metrics for insert/update failures and row counts (messages, conversations).

## Decisions
- Enforce one direct conversation per user pair via a uniqueness check (service-level or constraint); return the existing conversation if a duplicate request arrives.
- No soft deletes for messages in v1; deletes occur only via explicit admin tooling or retention jobs if added later.
