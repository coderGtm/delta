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

Start the stack:

```bash
docker compose up --build
```

Stop the stack:

```bash
docker compose down
```

Stop and remove the Postgres volume:

```bash
docker compose down -v
```

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

### View raw metrics directly

Get an access token, then run:

```bash
curl -H "Authorization: Bearer <access-token>" \
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

## When Grafana is needed

Prometheus stores and queries time-series metrics. Grafana visualizes them.

Use Grafana when you want:

- dashboards for latency, error rate, traffic, JVM, DB pool, and business metrics
- visual trends over time
- alert dashboards
- non-engineers/operators to inspect system health
- comparison across deploys or time windows

You can start without Grafana by querying Prometheus directly, but Grafana becomes valuable once you need dashboards and operational visibility.

Recommended dashboard panels:

- HTTP request rate by route/status
- p95/p99 request latency
- 4xx and 5xx rate
- JVM heap/non-heap usage
- GC pauses
- Hikari active/idle connections
- DB connection timeout count
- attendance created count
- geofence rejection count
- salary report generation count
- auth login/refresh success/failure count

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
