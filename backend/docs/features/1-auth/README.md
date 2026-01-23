# Authentication

- Status: Draft
- Owner: Backend Team
- Last updated: 23/01/2026

## Overview
Secure username and password authentication to replace the previous username-only handshake. Supports user registration and login, ensuring that user identity is protected by hashed passwords (Argon2/Bcrypt) and that sessions are properly authenticated.

## Goals
- Allow users to register with a username and password.
- Allow users to login with credentials and obtain a session/token (or just ID for this simple MVP if stateless).
- Securely store passwords using industry-standard hashing.
- Prevent impersonation which was possible with username-only auth.

## Non-goals
- OAuth/Social Login (Google, Facebook).
- Password reset flow (email/SMS).
- Two-Factor Authentication (2FA).
- Complex session management (refresh tokens, sliding expiration) for v1—we will keep it simple.

## User flow
1) **Register**: Client sends `POST /v1/register` with `{username, password}`.
   - Server hashes password, creates user, returns `user_id`.
2) **Login**: Client sends `POST /v1/login` with `{username, password}`.
   - Server verifies hash.
   - Response includes `user_id` (and potentially a basic token if we want to enforce auth on WS).
3) **Connect**: Client connects to `ws://.../ws?username=...` (legacy) OR with a token/header.
   - *Design Choice*: For v1, we can keep using `user_id` or `username` for the WS handshake if we trust the client after login, OR we can pass a temporary `session_token` returned by login.
   - *Decision*: To keep it simple but "more" secure: Login returns a `session_token` (UUID). The WS connection must pass `?token=<session_token>`.

## Data sources
- Remote: `POST /v1/register`, `POST /v1/login`.
- Local: Postgres `users` table (updated with `password_hash`).

## Architecture
- **Presentation**: HTTP handlers for auth routes.
- **Domain**: Auth service for hashing, validation, and user creation.
- **Data**: User repository with simple CRUD + password field.

## Dependencies
- Go `crypto` (e.g., `golang.org/x/crypto/bcrypt`) for hashing.
- UUID generator.

## Data Model Updates
- `users` table:
  - Add `password_hash` (text/varchar).
  - Add `last_login_at` (timestamptz, optional).

## Security/Privacy
- **Hashing**: Use Bcrypt (cost 10+) or Argon2id. Never store plain text.
- **Transport**: Always HTTPS (TLS) in production to protect credentials.
- **Rate Limiting**: Strict limits on `/v1/login` to prevent brute force (e.g., 5 attempts per minute per IP).

## Error handling
- Invalid credentials return `401 Unauthorized` generic message (don't reveal "user not found" vs "bad password" if possible, or accepted risk for v1).
- Duplicate username on register returns `409 Conflict`.

## Testing plan
- Unit: Hash generation/comparison, regex validation for passwords.
- Integration: Register -> Login success; Login with bad password fails.

## Rollout/flags
- Implementation will replace the existing `POST /v1/login` (which was handshake-only).
- Existing users (if any) with no password might need a migration strategy (e.g., set temp password or allow empty password logic until claimed). For this project, we assume a fresh start or "claiming" legacy accounts.

## Decisions
- We will use **Bcrypt** for simplicity and wide support in Go.
- We will introduce a simple `sessions` table or Redis key `session:{token} -> user_id` for the WebSocket handshake to verify identity.
