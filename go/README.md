# delta — Go backend

Go implementation of the `delta` employee attendance backend: Firebase login, local JWT auth, outlet memberships, attendance/geofencing, salary reports, golang-migrate migrations, and Prometheus metrics.

The Java sources under `../src/main/java` remain as a read-only contract reference during the migration.

## Quickstart

Prerequisites: Go 1.25+, Docker (or rootless Podman), `make`, and PostgreSQL if running outside Docker.

Copy the example env file and edit it (a `JWT_SECRET` is required):

```bash
cp ../.env.example .env
```

All configuration comes from environment variables; see `../SETUP.md` for the full table and `config/config.go` for the source of truth.

Run the service locally (applies migrations when `AUTO_MIGRATE=true`):

```bash
make run
```

Build, test, and lint:

```bash
make build
make test
make lint
```

Integration and contract tests use [testcontainers-go](https://golang.testcontainers.org) and need a container runtime. Under rootless Podman, set `TESTCONTAINERS_RYUK_DISABLED=true` and `DOCKER_HOST=unix:///run/user/<uid>/podman/podman.sock`.

Run the full stack with Docker Compose from the repository root:

```bash
cp ../.env.example ../.env
# edit ../.env and provide ../firebase/service-account.json

docker compose up --build
```

## Package map

| Package | Purpose |
| --- | --- |
| `cmd/delta` | Entrypoint; wiring only, no business logic |
| `config` | Environment-based configuration |
| `httpapi` | Router, middleware, error/response envelopes, pagination, rate limiting, /docs |
| `db` | pgx pool, sqlc-generated queries, embedded golang-migrate migrations |
| `auth` | Firebase verify/delete, JWT, refresh tokens, middleware, handlers |
| `user` | Account deletion |
| `outlet` | Outlets + memberships domain and handlers |
| `attendance` | Attendance + geofence and handlers |
| `report` | Salary reports + Excel export and handlers |
| `audit` | Best-effort audit event recording |
| `metrics` | Prometheus business counter registry |
| `decimal` | Exact decimal arithmetic |
| `contract` | Testcontainers infrastructure + contract tests |

See `../STRUCTURE.md` for the full tree.

## Ops endpoints

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Liveness; always `200 {"status":"UP"}` when serving |
| `GET /readyz` | Readiness; pings the database |
| `GET /metrics` | Prometheus metrics; bearer-gated when `PROMETHEUS_BEARER_TOKEN` is set |
| `GET /docs` | Swagger UI over the hand-maintained spec |
| `GET /docs/openapi.yaml` | The OpenAPI spec |

The client-facing API lives under `/api/v1` (e.g. `/api/v1/auth/login`, `/api/v1/outlets`).

## Links

- `../SETUP.md` — setup, configuration, monitoring, report usage.
- `../STRUCTURE.md` — package layout and sqlc workflow.
- `../AGENTS.md` — coding-agent/project guidance.
- `../docs/superpowers/specs/2026-08-14-delta-go-rewrite-design.md` — the rewrite design spec.
