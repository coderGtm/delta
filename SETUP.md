# Setup Guide

## Prerequisites

- Java 25 for local Gradle runs
- Docker and Docker Compose for containerized runs
- Firebase service-account JSON
- PostgreSQL if running outside Docker

## Local test/build commands

From the repository root:

```bash
./gradlew test
./gradlew bootJar
```

## Environment variables

The application reads configuration from environment variables with safe local defaults where appropriate.

Important variables:

| Variable | Purpose |
| --- | --- |
| `DATASOURCE_URL` | JDBC URL for PostgreSQL |
| `DATASOURCE_USERNAME` | PostgreSQL username |
| `DATASOURCE_PASSWORD` | PostgreSQL password |
| `JWT_SECRET` | Secret used to sign local JWT access tokens |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | Path to Firebase service-account JSON |
| `JAVA_OPTS` | Optional JVM options for Docker runtime |
| `FLYWAY_BASELINE_ON_MIGRATE` | Set to `true` once if adopting Flyway on an existing non-empty DB |

## Local Docker Compose setup

Copy the example env file:

```bash
cp .env.example .env
```

Edit `.env` and set local values:

```env
POSTGRES_DB=delta
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
JWT_SECRET=replace-this-with-a-long-random-production-secret-at-least-32-bytes
JAVA_OPTS=-XX:MaxRAMPercentage=75.0
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

## API docs

API docs are generated at runtime from controllers and DTOs using `springdoc-openapi`, so they always reflect the current API.

Interactive Swagger UI is available at:

```text
http://localhost:8080/docs
```

The OpenAPI spec is available at:

```text
http://localhost:8080/docs/openapi.yaml
```

The JSON variant is served at `/docs/openapi`.

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

Schema is managed by Flyway:

```text
src/main/resources/db/migration/V1__init_schema.sql
```

Hibernate is configured with:

```properties
spring.jpa.hibernate.ddl-auto=validate
```

If you are starting from a fresh DB, no special action is needed.

If you are migrating an existing DB that was created with Hibernate `ddl-auto=update`, run once with:

```env
FLYWAY_BASELINE_ON_MIGRATE=true
```

Then turn it back off.

## Health endpoints

Public health/info endpoints:

```bash
curl http://localhost:8080/actuator/health
curl http://localhost:8080/actuator/health/liveness
curl http://localhost:8080/actuator/health/readiness
curl http://localhost:8080/actuator/info
```

`/actuator/prometheus` requires authentication in the app and should normally be kept private to your monitoring network.

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
http://app:8080/actuator/prometheus
```

It authenticates with the bearer token mounted from:

```text
monitoring/prometheus/prometheus-token.txt
```

This token must match `.env` value:

```env
PROMETHEUS_BEARER_TOKEN=...
```

### View raw metrics directly

Use the monitoring token, not a user JWT:

```bash
curl -H "Authorization: Bearer <PROMETHEUS_BEARER_TOKEN>" \
  http://localhost:8080/actuator/prometheus
```

This returns Prometheus text format metrics.

Useful metric families include:

- JVM metrics
- HTTP request metrics
- datasource / Hikari metrics
- custom business counters such as:
  - `auth_login_success_total`
  - `auth_refresh_success_total`
  - `outlet_created_total`
  - `attendance_created_total`
  - `attendance_geofence_rejected_total`
  - `report_salary_generated_total`

Micrometer converts dots in counter names to Prometheus underscores.

### Recommended Prometheus scrape config

In production, Prometheus should scrape the app over a private network, not through a public route.

Example:

```yaml
scrape_configs:
  - job_name: delta-api
    metrics_path: /actuator/prometheus
    static_configs:
      - targets: ["delta-app:8080"]
```

If Prometheus cannot use your JWT auth flow, secure `/actuator/prometheus` at the network or reverse-proxy layer instead:

- private Docker/Kubernetes network
- IP allowlist
- VPN-only access
- mTLS
- reverse-proxy basic auth

Do not expose `/actuator/prometheus` publicly.

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

## Load and rate limit testing

Quick load and rate-limit tests are available under `loadtest/` and run with [k6](https://k6.io) from your machine. They generate real HTTP load through the full stack (Security chain, JWT filter, rate-limiting filter, Hikari, Postgres).

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

Ramps 1 → 60 VUs against `GET /outlets/{id}/attendance` (deliberately chosen because it is not rate-limited, so the numbers reflect app + JVM + connection pool + DB capacity):

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

Verifies the 429 boundaries from `RateLimitingFilter` (`loadtest/rate-limit.js`):

- `POST /api/v1/auth/login` → 10/min per IP
- `POST /api/v1/auth/refresh` → 30/min per IP
- `POST /api/v1/outlets/{id}/attendance` → 20/min per user
- Per-IP isolation via `X-Forwarded-For`

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
