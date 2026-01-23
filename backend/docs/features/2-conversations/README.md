# Conversations

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Conversation creation and membership management using username inputs. Supports direct and group conversations via `POST /v1/conversations`, upserting users as needed, and establishing `conversation_members` records used for authorization across chat and live location flows.

## Goals
- Allow clients to create direct/group conversations with participant usernames.
- Upsert users automatically and record membership with roles and join timestamps.
- Provide membership checks for downstream messaging and live location features.

## Non-goals
- Invitation flows or approval workflows.
- Advanced roles/permissions beyond simple member role.
- Bulk membership sync from external directories.

## User flow
1) Client calls `POST /v1/conversations` with `type`, optional `title`, and participant `usernames`.
2) Server upserts users, creates conversation row, and inserts `conversation_members`.
3) Response returns conversation metadata and member ids; clients then use the id for chat and live location.

## Data sources
- Remote: `POST /v1/conversations`.
- Local: Postgres tables `users`, `conversations`, `conversation_members`.

## Architecture
- Presentation: HTTP handler for conversation creation.
- Domain: Conversation service handling user upserts, direct-vs-group rules, and membership writes.
- Data: Repositories for users, conversations, and membership rows; optional Redis notification for downstream listeners.

## Dependencies
- Fiber HTTP, Postgres driver/tx support, UUID generator.

## State management
- Maintain `role`, `joined_at`, and `last_read_message_id` per member; service enforces membership existence for validation hooks used by other features.

## Error handling
- Validation for missing usernames/type; reject excessively large participant lists.
- Handle duplicate membership inserts gracefully; direct-conversation uniqueness is enforced (return existing conversation on duplicate pair).
- Surface DB errors with request id; partial failure should rollback the transaction.

## Performance considerations
- Index `conversation_members(conversation_id, user_id)` and `conversations(type)` for lookups.
- Batch user upserts and membership inserts within a single transaction.

## Security/Privacy
- Cap participant count and strip unsafe characters from titles.
- Enforce ownership of conversation updates (future extension); currently creation-only.

## Listing conversations
- Endpoint: `GET /v1/conversations?user_id=<uuid>&type=<direct|group|all>`
- AuthZ: caller must be the requested `user_id`; otherwise 403.
- Sorting: `created_at` desc, then `id` desc.
- Response (array):
  ```json
  [
    {
      "id": "uuid",
      "type": "direct",
      "title": "optional for groups",
      "direct_pair_key": "userA:userB",
      "created_at": "2026-01-20T12:00:00Z",
      "member_ids": ["uuid1","uuid2"],
      "roles": {"uuid1":"member","uuid2":"member"}
    }
  ]
  ```
- Pagination: `limit` (default 50, max 200) and `cursor` (opaque, typically `created_at:id` base64).
- Notes:
  - Direct conversations include all members (2 users) and pair key.
  - Group conversations include all member_ids; roles map is optional and may be omitted for performance on large groups.
  - Empty result is 200 with `[]`.

## Analytics/telemetry
- Counters: conversations created by type, failed creations, participant counts.
- Timers: end-to-end creation latency.

## Testing plan
- Unit: request validation, user upsert/membership creation, transaction rollback on error.
- Integration: create direct and group conversations and verify members are persisted and returned.

## Rollout/flags
- Enforce a single direct conversation per user pair by default (unique constraint/service check); feature flag can relax this only for testing.
- Configurable max participant count and title length.

## Decisions
- Direct conversations are unique per user pair; creation returns the existing conversation if already present.
- Creation publishes a lightweight event/notification to online members to sync UI in real time.
