# Setup Guide

## Prerequisites

- Go 1.25+ for local builds and runs
- Docker and Docker Compose (or rootless Podman with a Docker-compatible socket) for containerized runs
- `make` for the build/test/lint shortcuts in the root `Makefile`
- Firebase service-account JSON
- PostgreSQL if running outside Docker (the Go app only needs a `DATABASE_URL`)

The testcontainers-based integration and contract suites in `contract`, `auth`, `outlet`, `attendance`, and `report` spin up a real PostgreSQL container, so they also need a working container runtime (Docker or rootless Podman).

## Local test/build commands

From the repository root:

```bash
make test
make build
make lint
```

- `make test` runs `go test ./...`.
- `make build` builds the `delta` binary from `./cmd/delta`.
- `make lint` runs `go vet ./...` plus `gofmt -l .`.

## Environment variables

The application reads all configuration from environment variables (see `config/config.go`). Variables not listed below use their defaults.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/delta` | PostgreSQL connection URL |
| `AUTO_MIGRATE` | `true` | Apply golang-migrate migrations at startup |
| `JWT_SECRET` | (none; required) | Secret used to sign local JWT access and refresh tokens |
| `JWT_ACCESS_TOKEN_TTL` | `900000` (15 min) | Access token lifetime, milliseconds |
| `JWT_REFRESH_TOKEN_TTL` | `2592000000` (30 days) | Refresh token lifetime, milliseconds |
| `JWT_REFRESH_CLEANUP_INTERVAL` | `86400000` (24 h) | How often expired refresh tokens are cleaned up, milliseconds |
| `JWT_REFRESH_REVOKED_RETENTION` | `604800000` (7 days) | How long revoked tokens are kept before permanent deletion, milliseconds |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | `firebase/service-account.json` | Path to the Firebase service-account JSON |
| `PROMETHEUS_BEARER_TOKEN` | (empty) | Bearer token required to access `GET /metrics`; empty disables the gate |
| `TRUST_PROXY_HEADERS` | `true` | Honor `X-Forwarded-For`/`X-Real-IP` for client-IP and rate-limit keys; set `false` for direct exposure to prevent IP rate-limit spoofing |
| `LOG_LEVEL` | `info` | Structured log level (`info`, `debug`, ...) |
| `LOG_FORMAT` | `text` | Log output format: `text` or `json` |

## Local Docker Compose setup

Copy the example env file:

```bash
cp .env.example .env
```

Edit `.env` and set local values:

```env
JWT_SECRET=replace-this-with-a-long-random-production-secret-at-least-32-bytes
PROMETHEUS_BEARER_TOKEN=replace-this-with-a-long-random-monitoring-token
```

Place the Firebase service account at:

```text
firebase/service-account.json
```

This file is ignored by Git.

Before starting the stack, keep the Prometheus scrape token in sync:

```bash
cp monitoring/prometheus/prometheus-token.example.txt monitoring/prometheus/prometheus-token.txt
# edit .env PROMETHEUS_BEARER_TOKEN and monitoring/prometheus/prometheus-token.txt to the same value
```

Start the stack:

```bash
docker compose up --build
```

Services:

| Service | URL |
| --- | --- |
| App | http://localhost:8080 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 |

Default local Grafana credentials come from `.env`:

```text
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin
```

Stop the stack:

```bash
docker compose down
```

Stop and remove the Postgres volume:

```bash
docker compose down -v
```

## Ops endpoints

The Go app exposes Go-native operational endpoints in addition to the `/api/v1` API:

| Endpoint | Purpose |
| --- | --- |
| `GET /healthz` | Liveness: always `200 {"status":"UP"}` when the process is serving |
| `GET /readyz` | Readiness: pings the database; `200 {"status":"UP"}` or `503 {"status":"DOWN","error":...}` |
| `GET /metrics` | Prometheus text-format metrics; gated by `Authorization: Bearer <PROMETHEUS_BEARER_TOKEN>` when that variable is set |
| `GET /docs` | Interactive Swagger UI (embedded) |
| `GET /docs/openapi.yaml` | The hand-maintained OpenAPI spec |

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

`/metrics` should normally be kept private to your monitoring network:

```bash
curl -H "Authorization: Bearer <PROMETHEUS_BEARER_TOKEN>" http://localhost:8080/metrics
```

## API docs

The OpenAPI spec is hand-maintained at `httpapi/openapi.yaml` and embedded into the binary. Contract tests in `contract` assert that the runtime matches it.

Interactive Swagger UI is available at:

```text
http://localhost:8080/docs
```

The OpenAPI spec is available at:

```text
http://localhost:8080/docs/openapi.yaml
```

Use this while building the Android app to inspect request/response models, auth requirements, and report query parameters.

## API base URL

The current API is versioned under:

```text
/api/v1
```

Examples:

```text
/api/v1/auth/login
/api/v1/outlets
/api/v1/outlets/{outletId}/attendance
/api/v1/outlets/{outletId}/reports/salary
/api/v1/outlets/{outletId}/leave
/api/v1/users/me
```

## Database migrations

Schema is managed by golang-migrate, embedded into the binary:

```text
db/migrations
```

```text
db/migrations/000001_init.up.sql
db/migrations/000001_init.down.sql
```

By default `AUTO_MIGRATE=true` applies all pending migrations at startup. If you are starting from a fresh DB, no special action is needed.

Query code is generated by sqlc from hand-written queries in `db/queries/*.sql`; see `STRUCTURE.md`.

## Metrics and monitoring

### Local Grafana dashboard

When running Docker Compose, open:

```text
http://localhost:3000
```

Log in with the credentials from `.env`. A `Delta Overview` dashboard is provisioned automatically and uses Prometheus as the default datasource.

### Prometheus

Prometheus is available at:

```text
http://localhost:9090
```

Prometheus scrapes the app over the private Compose network at:

```text
http://app:8080/metrics
```

It authenticates with the bearer token mounted from:

```text
monitoring/prometheus/prometheus-token.txt
```

This token must match the `.env` value:

```env
PROMETHEUS_BEARER_TOKEN=...
```

### View raw metrics directly

Use the monitoring token, not a user JWT:

```bash
curl -H "Authorization: Bearer <PROMETHEUS_BEARER_TOKEN>" \
  http://localhost:8080/metrics
```

This returns Prometheus text format metrics.

Useful metric families include:

- Go runtime and process metrics
- HTTP request metrics
- custom business counters such as:
  - `auth_login_success_total`
  - `auth_refresh_success_total`
  - `outlet_created_total`
  - `attendance_created_total`
  - `attendance_geofence_rejected_total`
  - `report_salary_generated_total`

### Recommended Prometheus scrape config

In production, Prometheus should scrape the app over a private network, not through a public route.

Example:

```yaml
scrape_configs:
  - job_name: delta-api
    metrics_path: /metrics
    static_configs:
      - targets: ["delta-app:8080"]
```

If Prometheus cannot use your bearer token, secure `/metrics` at the network or reverse-proxy layer instead:

- private Docker/Kubernetes network
- IP allowlist
- VPN-only access
- mTLS
- reverse-proxy basic auth

Do not expose `/metrics` publicly.

## Secrets guidance

### Local development

Use `.env` and `firebase/service-account.json`.

Both are ignored by Git.

### Production

Do not put secrets in Docker images or source control.

Use your platform secret manager:

- Docker Swarm secrets
- Kubernetes Secrets or External Secrets
- AWS Secrets Manager
- GCP Secret Manager
- Azure Key Vault
- HashiCorp Vault

For Firebase, mount the JSON as a file and point:

```env
FIREBASE_SERVICE_ACCOUNT_PATH=/run/secrets/firebase-service-account.json
```

For JWT, generate a long random value and inject it as:

```env
JWT_SECRET=<secret>
```

## Testing

Run the unit, integration, and contract suites:

```bash
make test
```

Integration and contract tests use [testcontainers-go](https://golang.testcontainers.org) and require a running container runtime.

Under rootless Podman, container creation and Ryuk (the testcontainers reaper) behave differently. Disable Ryuk and point the Docker client at the Podman socket:

```bash
export TESTCONTAINERS_RYUK_DISABLED=true
export DOCKER_HOST=unix:///run/user/<uid>/podman/podman.sock
```

## Load and rate limit testing

Quick load and rate-limit tests are available under `loadtest/` and run with [k6](https://k6.io) from your machine. They generate real HTTP load through the full stack (JWT auth, rate-limiting middleware, connection pool, Postgres).

The scripts mint local HS256 access tokens with the same `JWT_SECRET` the app uses, so authenticated endpoints are tested without hitting Firebase or the login rate limit.

### Prerequisites

- The docker compose stack is running (`docker compose up --build`).
- `k6` is installed locally (`brew install k6` or from the k6 website).
- The app's `JWT_SECRET` matches `loadtest/config.js` (compose default does).

### Seed test data

```bash
./loadtest/seed.sh
```

This inserts 5 users, 1 outlet with accepted memberships, and 160 attendance entries into the running `delta-postgres` container. Idempotent, safe to re-run.

### Smoke test

Sanity check that auth and the seeded data work:

```bash
k6 run loadtest/smoke.js
```

### Capacity test

Ramps 1 → 60 VUs against `GET /outlets/{id}/attendance` (deliberately chosen because it is not rate-limited, so the numbers reflect app + connection pool + DB capacity):

```bash
k6 run loadtest/capacity.js
```

Raise the ceiling with:

```bash
k6 run -e MAX_VUS=100 loadtest/capacity.js
```

The summary reports requests/sec, latency percentiles, and error rate. Watch memory during the run with:

```bash
docker stats delta-app
```

For a real-world baseline run this against your production-like hardware and Postgres, not the local laptop.

### Rate limit tests

Verifies the 429 boundaries from the in-memory rate limiter (`loadtest/rate-limit.js`):

- `POST /api/v1/auth/login` → 10/min per IP
- `POST /api/v1/auth/refresh` → 30/min per IP
- `POST /api/v1/outlets/{id}/attendance` → 20/min per user
- Per-IP isolation via `X-Forwarded-For`

Per-IP isolation relies on `TRUST_PROXY_HEADERS=true`, which is the compose default; if you set it to `false`, IP keys fall back to the socket remote address.

```bash
k6 run loadtest/rate-limit.js
```

Note: the app's rate-limit counters are in-memory and reset after 1 minute, so wait at least a minute between runs against the same app instance.

### Configuration

All scripts read the same env vars (defaults in `loadtest/config.js`):

| Variable | Default | Purpose |
| --- | --- | --- |
| `BASE_URL` | `http://localhost:8080` | App base URL |
| `JWT_SECRET` | compose default | Secret used to mint local access tokens |
| `OUTLET_ID` | seeded outlet | Outlet used in tests |
| `OWNER_ID` | seeded owner | Owner user id for read endpoints |
| `EMPLOYEE_IDS` | seeded employees | Employee user ids for write endpoints |
| `MAX_VUS` | `60` | Capacity test peak virtual users |

### Interpreting results

- The unthrottled read endpoint sustains thousands of requests/sec on a local machine, so the app is typically not the bottleneck for small deployments — Postgres connection pool size and the heaviest endpoints (salary `.xlsx` export, attendance writes) matter more.
- Writes are intentionally capped by rate limits (e.g. 20 attendance entries/min/user), so those endpoints are bottlenecked by policy, not capacity.

## Security hardening

The API is a stateless JSON service. Defaults and how to tighten them for production:

- **SQL injection** — not applicable: all persistence goes through sqlc-generated, parameterized queries; there is no raw SQL in application code.
- **Excel formula injection** — user-supplied outlet names and member display names written to salary report cells are sanitized on export, so values beginning with `=`, `+`, `-`, `@`, or control characters are rendered as text, not evaluated.
- **Response headers** — `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, `Permissions-Policy`, a same-origin `Content-Security-Policy`, and HTTP `Strict-Transport-Security` (HSTS) are applied on every response.
- **Request body limits** — request bodies over 2 MiB are rejected via `http.MaxBytesReader` (`413` for declared `Content-Length` over the limit).
- **Proxy headers** — the app resolves client IPs from `X-Forwarded-For`/`X-Real-IP` only when `TRUST_PROXY_HEADERS=true`. Behind a public proxy, ensure it overwrites these headers from the trusted connection. When exposed directly, set `TRUST_PROXY_HEADERS=false` so the socket remote address is used and IP-based rate-limit keys cannot be spoofed.
- **Rate limiting** — apply particularly to auth and write endpoints; the counters are in-memory (single instance). Move to Redis/gateway when scaling horizontally.
- **`/metrics`** — protected by a bearer token (when `PROMETHEUS_BEARER_TOKEN` is set) and intended for private monitoring networks only.

## Report generation

Salary report endpoints:

```text
GET /api/v1/outlets/{outletId}/reports/salary
GET /api/v1/outlets/{outletId}/reports/salary.xlsx
```

Required query parameters:

| Parameter | Example | Notes |
| --- | --- | --- |
| `userId` | UUID | Employee user ID |
| `startTime` | `2024-01-01T00:00:00Z` | Inclusive instant |
| `endTime` | `2024-01-31T23:59:59Z` | Exclusive instant in calculation |
| `timezone` | `Asia/Kolkata` | IANA timezone used for daily grouping/display |
| `hourlyRate` | `100.00` | Rate multiplied by calculated hours |

Example:

```bash
curl -H "Authorization: Bearer <access-token>" \
  "http://localhost:8080/api/v1/outlets/<outletId>/reports/salary?userId=<employeeId>&startTime=2024-01-01T00:00:00Z&endTime=2024-02-01T00:00:00Z&timezone=Asia/Kolkata&hourlyRate=100.00"
```

Excel export:

```bash
curl -L -H "Authorization: Bearer <access-token>" \
  -o salary-report.xlsx \
  "http://localhost:8080/api/v1/outlets/<outletId>/reports/salary.xlsx?userId=<employeeId>&startTime=2024-01-01T00:00:00Z&endTime=2024-02-01T00:00:00Z&timezone=Asia/Kolkata&hourlyRate=100.00"
```
