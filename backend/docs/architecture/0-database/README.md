# Database Specification

- Status: Draft
- Owner: Backend Team
- Last updated: 21/01/2026

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

### users
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `uuid pk` | Primary Key |
| `username` | `text unique not null` | Unique username, used for login/handshake |
| `display_name` | `text null` | Optional display name |
| `created_at` | `timestamptz not null` | Default: `now()` |

### conversations
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `uuid pk` | Primary Key |
| `type` | `text not null` | Check: `('direct','group')` |
| `title` | `text null` | Title for group chats, null for direct |
| `direct_pair_key` | `text null` | Deterministic sorted pair of user IDs (e.g., `least:greatest`) for direct chats. Unique index on `(type, direct_pair_key)` where `direct_pair_key IS NOT NULL`. |
| `created_at` | `timestamptz not null` | Default: `now()` |

### conversation_members
| Column | Type | Description |
| :--- | :--- | :--- |
| `conversation_id` | `uuid fk` | References `conversations.id`. Part of PK. |
| `user_id` | `uuid fk` | References `users.id`. Part of PK. Indexed for "my conversations" lookups. |
| `role` | `text not null` | Default: `'member'` |
| `joined_at` | `timestamptz not null` | Default: `now()` |
| `last_read_message_id`| `uuid null fk` | References `messages.id`. Composite FK allows validation within the same conversation. |

### messages
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `uuid pk` | Primary Key |
| `conversation_id` | `uuid fk` | References `conversations.id` |
| `sender_id` | `uuid fk` | References `users.id` |
| `body` | `jsonb not null` | Message content (validated size) |
| `message_type` | `text not null` | Check: `('text','location','media')` |
| `client_msg_id` | `text not null` | Client-side ID for deduplication. Index: unique `(conversation_id, sender_id, client_msg_id)`. |
| `created_at` | `timestamptz not null` | Default: `now()`. Indexed: `(conversation_id, created_at, id)`. |

### delivery_receipts
| Column | Type | Description |
| :--- | :--- | :--- |
| `message_id` | `uuid fk` | References `messages.id`. Part of PK. |
| `user_id` | `uuid fk` | References `users.id`. Part of PK. |
| `status` | `text not null` | Check: `('delivered','read')`. Indexed: `(user_id, status)`. |
| `updated_at` | `timestamptz not null` | Default: `now()` |

### live_location_sessions
| Column | Type | Description |
| :--- | :--- | :--- |
| `id` | `uuid pk` | Primary Key (Session/Stream ID) |
| `user_id` | `uuid fk` | References `users.id`. Unique if `conversation_id` is NULL (global session). |
| `conversation_id` | `uuid null fk` | References `conversations.id`. Unique per user per conversation. |
| `expires_at` | `timestamptz not null` | Session expiry. Indexed: `(conversation_id, expires_at)`. |

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
- List conversations: `SELECT c.* FROM conversations c JOIN conversation_members cm ON c.id=cm.conversation_id WHERE cm.user_id=$1 ORDER BY c.created_at DESC`.

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
