# Repository Guidelines

## Project Structure & Module Organization
- `docs/architecture/`: Architecture, datastore, Docker, and environment configuration guides.
- `docs/features/`: Feature specs for handshake, messaging, conversations, and live location.
- `docs/`: Root technical architecture overview and templates.
- Source code and migrations live alongside these docs (e.g., `cmd/`, `internal/`, `migrations/`) when added; keep new packages organized by domain (api, service, repo, worker).

## Build, Test, and Development Commands
- `go build ./...`: Compile the services; run from repo root.
- `go test ./...`: Run unit/integration tests.
- `docker compose up`: Bring up Postgres + Redis (+ API if configured) using the documented Docker setup.
- `docker compose down -v`: Tear down stack and drop volumes for a clean slate.

## Coding Style & Naming Conventions
- Go modules: prefer `internal/<domain>` and `pkg/` only for shared exports; use lower_snake for packages and UpperCamel for types.
- Keep handlers thin; push logic into services/use-cases; repos isolate DB/Redis access.
- Follow Go fmt/goimports; add comments sparingly for non-obvious logic.
- Config: load via YAML (`yaml.v3`) + explicit env overrides; no global singletons.

## Testing Guidelines
- Name tests with `_test.go`; table-driven where helpful.
- Use `go test ./...` locally; add integration tests for WebSocket flows and DB/Redis interactions where feasible.
- Prefer fast unit tests; gate slower integration tests behind build tags if they need external services.

## Commit & Pull Request Guidelines
- Commit messages must start with `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, or `test:` (enforced by `.githooks/commit-msg`).
- Keep commits focused and small; include context in the body when needed.
- PRs: describe the change, list test commands run, link issues/tasks, and add screenshots/log snippets for user-visible or operational changes.

## Security & Configuration Tips
- Do not commit secrets or real connection strings. Use env vars to override DB/Redis URLs at runtime.
- Generate environment YAMLs locally under `config/` as per `docs/architecture/environment-config.md`; they are intentionally untracked.
- Ensure TLS/`sslmode=require` for production DB and `rediss://` for production Redis per docs.

## Architecture Alignment
- Follow `docs/architecture/*.md` for persistence, Redis keys, Docker, and config defaults.
- Follow `docs/features/*.md` for contract details (dedupe keys, ordering rules, TTL clamps, subscription behavior).***
