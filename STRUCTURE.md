# Project Structure

The backend is a single Go module at the repository root:

```text
github.com/coderGtm/delta
```

## Application entrypoint

```text
cmd/delta/main.go
```

Pure wiring: loads config, opens the pgx pool, applies embedded migrations, constructs services, registers the `/api/v1` routes, and starts the HTTP server with graceful shutdown. No business logic.

## Feature packages

### `config`

```text
config/config.go
```

Loads the service configuration from environment variables into a single `Config` struct (see `SETUP.md` for the env table).

### `httpapi`

```text
httpapi
 ├── router.go       # ops endpoints (/docs, /healthz, /readyz, /metrics) + middleware chain
 ├── middleware.go   # RequestID, RequestLog, Recoverer, SecurityHeaders, BodyLimit
 ├── ratelimit.go    # in-memory fixed-window rate limiter (single instance)
 ├── errors.go       # APIError, error envelope, code constructors
 ├── response.go     # JSON helpers, ErrorResponse, PageResponse
 ├── pagination.go   # PageParams / ParsePageParams, sort mapping to ORDER BY
 ├── context.go      # request context: subject and request ID
 ├── docs.go         # embedded Swagger UI + openapi.yaml served at /docs
 ├── openapi.yaml    # hand-maintained API contract (also the contract-test oracle)
 └── web/swagger-ui/ # vendored Swagger UI assets (embedded via go:embed)
```

Shared HTTP plumbing used by all domain handlers.

### `db`

```text
db
 ├── connect.go     # pgx connection pool
 ├── migrations.go  # golang-migrate runner over embedded migrations
 ├── store.go       # Store: pool wrapper, Querier access, and Tx
 ├── models.go      # sqlc-generated table models
 ├── db.go          # sqlc-generated query wrapper
 ├── querier.go     # sqlc-generated Querier interface
 ├── *.sql.go       # sqlc-generated per-domain queries
 ├── migrations/    # golang-migrate .sql files (embedded via go:embed)
 ├── queries/       # hand-written .sql files per domain
 └── sqlc.yaml      # sqlc configuration
```

Persistence concerns only; services depend on `db.Store` / `db.Querier`.

### `auth`

```text
auth
 ├── firebase.go     # Firebase ID-token verify/delete (+ stub when unconfigured)
 ├── jwt.go          # HS256 access-token sign/parse
 ├── refreshtoken.go # refresh-token lifecycle: create/validate/rotate/revoke/cleanup
 ├── middleware.go   # AttachUser (loads active user) + Require
 ├── service.go      # login/refresh/logout/logout-all/delete-account
 └── handlers.go     # auth HTTP handlers
```

Authentication and token lifecycle.

### `user`

```text
user
 ├── service.go  # account-deletion boundary
 └── handlers.go # DELETE /users/me
```

Signed-in user's own account endpoints.

### `outlet`

```text
outlet
 ├── service.go     # outlet CRUD + membership domain logic
 ├── handlers.go    # outlet/membership HTTP handlers
 ├── listings.go    # paginated /mine, /invites, /memberships listings
 └── membership.go  # membership invite/accept/reject/remove/display-name operations
```

Outlet management, owner/employee memberships, invitations, membership soft-removal, and geofence configuration.

### `attendance`

```text
attendance
 ├── service.go  # attendance entry domain logic
 ├── geofence.go # haversine geofence enforcement
 ├── handlers.go # attendance HTTP handlers
 └── listings.go # paginated attendance listing
```

Attendance entry CRUD and geofence enforcement.

### `report`

```text
report
 ├── service.go  # salary report calculation (pairing, hours, daily grouping)
 ├── excel.go    # .xlsx export with formula-injection sanitization
 └── handlers.go # salary report HTTP handlers
```

Owner-facing salary reports (JSON + Excel).

## Shared packages

### `audit`

```text
audit/audit.go
```

`audit.Recorder` writes business audit events best-effort in their own transaction, so a failure never rolls back the business write.

### `metrics`

```text
metrics/metrics.go
```

`metrics.Registry` holds lazy Prometheus business counters and exposes them via the `/metrics` handler.

### `decimal`

```text
decimal/decimal.go
```

Exact decimal arithmetic on arbitrary-precision integers, used for latitudes, longitudes, radii, and salary math.

### `contract`

```text
contract
 ├── contract_test.go # contract tests asserting the runtime matches openapi.yaml
 ├── testdb.go        # testcontainers PostgreSQL setup
 └── stub.go          # Firebase stub for tests
```

Test-only package: shared testcontainers infrastructure and the contract-test suite.

## Resources

```text
Dockerfile   # multi-stage: golang:1.25-alpine build, non-root alpine runtime
Makefile     # build / test / lint / vet / fmt / run
go.mod
go.sum
README.md
```

## Root-level files

```text
docker-compose.yml                       # builds the Go image, healthchecks on /readyz
.env.example                             # local secrets and Go service env vars
firebase/service-account.json            # local secret, ignored by Git
loadtest/                                # k6 scripts (mint local JWTs; unchanged)
monitoring/
 ├── prometheus/prometheus.yml           # scrapes /metrics with bearer token
 └── grafana/                            # provisioned Delta Overview dashboard
```

## Note on sqlc

Queries are hand-written in `db/queries/*.sql`. After editing them, regenerate the Go code:

```bash
cd db && sqlc generate
```

The generated files (`models.go`, `db.go`, `querier.go`, `*.sql.go`) are committed to the repository and must not be hand-edited.
