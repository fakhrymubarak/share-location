# Docker Setup

- Status: Draft
- Owner: Backend Team
- Last updated: 20/01/2026

## Overview
Containerized stack for local/dev/test: Go Fiber API, PostgreSQL, Redis, and migrations. Uses Docker Compose for reproducible setup with minimal host dependencies.

## Goals
- One-command startup for the full backend stack.
- Consistent environment variables across services.
- Fast teardown/reset for local iteration.

## Non-goals
- Production orchestration (Kubernetes/Terraform) or secret management.
- Autoscaling or multi-node clustering.

## Services (compose)
- `api`: Go Fiber app built from this repo; depends on `db`, `redis`. Ports: `8080:8080` (HTTP/WebSocket).
- `db`: PostgreSQL 15. Ports: `5432:5432`. Volume: `pgdata:/var/lib/postgresql/data`.
- `redis`: Redis 7. Ports: `6379:6379`.
- `migrate` (optional): one-shot migration job using the same image or a lightweight migrator; depends_on `db`.

## Example docker-compose.yml (snippet)
```yaml
services:
  db:
    image: postgres:15
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-chatapp}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-change-me}
      POSTGRES_DB: ${POSTGRES_DB:-chatapp}
    volumes:
      - pgdata:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  api:
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      DB_URL: ${DB_URL:-postgres://${POSTGRES_USER:-chatapp}:${POSTGRES_PASSWORD:-change-me}@db:5432/${POSTGRES_DB:-chatapp}?sslmode=disable}
      REDIS_URL: redis://redis:6379/0
      WS_HEARTBEAT_INTERVAL_SECONDS: 30
      WS_IDLE_TIMEOUT_SECONDS: 60
      LIVE_LOCATION_MAX_TTL_SECONDS: 900
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
    ports:
      - "8080:8080"

  pgadmin:
    image: dpage/pgadmin4:8
    environment:
      PGADMIN_DEFAULT_EMAIL: ${PGADMIN_EMAIL:-admin@example.com}
      PGADMIN_DEFAULT_PASSWORD: ${PGADMIN_PASSWORD:-change-me}
    depends_on:
      db:
        condition: service_healthy
    ports:
      - "5050:80"
    volumes:
      - pgadmin:/var/lib/pgadmin

volumes:
  pgdata:
  pgadmin:
```

## Configuration
- Environment sourced via `.env.docker` (recommended) and referenced in compose. Key vars: `DB_URL`, `REDIS_URL`, `LIVE_LOCATION_MAX_TTL_SECONDS`, `WS_HEARTBEAT_INTERVAL_SECONDS`, `WS_IDLE_TIMEOUT_SECONDS`.
- For tests/CI, override with shorter TTLs and in-memory fixtures if needed.

## Data and cleanup
- Postgres data persisted in `pgdata` volume. For a clean slate: `docker compose down -v`.
- Redis keeps ephemeral data only; no volume required.

## Startup flow
1) `docker compose up -d db redis`
2) Run migrations (`docker compose run --rm migrate` or app auto-migrate if enabled).
3) `docker compose up -d api`

## Logging and observability
- Default to stdout/stderr; use `docker compose logs -f api|db|redis`.
- Healthchecks gate API startup; alerts surfaced via logs/metrics in-app.

## Security basics
- Use strong, non-default credentials in `.env.docker` for non-local use.
- TLS termination expected at a reverse proxy/load balancer; not included in compose.

## Notes
- Compose file can live at repo root; keep service names stable (`db`, `redis`, `api`) to match connection strings.
- For hot reload in dev, add a bind mount of the source and run the app in watch mode.***
