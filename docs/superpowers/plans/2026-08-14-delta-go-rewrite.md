# Delta Go Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port the Java/Spring Boot `delta` attendance backend to an idiomatic Go service in `go/`, with byte-for-byte parity of the `/api/v1/*` client contract and business rules, Go-native ops endpoints, and full test coverage.

**Architecture:** Flat Go domain packages (`config`, `db`, `httpapi`, `auth`, `user`, `outlet`, `attendance`, `report`, `audit`, `metrics`) over stdlib `net/http` routing, `pgx` + `sqlc` for typed SQL, embedded `golang-migrate` migrations run on boot, and a thin `cmd/delta/main.go` as the only wiring point. Services depend on `*db.Store`; no package import cycles (see Global Constraints). The Java sources at the repo root are the authoritative reference for message strings and edge cases — never modify them.

**Tech Stack:** Go 1.25, stdlib `net/http` (ServeMux, method+path patterns), `github.com/jackc/pgx/v5`, `sqlc`, `github.com/golang-migrate/migrate/v4` (iofs source), `github.com/golang-jwt/jwt/v5`, `firebase.google.com/go/v4`, `github.com/xuri/excelize/v2`, `github.com/prometheus/client_golang`, stdlib `log/slog`, `github.com/google/uuid`, `github.com/testcontainers/testcontainers-go`, `github.com/google/go-cmp/cmp`.

**Spec:** `docs/superpowers/specs/2026-08-14-delta-go-rewrite-design.md` (read it; this plan argues from it).

## Global Constraints

- Go 1.25. Module path `github.com/coderGtm/delta/go`.
- Only these third-party deps may be added: pgx/v5, golang-migrate/v4, golang-jwt/jwt/v5, firebase admin, excelize/v2, prometheus/client_golang, google/uuid, testcontainers-go, go-cmp. (Dev tools: sqlc CLI, golangci-lint optional.)
- Package graph must stay acyclic. Allowed edges: everything → `db`, `httpapi`, `metrics`, `audit`; `auth` → `user` is forbidden (the `user` package must not import `auth`; `user` defines interfaces that `auth` implements, wired in `main`).
- The `User` row model lives in `db` (sqlc-generated), not in `user`.
- `/api/v1/*` responses must match the Java camelCase JSON field names and shapes exactly. Timestamps encode as RFC 3339 UTC (`…Z`).
- Do NOT modify anything under `src/` (Java reference). Do NOT delete Java files.
- Every behavior change lands with its tests; `go test ./...`, `go vet ./...`, and `gofmt` must pass before moving on.
- Ops endpoints are Go-native: `/healthz`, `/readyz`, `/metrics` (bearer-gated), `/docs`.
- Migrations run automatically at startup when `AUTO_MIGRATE=true` (default).
- Logs must never contain tokens, `Authorization` headers, or PII beyond what the spec permits.
- Every task ends with a git commit on branch `go-rewrite`.

---

## File Structure

```
go/
├── go.mod, go.sum
├── Makefile
├── Dockerfile
├── openapi.yaml
├── README.md
├── cmd/delta/main.go                     # wiring: config, pool, migrate, router, server, shutdown
├── config/
│   ├── config.go                         # Config struct + Load() from env
│   └── config_test.go
├── db/
│   ├── connect.go                        # Open pool
│   ├── migrations.go                     # embed + Migrate()
│   ├── migrations/000001_init.up.sql     # full schema (spec §Data model)
│   ├── migrations/000001_init.down.sql
│   ├── store.go                          # Store{pool}; Querier(); Tx()
│   ├── queries/*.sql                     # per-domain sqlc query files
│   └── (generated) models.go, db.go, querier.go, *.sql.go
├── httpapi/
│   ├── errors.go                         # APIError + ErrorResponse + WriteError
│   ├── response.go                       # WriteJSON, WritePage[T]
│   ├── pagination.go                     # PageParams, SortOrder, ParsePageParams
│   ├── context.go                        # Subject interface + WithSubject/SubjectFrom
│   ├── middleware.go                     # requestID+log, recovery, headers, body limit, clientIP
│   └── router.go                         # NewRouter(srv) http.Handler
├── metrics/
│   └── metrics.go                        # Registry{...}; Increment(name, tags...)
├── audit/
│   └── audit.go                          # Recorder{Store}; Record(...) in own tx (best-effort)
├── auth/
│   ├── firebase.go                       # Firebase interface + firebaseClient + stub types
│   ├── jwt.go                            # JWTService
│   ├── refreshtoken.go                   # RefreshTokenService + cleanup ticker
│   ├── service.go                        # Service (login/refresh/logout/logout-all/deleteAccount/createUser)
│   ├── middleware.go                     # AttachUser, Require
│   └── handlers.go                       # login/refresh/logout/logout-all handlers
├── user/
│   ├── service.go                        # AccountDeleter interface (impl: auth.Service)
│   └── handlers.go                       # DELETE /users/me
├── outlet/
│   ├── service.go                        # outlet + membership business logic
│   └── handlers.go
├── attendance/
│   ├── geofence.go                       # haversine
│   ├── service.go
│   └── handlers.go
├── report/
│   ├── service.go                        # salary calculation
│   ├── excel.go                          # xlsx build + sanitization
│   └── handlers.go
├── contract/                             # end-to-end contract tests (testcontainers)
│   └── contract_test.go
└── README.md
```

Root (modified): `docker-compose.yml`, `.env.example`, `monitoring/prometheus/prometheus.yml`, `README.md`, `SETUP.md`, `STRUCTURE.md`, `AGENTS.md`.

---

## Phase 0: Scaffold

### Task 1: Go module + `config` package

**Files:**
- Create: `go/go.mod`
- Create: `go/config/config.go`
- Create: `go/config/config_test.go`

**Interfaces:**
- Produces: `func Load() (Config, error)`; `Config` struct with fields `Port int`, `DatabaseURL string`, `AutoMigrate bool`, `JWTSecret string`, `AccessTokenTTL time.Duration`, `RefreshTokenTTL time.Duration`, `RefreshCleanupInterval time.Duration`, `RefreshRevokedRetention time.Duration`, `FirebaseServiceAccountPath string`, `PrometheusBearerToken string`, `TrustProxyHeaders bool`, `LogLevel string`, `LogFormat string`.

- [ ] **Step 1: Create module and write the failing test**

```bash
cd go && go mod init github.com/coderGtm/delta/go && cd ..
```

```go
package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/delta" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("RefreshTokenTTL = %v", cfg.RefreshTokenTTL)
	}
	if !cfg.AutoMigrate || !cfg.TrustProxyHeaders {
		t.Errorf("AutoMigrate=%v TrustProxyHeaders=%v", cfg.AutoMigrate, cfg.TrustProxyHeaders)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://u:p@h/db")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("JWT_ACCESS_TOKEN_TTL", "60000")
	t.Setenv("TRUST_PROXY_HEADERS", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 || cfg.DatabaseURL != "postgres://u:p@h/db" || cfg.AutoMigrate ||
		cfg.AccessTokenTTL != time.Minute || cfg.TrustProxyHeaders {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error when JWT_SECRET empty")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./config/`
Expected: FAIL (Load not defined).

- [ ] **Step 3: Write minimal implementation**

```go
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                       int
	DatabaseURL                string
	AutoMigrate                bool
	JWTSecret                  string
	AccessTokenTTL             time.Duration
	RefreshTokenTTL            time.Duration
	RefreshCleanupInterval     time.Duration
	RefreshRevokedRetention    time.Duration
	FirebaseServiceAccountPath string
	PrometheusBearerToken      string
	TrustProxyHeaders          bool
	LogLevel                   string
	LogFormat                  string
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDurationMillis(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return def
}

func Load() (Config, error) {
	cfg := Config{
		Port:                       getEnvInt("PORT", 8080),
		DatabaseURL:                getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/delta"),
		AutoMigrate:                getEnvBool("AUTO_MIGRATE", true),
		JWTSecret:                  os.Getenv("JWT_SECRET"),
		AccessTokenTTL:             getEnvDurationMillis("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:            getEnvDurationMillis("JWT_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		RefreshCleanupInterval:     getEnvDurationMillis("JWT_REFRESH_CLEANUP_INTERVAL", 24*time.Hour),
		RefreshRevokedRetention:    getEnvDurationMillis("JWT_REFRESH_REVOKED_RETENTION", 7*24*time.Hour),
		FirebaseServiceAccountPath: getEnv("FIREBASE_SERVICE_ACCOUNT_PATH", "firebase/service-account.json"),
		PrometheusBearerToken:      os.Getenv("PROMETHEUS_BEARER_TOKEN"),
		TrustProxyHeaders:          getEnvBool("TRUST_PROXY_HEADERS", true),
		LogLevel:                   getEnv("LOG_LEVEL", "info"),
		LogFormat:                  getEnv("LOG_FORMAT", "text"),
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set")
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./config/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/go.mod go/config
git commit -m "feat(go): scaffold module and config package"
```

---

### Task 2: DB pool + migrations + sqlc setup

**Files:**
- Create: `go/db/migrations/000001_init.up.sql`
- Create: `go/db/migrations/000001_init.down.sql`
- Create: `go/db/connect.go`
- Create: `go/db/migrations.go`
- Create: `go/db/store.go`
- Create: `go/db/sqlc.yaml`
- Create: `go/db/queries/users.sql`
- Create: `go/db/queries/refresh_tokens.sql`
- Create: `go/db/queries/outlets.sql`
- Create: `go/db/queries/attendance.sql`
- Create: `go/db/queries/audit.sql`
- Create: `go/db/queries/reports.sql`

**Interfaces:**
- Produces:
  - `func Open(ctx context.Context, url string) (*pgxpool.Pool, error)`
  - `func Migrate(ctx context.Context, url string) error`
  - `type Store struct { pool *pgxpool.Pool }`
  - `func NewStore(pool *pgxpool.Pool) *Store`
  - `func (s *Store) Pool() *pgxpool.Pool`
  - `func (s *Store) Querier() db.Querier` (generated; method name from generated `New`)
  - `func (s *Store) Tx(ctx context.Context, fn func(q db.Querier) error) error`
- Consumes: `config.Config` (DatabaseURL, AutoMigrate).

- [ ] **Step 1: Write the full init migration (up)**

`go/db/migrations/000001_init.up.sql` — copy verbatim the DDL from the spec §Data model (all tables, CHECKs, FKs, indexes, `gen_random_uuid()` defaults). Verify against spec before committing. Down migration: drop all tables (audit_events, attendance_entries, refresh_tokens, outlet_memberships, outlets, users) in reverse FK order.

- [ ] **Step 2: Write connect, migrate, store**

```go
// connect.go
package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, url)
}
```

```go
// migrations.go
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Migrate(ctx context.Context, url string) error {
	pool, err := Open(ctx, url)
	if err != nil {
		return fmt.Errorf("opening pool for migration: %w", err)
	}
	defer pool.Close()

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("loading embedded migrations: %w", err)
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Release()

	driver, err := postgres.WithInstance(conn.Conn(), &postgres.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, url, driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("applying migrations: %w", err)
	}
	return nil
}
```

```go
// store.go
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) Pool() *pgxpool.Pool { return s.pool }

func (s *Store) Querier() *Queries { return New(s.pool) }

func (s *Store) Tx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(New(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
```

Note: the two `Querier`/`Queries` signatures above are provisional; whichever the sqlc output uses (package `db`, generated `New(dbtx DBTX) *Queries`), keep that and update `store.go` accordingly. The rest of the codebase uses `*db.Store`.

- [ ] **Step 3: Write the sqlc query files**

`go/db/queries/users.sql` (sqlc annotations set Go method names):

```sql
-- name: GetUserByAuthUID :one
SELECT * FROM users WHERE auth_uid = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetUserByEmailCaseInsensitive :one
SELECT * FROM users WHERE lower(email) = lower($1) AND deleted_at IS NULL LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (auth_uid, name, email, phone)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: DeleteUserRow :one
UPDATE users
SET historical_email = email, email = NULL, deleted_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;
```

`go/db/queries/refresh_tokens.sql`:

```sql
-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token_hash, expires_at, revoked, user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens WHERE token_hash = $1 LIMIT 1;

-- name: UpdateRefreshTokenRevoked :one
UPDATE refresh_tokens SET revoked = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: RevokeAllRefreshTokensForUser :execrows
UPDATE refresh_tokens SET revoked = true, updated_at = now()
WHERE user_id = $1 AND revoked = false;

-- name: DeleteExpiredRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE expires_at < $1;

-- name: DeleteOldRevokedRefreshTokens :execrows
DELETE FROM refresh_tokens WHERE revoked = true AND updated_at < $1;
```

`go/db/queries/outlets.sql`:

```sql
-- name: CreateOutlet :one
INSERT INTO outlets (name, latitude, longitude, radius_meters)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetOutletByID :one
SELECT * FROM outlets WHERE id = $1 AND removed_at IS NULL LIMIT 1;

-- name: UpdateOutlet :one
UPDATE outlets
SET name = $2, latitude = $3, longitude = $4, radius_meters = $5, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateOutletGeofence :one
UPDATE outlets SET geofence_enabled = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: DeleteOutlet :one
UPDATE outlets SET removed_at = now(), removed_by_user_id = $2, updated_at = now()
WHERE id = $1 RETURNING *;

-- name: CreateMembership :one
INSERT INTO outlet_memberships
  (outlet_id, user_id, role, status, display_name, invited_by_user_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMembershipByOutletAndUser :one
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2 AND removed_at IS NULL LIMIT 1;

-- name: GetMembershipByOutletAndUserIncludingRemoved :one
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2 LIMIT 1;

-- name: GetMembershipByIDIncludingRemoved :one
SELECT * FROM outlet_memberships WHERE id = $1 LIMIT 1;

-- name: UpdateMembershipInvite :one
UPDATE outlet_memberships
SET role = $2, status = $3, invited_by_user_id = $4, removed_at = NULL, removed_by_user_id = NULL, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateMembershipStatus :one
UPDATE outlet_memberships SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateMembershipDisplayName :one
UPDATE outlet_memberships SET display_name = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: RemoveMembership :one
UPDATE outlet_memberships
SET removed_at = now(), removed_by_user_id = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListMembershipsForUserByStatus :many
SELECT m.*, o.id AS outlet_id, o.name AS outlet_name, o.latitude, o.longitude, o.radius_meters,
       o.geofence_enabled, o.removed_at AS outlet_removed_at, o.created_at AS outlet_created_at,
       o.updated_at AS outlet_updated_at
FROM outlet_memberships m
JOIN outlets o ON o.id = m.outlet_id
WHERE m.user_id = $1 AND m.status = $2 AND m.removed_at IS NULL AND o.removed_at IS NULL
ORDER BY m.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountMembershipsForUserByStatus :one
SELECT count(*) FROM outlet_memberships m
JOIN outlets o ON o.id = m.outlet_id
WHERE m.user_id = $1 AND m.status = $2 AND m.removed_at IS NULL AND o.removed_at IS NULL;

-- name: ListMembershipsForOutlet :many
SELECT m.*, u.id AS user_id, u.name AS user_name, u.email AS user_email,
       iu.id AS invited_by_user_id, iu.name AS invited_by_user_name
FROM outlet_memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN users iu ON iu.id = m.invited_by_user_id
WHERE m.outlet_id = $1 AND m.removed_at IS NULL
ORDER BY m.created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountMembershipsForOutlet :one
SELECT count(*) FROM outlet_memberships WHERE outlet_id = $1 AND removed_at IS NULL;

-- name: ListMembershipsForOutletByUser :many
SELECT * FROM outlet_memberships
WHERE outlet_id = $1 AND user_id = $2
ORDER BY created_at ASC;
```

Note: when a JOIN references the same column name from multiple tables (e.g. `id`, `created_at`), sqlc names the Go field after the qualified alias you give it (`outlet_id`, `outlet_name`, …). The generated row struct will be `ListMembershipsForUserByStatusRow` with embedded `OutletMembership` + outlet fields. For `ListMembershipsForOutlet`, alias membership columns too if the generator complains about ambiguity; follow sqlc's generated field names and use them in `outlet` package.

`go/db/queries/attendance.sql`:

```sql
-- name: CreateAttendanceEntry :one
INSERT INTO attendance_entries (user_id, outlet_id, type, entry_time, latitude, longitude, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAttendanceEntryByIDAndOutlet :one
SELECT * FROM attendance_entries WHERE id = $1 AND outlet_id = $2 LIMIT 1;

-- name: UpdateAttendanceEntry :one
UPDATE attendance_entries
SET type = $2, entry_time = $3, latitude = $4, longitude = $5, updated_by = $6, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteAttendanceEntry :exec
DELETE FROM attendance_entries WHERE id = $1;

-- name: ListAttendanceByOutlet :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1
ORDER BY entry_time DESC, created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountAttendanceByOutlet :one
SELECT count(*) FROM attendance_entries WHERE outlet_id = $1;

-- name: ListAttendanceByOutletAndUser :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1 AND user_id = $2
ORDER BY entry_time DESC, created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountAttendanceByOutletAndUser :one
SELECT count(*) FROM attendance_entries WHERE outlet_id = $1 AND user_id = $2;

-- name: ListAttendanceByOutletUserRange :many
SELECT * FROM attendance_entries
WHERE outlet_id = $1 AND user_id = $2 AND entry_time >= $3 AND entry_time < $4
ORDER BY entry_time ASC;
```

`go/db/queries/audit.sql`:

```sql
-- name: InsertAuditEvent :one
INSERT INTO audit_events (actor_user_id, action, entity_type, entity_id, metadata_json, ip_address, user_agent)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
```

`go/db/queries/reports.sql` is the same range query as `ListAttendanceByOutletUserRange` (import it from the `attendance` queries instead — reports.sql only needed if the report package wants its own file; it does NOT: use `ListAttendanceByOutletUserRange`).

`go/db/sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"
    queries: "queries"
    gen:
      go:
        package: "db"
        out: "."
        emit_interface: true
        emit_empty_slices: true
        emit_exported_queries: false
```

- [ ] **Step 4: Generate sqlc code and verify it compiles**

```bash
cd go && sqlc generate && go build ./... && cd ..
```

If `sqlc` is not installed: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` then `export PATH=$PATH:$(go env GOPATH)/bin`.
Expected: generated files under `go/db/` (models.go, db.go, querier.go, *.sql.go) and clean build. If `gen_random_uuid()` is unknown to sqlc's parser, add `create extension if not exists pgcrypto;` to the top of the up migration (harmless; PostgreSQL 13+ ships `gen_random_uuid()` built-in).

- [ ] **Step 5: Commit**

```bash
git add go/db
git commit -m "feat(go): db pool, embedded migrations, sqlc queries"
```

---

### Task 3: `httpapi` core — errors, responses, pagination, context

**Files:**
- Create: `go/httpapi/errors.go`
- Create: `go/httpapi/errors_test.go`
- Create: `go/httpapi/response.go`
- Create: `go/httpapi/pagination.go`
- Create: `go/httpapi/pagination_test.go`
- Create: `go/httpapi/context.go`

**Interfaces:**
- Produces:
  - `func BadRequest(msg string) *APIError`, `Conflict`, `Forbidden`, `NotFound`, `InvalidToken`, `Validation`, `RateLimitExceeded` (all `func(string) *APIError`), `func Internal(msg string) *APIError`
  - `type APIError struct { Status int; Code string; Message string }` with `func (e *APIError) Error() string`
  - `type ErrorResponse struct { Code string \`json:"code"\`; Message string \`json:"message"\`; Timestamp time.Time \`json:"timestamp"\` }`
  - `func WriteJSON(w http.ResponseWriter, status int, v any)`
  - `func WriteError(w http.ResponseWriter, err error)`
  - `func WritePage[T any](w http.ResponseWriter, content []T, total int64, p PageParams)`
  - `type SortOrder struct { Field string; Desc bool }`
  - `type PageParams struct { Page int; Size int; Sorted bool; Sort []SortOrder }`
  - `func ParsePageParams(r *http.Request) PageParams`
  - `func (p PageParams) OrderClause(sortable map[string]string) (string, []any)`
  - `type Subject interface { SubjectID() string }`
  - `func WithSubject(r *http.Request, s Subject) *http.Request`
  - `func SubjectFrom(r *http.Request) Subject`
  - `func SubjectID(r *http.Request) string`

- [ ] **Step 1: Write the failing tests**

```go
// errors_test.go
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, NotFound("Outlet not found: x"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "NOT_FOUND" || body.Message != "Outlet not found: x" {
		t.Errorf("body = %+v", body)
	}
	if body.Timestamp.IsZero() {
		t.Error("timestamp missing")
	}
}

func TestWriteErrorUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body ErrorResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Code != "INTERNAL_ERROR" {
		t.Errorf("code = %q, want INTERNAL_ERROR", body.Code)
	}
}
```

```go
// pagination_test.go
package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestParsePageParamsDefaults(t *testing.T) {
	p := ParsePageParams(httptest.NewRequest("GET", "/x", nil))
	if p.Page != 0 || p.Size != 20 || p.Sorted {
		t.Fatalf("defaults wrong: %+v", p)
	}
}

func TestParsePageParamsFull(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?page=2&size=10&sort=updatedAt,desc&sort=name,asc", nil)
	p := ParsePageParams(req)
	if p.Page != 2 || p.Size != 10 || !p.Sorted {
		t.Fatalf("parse wrong: %+v", p)
	}
	if len(p.Sort) != 2 || p.Sort[0] != (SortOrder{"updatedAt", true}) || p.Sort[1] != (SortOrder{"name", false}) {
		t.Fatalf("sort wrong: %+v", p.Sort)
	}
}

func TestParsePageParamsBadValuesFallBack(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?page=abc&size=-5", nil)
	p := ParsePageParams(req)
	if p.Page != 0 || p.Size != 20 {
		t.Fatalf("fallback wrong: %+v", p)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./httpapi/`
Expected: FAIL (types undefined).

- [ ] **Step 3: Write the implementation**

```go
// errors.go
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func BadRequest(msg string) *APIError        { return &APIError{http.StatusBadRequest, "BAD_REQUEST", msg} }
func Conflict(msg string) *APIError          { return &APIError{http.StatusConflict, "CONFLICT", msg} }
func Forbidden(msg string) *APIError         { return &APIError{http.StatusForbidden, "FORBIDDEN", msg} }
func NotFound(msg string) *APIError          { return &APIError{http.StatusNotFound, "NOT_FOUND", msg} }
func InvalidToken(msg string) *APIError      { return &APIError{http.StatusUnauthorized, "INVALID_TOKEN", msg} }
func Validation(msg string) *APIError        { return &APIError{http.StatusBadRequest, "VALIDATION_ERROR", msg} }
func RateLimitExceeded(msg string) *APIError { return &APIError{http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", msg} }
func Internal(msg string) *APIError          { return &APIError{http.StatusInternalServerError, "INTERNAL_ERROR", msg} }

type ErrorResponse struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
```

```go
// response.go
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

func WriteError(w http.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.Status, ErrorResponse{apiErr.Code, apiErr.Message, time.Now().UTC()})
		return
	}
	slog.Error("unhandled error", "err", err)
	WriteJSON(w, http.StatusInternalServerError, ErrorResponse{"INTERNAL_ERROR", "Internal server error", time.Now().UTC()})
}
```

```go
// pagination.go
package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

type SortOrder struct {
	Field string
	Desc  bool
}

type PageParams struct {
	Page   int
	Size   int
	Sorted bool
	Sort   []SortOrder
}

func ParsePageParams(r *http.Request) PageParams {
	p := PageParams{Page: 0, Size: 20}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Size = n
		}
	}
	for _, raw := range r.URL.Query()["sort"] {
		for _, part := range strings.Split(raw, ",") {
			if part == "" {
				continue
			}
			seg := strings.Split(part, ",")
			field, dir := seg[0], ""
			if len(seg) > 1 {
				dir = seg[1]
			}
			p.Sort = append(p.Sort, SortOrder{Field: field, Desc: strings.EqualFold(dir, "desc")})
			p.Sorted = true
		}
	}
	return p
}

// OrderClause maps requested sort fields to SQL column names; unknown fields
// are skipped. Returns "" when there are no sortable columns. The caller
// applies its default sort when p.Sorted is false.
func (p PageParams) OrderClause(sortable map[string]string) (string, []any) {
	cols := make([]string, 0, len(p.Sort))
	args := make([]any, 0)
	for _, so := range p.Sort {
		col, ok := sortable[so.Field]
		if !ok {
			continue
		}
		dir := "ASC"
		if so.Desc {
			dir = "DESC"
		}
		cols = append(cols, col+" "+dir)
	}
	if len(cols) == 0 {
		return "", nil
	}
	return " ORDER BY " + strings.Join(cols, ", "), args
}
```

```go
// context.go
package httpapi

import (
	"context"
	"net/http"
)

type ctxKey int

const subjectKey ctxKey = iota

// Subject is implemented by the authenticated user model (db.User) and
// anything else that needs to be attached to the request context.
type Subject interface {
	SubjectID() string
}

func WithSubject(r *http.Request, s Subject) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), subjectKey, s))
}

func SubjectFrom(r *http.Request) Subject {
	if s, ok := r.Context().Value(subjectKey).(Subject); ok {
		return s
	}
	return nil
}

func SubjectID(r *http.Request) string {
	if s := SubjectFrom(r); s != nil {
		return s.SubjectID()
	}
	return ""
}
```

Note: the `db` `User` model must implement `SubjectID() string` — the sqlc model is generated as a plain struct; add a small method file `go/db/subject.go`:

```go
package db

func (u User) SubjectID() string { return u.ID.String() }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go test ./httpapi/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/httpapi go/db/subject.go
git commit -m "feat(go): httpapi error envelope, responses, pagination, subject context"
```

---

### Task 4: `httpapi` middleware

**Files:**
- Create: `go/httpapi/middleware.go`
- Create: `go/httpapi/middleware_test.go`

**Interfaces:**
- Produces:
  - `func RequestID(next http.Handler) http.Handler` — uses `X-Request-Id` else generated UUID; sets response header `X-Request-Id`; stores in request context via `WithRequestID`.
  - `func WithRequestID(r *http.Request, id string) *http.Request` / `func RequestIDFrom(r *http.Request) string`
  - `func RequestLog(logger *slog.Logger, trustProxyHeaders bool) func(http.Handler) http.Handler` — INFO line: `method path completed with status=... durationMs=... clientIp=... requestId=... userId=...`
  - `func Recoverer(next http.Handler) http.Handler` — recovers panics → 500 envelope.
  - `func SecurityHeaders(next http.Handler) http.Handler` — exact headers from spec §Request logging & headers.
  - `func BodyLimit(maxBytes int64) func(http.Handler) http.Handler` — `http.MaxBytesReader` per request.
  - `func ClientIP(r *http.Request, trustProxyHeaders bool) string` — X-Forwarded-For first entry, then X-Real-IP, else RemoteAddr.
- Consumes: `httpapi.WriteError` (Task 3).

- [ ] **Step 1: Write the failing tests**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).
		ServeHTTP(rec, req)
	for _, h := range []string{"X-Frame-Options", "Referrer-Policy", "Permissions-Policy",
		"Content-Security-Policy", "Strict-Transport-Security"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Errorf("X-Frame-Options = %q", rec.Header().Get("X-Frame-Options"))
	}
}

func TestRequestID(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	var got string
	RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = RequestIDFrom(r) })).
		ServeHTTP(rec, req)
	if got == "" || rec.Header().Get("X-Request-Id") == "" || got != rec.Header().Get("X-Request-Id") {
		t.Fatalf("request id mismatch: ctx=%q header=%q", got, rec.Header().Get("X-Request-Id"))
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	if got := ClientIP(req, true); got != "203.0.113.5" {
		t.Errorf("trusted = %q", got)
	}
	if got := ClientIP(req, false); got != "10.0.0.1" {
		t.Errorf("untrusted = %q", got)
	}
}

func TestBodyLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"x":"`+strings.Repeat("a", 100)+`"}`))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	BodyLimit(32)(next).ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", rec.Code)
	}
}

func TestRecoverer(t *testing.T) {
	rec := httptest.NewRecorder()
	Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })).
		ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./httpapi/`
Expected: FAIL (functions undefined).

- [ ] **Step 3: Write the implementation**

```go
package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ctxKey int

const requestIDKey ctxKey = iota

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, WithRequestID(r, id))
	})
}

func WithRequestID(r *http.Request, id string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
}

func RequestIDFrom(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func RequestLog(logger *slog.Logger, trustProxyHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			logger.Info(fmt.Sprintf("HTTP %s %s completed", r.Method, r.URL.Path),
				"status", rec.status,
				"durationMs", time.Since(start).Milliseconds(),
				"clientIp", ClientIP(r, trustProxyHeaders),
				"requestId", RequestIDFrom(r),
				"userId", SubjectID(r),
			)
		})
	}
}

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "err", rec, "requestId", RequestIDFrom(r))
				WriteError(w, Internal("Internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return strings.TrimSpace(realIP)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go test ./httpapi/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/httpapi
git commit -m "feat(go): httpapi middleware (request id, logging, recovery, headers, body limit)"
```

---

### Task 5: `metrics` + `audit` packages

**Files:**
- Create: `go/metrics/metrics.go`
- Create: `go/audit/audit.go`

**Interfaces:**
- Produces:
  - `func NewRegistry() *Registry`
  - `func (r *Registry) Increment(name string, tags ...string)` — even number of tag key/values; missing counter name is created lazily.
  - `func (r *Registry) Handler() http.Handler` — promhttp handler.
  - `type Recorder struct { Store *db.Store }`
  - `func NewRecorder(store *db.Store) *Recorder`
  - `func (r *Recorder) Record(ctx context.Context, actorUserID, action, entityType string, entityID uuid.UUID, metadata map[string]any, ip, userAgent string)` — best-effort, separate short transaction, logs on failure, never returns error.

- [ ] **Step 1: Write the implementation (no unit test needed — covered by integration)**

```go
// metrics/metrics.go
package metrics

import (
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Registry struct {
	mu       sync.Mutex
	counters map[string]*prometheus.CounterVec
	reg      *prometheus.Registry
}

func NewRegistry() *Registry {
	r := &Registry{
		counters: make(map[string]*prometheus.CounterVec),
		reg:      prometheus.NewRegistry(),
	}
	r.reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return r
}

func (r *Registry) Increment(name string, tags ...string) {
	if len(tags)%2 != 0 {
		tags = tags[:len(tags)-1]
	}
	var labelNames []string
	for i := 0; i < len(tags); i += 2 {
		labelNames = append(labelNames, tags[i])
	}
	key := name + "|" + strings.Join(labelNames, ",")
	r.mu.Lock()
	cv, ok := r.counters[key]
	if !ok {
		cv = prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: name}, labelNames)
		r.reg.MustRegister(cv)
		r.counters[key] = cv
	}
	r.mu.Unlock()
	cv.WithLabelValues(labelValues(tags)...).Inc()
}

func labelValues(tags []string) []string {
	var vals []string
	for i := 0; i < len(tags); i += 2 {
		vals = append(vals, tags[i+1])
	}
	return vals
}

func (r *Registry) Handler() http.Handler { return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{}) }
```

```go
// audit/audit.go
package audit

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/google/uuid"
)

type Recorder struct {
	store *db.Store
}

func NewRecorder(store *db.Store) *Recorder { return &Recorder{store: store} }

// Record persists an audit event in its own transaction so a failure here
// never rolls back the business write. Failures are logged and dropped.
func (r *Recorder) Record(ctx context.Context, actorUserID, action, entityType string, entityID uuid.UUID, metadata map[string]any, ip, userAgent string) {
	var metaJSON []byte
	var err error
	if len(metadata) > 0 {
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			slog.Error("audit: marshal metadata", "action", action, "err", err)
			return
		}
	}
	ua := userAgent
	if len(ua) > 500 {
		ua = ua[:500]
	}
	ipStr := ip
	if len(ipStr) > 100 {
		ipStr = ipStr[:100]
	}

	txCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := r.store.Tx(txCtx, func(q *db.Queries) error {
		var actor *uuid.UUID
		if actorUserID != "" {
			if id, perr := uuid.Parse(actorUserID); perr == nil {
				actor = &id
			}
		}
		_, err := q.InsertAuditEvent(txCtx, db.InsertAuditEventParams{
			ActorUserID: actor,
			Action:      action,
			EntityType:  entityType,
			EntityID:    entityID,
			MetadataJson: metaJSON,
			IpAddress:   ipStr,
			UserAgent:   ua,
		})
		return err
	}); err != nil {
		slog.Error("audit: insert failed", "action", action, "err", err)
	}
}
```

Note: the `InsertAuditEventParams` field names/`MetadataJson` vs `MetadataJSON` depend on sqlc output (it may name it `MetadataJSON`). Adjust to match the generated struct exactly.

- [ ] **Step 2: Verify compile**

Run: `cd go && go build ./...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add go/metrics go/audit
git commit -m "feat(go): metrics registry and audit recorder"
```

---

### Task 6: Health endpoints + `main.go` wiring + server lifecycle

**Files:**
- Create: `go/httpapi/router.go`
- Create: `go/cmd/delta/main.go`

**Interfaces:**
- Produces:
  - `func NewRouter(logger *slog.Logger, cfg config.Config, ready func(ctx context.Context) error, metrics http.Handler, mux *http.ServeMux) http.Handler` — applies middleware chain (RequestID → Recoverer → SecurityHeaders → BodyLimit → RequestLog) over the provided mux; registers `GET /healthz` and `GET /readyz` and `GET /metrics` (bearer-gated when token set).
  - `main.go` — loads config, builds logger, opens pool, migrates if `AutoMigrate`, builds Store, constructs all services/handlers, registers API routes on the mux, runs server with timeouts and graceful shutdown, starts the refresh-token cleanup ticker (added in Task 8).
- Consumes: `config`, `db`, `httpapi` (Tasks 1–4), `metrics` (Task 5).

- [ ] **Step 1: Write router.go**

```go
package httpapi

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
)

// readyCheck is injected by main; it pings the DB and Firebase.
func NewRouter(logger *slog.Logger, trustProxyHeaders bool, prometheusToken string, ready func(ctx context.Context) error, metricsHandler http.Handler, mux *http.ServeMux) http.Handler {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context()); err != nil {
			WriteJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "DOWN", "error": err.Error()})
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	if metricsHandler != nil {
		if prometheusToken != "" {
			mux.Handle("GET /metrics", prometheusAuth(prometheusToken, metricsHandler))
		} else {
			mux.Handle("GET /metrics", metricsHandler)
		}
	}
	return Recoverer(SecurityHeaders(BodyLimit(2 << 20)(RequestLog(logger, trustProxyHeaders)(RequestID(mux)))))
}

func prometheusAuth(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, "Bearer ")), []byte(token)) != 1 {
			WriteError(w, Forbidden("Forbidden"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Add missing import `"strings"` to the import block.

- [ ] **Step 2: Write main.go (temporary API mux with placeholder health)**

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coderGtm/delta/go/config"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	logger := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
			logger.Error("migrate", "err", err)
			os.Exit(1)
		}
	}

	store := db.NewStore(pool)
	registry := metrics.NewRegistry()

	apiMux := http.NewServeMux()
	// API routes are registered here by later tasks.

	ready := func(ctx context.Context) error {
		return pool.Ping(ctx)
	}

	handler := httpapi.NewRouter(logger, cfg.TrustProxyHeaders, cfg.PrometheusBearerToken, ready, registry.Handler(), apiMux)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Info("server started", "addr", srv.Addr)

	select {
	case err := <-errCh:
		logger.Error("server error", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", "err", err)
		}
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
```

Add import `"strconv"`. Remove unused `errors` import if vet complains (it will): keep imports minimal.

- [ ] **Step 3: Build and smoke-test the server**

```bash
cd go && go build ./... && cd ..
```

Run with a local Postgres (compose `postgres` service) or skip run if no DB is available; the build is the gate here.
Expected: clean build.

- [ ] **Step 4: Commit**

```bash
git add go/httpapi/router.go go/cmd
git commit -m "feat(go): health endpoints, metrics endpoint, server lifecycle"
```

---

## Phase 1: Auth

### Task 7: JWT service

**Files:**
- Create: `go/auth/jwt.go`
- Create: `go/auth/jwt_test.go`

**Interfaces:**
- Produces:
  - `type JWTService struct { secret []byte; ttl time.Duration }`
  - `func NewJWTService(secret string, ttl time.Duration) *JWTService`
  - `func (s *JWTService) GenerateAccessToken(userID uuid.UUID) (string, error)` — HS256, `sub` = userID string, `iat`/`exp`.
  - `func (s *JWTService) ParseAccessToken(token string) (uuid.UUID, error)` — returns subject UUID; any parse/verify error → error.
- Consumes: `github.com/golang-jwt/jwt/v5`, `github.com/google/uuid`.

- [ ] **Step 1: Write the failing test**

```go
package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTGenerateParseRoundTrip(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", time.Minute)
	id := uuid.New()
	tok, err := s.GenerateAccessToken(id)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := s.ParseAccessToken(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != id {
		t.Errorf("subject = %v, want %v", got, id)
	}
}

func TestJWTParseWrongSecret(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", time.Minute)
	tok, _ := s.GenerateAccessToken(uuid.New())
	other := NewJWTService("wrong-secret-wrong-secret-wrong-secret-wrong", time.Minute)
	if _, err := other.ParseAccessToken(tok); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestJWTParseExpired(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", -time.Minute)
	tok, _ := s.GenerateAccessToken(uuid.New())
	if _, err := s.ParseAccessToken(tok); err == nil {
		t.Fatal("expected expiry error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./auth/`
Expected: FAIL (package does not compile — files not created yet; create jwt.go stub returning errors first, or accept compile error).

- [ ] **Step 3: Write the implementation**

```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTService struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTService(secret string, ttl time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: ttl}
}

func (s *JWTService) GenerateAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

func (s *JWTService) ParseAccessToken(token string) (uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return uuid.Nil, errors.New("invalid token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./auth/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/auth
git commit -m "feat(go): JWT access token service"
```

---

### Task 8: Refresh token service + cleanup ticker

**Files:**
- Create: `go/auth/refreshtoken.go`
- Create: `go/auth/refreshtoken_test.go`

**Interfaces:**
- Produces:
  - `type RefreshTokenService struct { store *db.Store; ttl time.Duration; retention time.Duration }`
  - `func NewRefreshTokenService(store *db.Store, ttl, retention time.Duration) *RefreshTokenService`
  - `func (s *RefreshTokenService) Create(ctx context.Context, userID uuid.UUID) (string, error)` — returns raw token; persists SHA-256 hex.
  - `func (s *RefreshTokenService) validate(ctx context.Context, raw string) (*db.RefreshToken, error)` — errors are `httpapi.InvalidToken("Invalid refresh token"|"Refresh token has been revoked"|"Refresh token has expired")`.
  - `func (s *RefreshTokenService) Revoke(ctx context.Context, raw string) error`
  - `func (s *RefreshTokenService) Rotate(ctx context.Context, raw string) (string, error)` — validates, revokes old, returns new.
  - `func (s *RefreshTokenService) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error`
  - `func (s *RefreshTokenService) Cleanup(ctx context.Context) (expired, revoked int64, err error)`
  - `func (s *RefreshTokenService) RunCleanupTicker(ctx context.Context, interval time.Duration)`
- Consumes: `db` queries, `httpapi.InvalidToken`.

- [ ] **Step 1: Write the failing tests**

```go
// pure-logic tests (hashing, expiry, validation errors) — no DB
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashToken(t *testing.T) {
	if h := hashToken("abc"); len(h) != 64 {
		t.Errorf("hash length = %d", len(h))
	}
	if hashToken("abc") != hashToken("abc") {
		t.Error("hash not deterministic")
	}
	if hashToken("abc") == hashToken("abd") {
		t.Error("hash collision on different input")
	}
}

func TestRotateInvalidToken(t *testing.T) {
	s := &RefreshTokenService{}
	if _, err := s.rotateInvalidForTest(); err == nil {
		t.Fatal("expected error")
	}
}
```

Note: `rotateInvalidForTest` is a placeholder to keep the test compiling; replace with a real DB integration test in the testcontainers suite (Task 14). The unit gate for this task is `hashToken` + token generation shape.

- [ ] **Step 2: Write the implementation**

```go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RefreshTokenService struct {
	store     *db.Store
	ttl       time.Duration
	retention time.Duration
}

func NewRefreshTokenService(store *db.Store, ttl, retention time.Duration) *RefreshTokenService {
	return &RefreshTokenService{store: store, ttl: ttl, retention: retention}
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *RefreshTokenService) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	raw, err := generateRandomToken()
	if err != nil {
		return "", err
	}
	_, err = s.store.Querier().CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		TokenHash: hashToken(raw),
		ExpiresAt: time.Now().UTC().Add(s.ttl),
		Revoked:   false,
		UserID:    userID,
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

func (s *RefreshTokenService) validate(ctx context.Context, raw string) (*db.RefreshToken, error) {
	row, err := s.store.Querier().GetRefreshTokenByHash(ctx, hashToken(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.InvalidToken("Invalid refresh token")
	}
	if err != nil {
		return nil, err
	}
	if row.Revoked {
		return nil, httpapi.InvalidToken("Refresh token has been revoked")
	}
	if row.ExpiresAt.Before(time.Now().UTC()) {
		return nil, httpapi.InvalidToken("Refresh token has expired")
	}
	return row, nil
}

func (s *RefreshTokenService) Revoke(ctx context.Context, raw string) error {
	row, err := s.validate(ctx, raw)
	if err != nil {
		return err
	}
	_, err = s.store.Querier().UpdateRefreshTokenRevoked(ctx, db.UpdateRefreshTokenRevokedParams{ID: row.ID, Revoked: true})
	return err
}

func (s *RefreshTokenService) Rotate(ctx context.Context, raw string) (string, error) {
	row, err := s.validate(ctx, raw)
	if err != nil {
		return "", err
	}
	if _, err := s.store.Querier().UpdateRefreshTokenRevoked(ctx, db.UpdateRefreshTokenRevokedParams{ID: row.ID, Revoked: true}); err != nil {
		return "", err
	}
	return s.Create(ctx, row.UserID)
}

func (s *RefreshTokenService) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.store.Querier().RevokeAllRefreshTokensForUser(ctx, userID)
	return err
}

func (s *RefreshTokenService) Cleanup(ctx context.Context) (expired, revoked int64, err error) {
	now := time.Now().UTC()
	expired, err = s.store.Querier().DeleteExpiredRefreshTokens(ctx, now)
	if err != nil {
		return 0, 0, err
	}
	revoked, err = s.store.Querier().DeleteOldRevokedRefreshTokens(ctx, now.Add(-s.retention))
	if err != nil {
		return 0, 0, err
	}
	return expired, revoked, nil
}

func (s *RefreshTokenService) RunCleanupTicker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expired, revoked, err := s.Cleanup(context.WithoutCancel(ctx))
				if err != nil {
					slog.Error("refresh token cleanup", "err", err)
					continue
				}
				if expired > 0 || revoked > 0 {
					slog.Info("refresh token cleanup removed tokens", "expired", expired, "revoked", revoked)
				}
			}
		}
	}()
}
```

- [ ] **Step 3: Run tests to verify they pass**

Run: `cd go && go test ./auth/`
Expected: PASS.

- [ ] **Step 4: Wire the ticker in main.go**

In `main.go` after building the store (ticker will be created in Task 9 once the service exists; for now leave a `// TODO(golang): start RefreshTokenService cleanup ticker in Task 9` marker? NO — no TODOs. The ticker is started in Task 9's Step 4 which wires real auth. Leave main.go as-is here; Task 9 wires everything.)

- [ ] **Step 5: Commit**

```bash
git add go/auth
git commit -m "feat(go): refresh token service with rotation, revocation, cleanup"
```

---

### Task 9: Firebase wrapper + auth service + middleware + handlers

**Files:**
- Create: `go/auth/firebase.go`
- Create: `go/auth/service.go`
- Create: `go/auth/middleware.go`
- Create: `go/auth/handlers.go`
- Create: `go/auth/service_test.go` (pure-logic auth flows)

**Interfaces:**
- Produces:
  - `type UserInfo struct { UID string; Name string; Email string; PhoneNumber string }`
  - `type Firebase interface { VerifyIDToken(ctx context.Context, token string) (*UserInfo, error); DeleteUser(ctx context.Context, uid string) error }`
  - `func NewFirebaseClient(ctx context.Context, serviceAccountPath string) (Firebase, error)` — returns a stub implementing the interface with clear "not configured" errors when the path is empty/missing (so the app boots without Firebase in dev/test).
  - `type Service struct { Store *db.Store; Firebase Firebase; JWT *JWTService; Refresh *RefreshTokenService; Audit *audit.Recorder; Metrics *metrics.Registry }`
  - `func NewService(store *db.Store, fb Firebase, jwt *JWTService, refresh *RefreshTokenService, audit *audit.Recorder, metrics *metrics.Registry) *Service`
  - `func (s *Service) CreateUser(ctx context.Context, uid, name, email, phone string) (*db.User, error)`
  - `func (s *Service) Login(ctx context.Context, firebaseIDToken string, ip, userAgent string) (*LoginResponse, error)`
  - `func (s *Service) Refresh(ctx context.Context, refreshToken string, userAgent string) (*RefreshTokenResponse, error)`
  - `func (s *Service) Logout(ctx context.Context, refreshToken string) error`
  - `func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, ip, userAgent string) error`
  - `func (s *Service) DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error`
  - `type LoginResponse struct { ID uuid.UUID; Name, Email, Phone, AccessToken, RefreshToken string; CreatedAt, UpdatedAt time.Time }` with JSON tags `id,name,email,phone,accessToken,refreshToken,createdAt,updatedAt`
  - `type RefreshTokenResponse struct { AccessToken, RefreshToken string }` JSON tags `accessToken,refreshToken`
  - `func AttachUser(jwt *JWTService, store *db.Store) func(http.Handler) http.Handler` — on valid bearer + active user, `httpapi.WithSubject`.
  - `func Require(next http.Handler) http.Handler` — no subject → 401 with `WWW-Authenticate: Bearer`, empty body.
  - Handlers: `type Handlers struct { Svc *Service }`; methods `Login`, `Refresh`, `Logout`, `LogoutAll` matching the route table, each `func(w http.ResponseWriter, r *http.Request)`.
- Consumes: Tasks 3, 4, 5, 7, 8; `db`.

- [ ] **Step 1: Write the failing tests (pure logic with a fake Firebase + stub refresh)**

```go
// service_test.go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeFirebase struct{ info *UserInfo; deleteErr error }

func (f fakeFirebase) VerifyIDToken(ctx context.Context, token string) (*UserInfo, error) {
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return f.info, nil
}
func (f fakeFirebase) DeleteUser(ctx context.Context, uid string) error { return f.deleteErr }

func TestVerifyIDTokenMapsClaims(t *testing.T) {
	fb := NewStubFirebase(&UserInfo{UID: "u1", Name: "N", Email: "e@x", PhoneNumber: "123"})
	info, err := fb.VerifyIDToken(context.Background(), "tok")
	if err != nil || info.UID != "u1" || info.Email != "e@x" {
		t.Fatalf("info = %+v err = %v", info, err)
	}
}

func TestUserInfoFromToken(t *testing.T) {
	// Verifies FirebaseUserInfo → db fields mapping used in CreateUser
	info := &UserInfo{UID: "u1", Name: "N", Email: "e@x", PhoneNumber: "123"}
	if info.UID != "u1" || info.PhoneNumber != "123" {
		t.Fatalf("unexpected info: %+v", info)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./auth/`
Expected: FAIL (types undefined).

- [ ] **Step 3: Write the implementation**

```go
// firebase.go
package auth

import (
	"context"
	"errors"
	"os"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

type UserInfo struct {
	UID         string
	Name        string
	Email       string
	PhoneNumber string
}

type Firebase interface {
	VerifyIDToken(ctx context.Context, token string) (*UserInfo, error)
	DeleteUser(ctx context.Context, uid string) error
}

type firebaseClient struct {
	verify func(ctx context.Context, token string) (*UserInfo, error)
	del    func(ctx context.Context, uid string) error
}

func (c *firebaseClient) VerifyIDToken(ctx context.Context, token string) (*UserInfo, error) { return c.verify(ctx, token) }
func (c *firebaseClient) DeleteUser(ctx context.Context, uid string) error                   { return c.del(ctx, uid) }

var errFirebaseNotConfigured = errors.New("firebase not configured")

// NewFirebaseClient returns a real Firebase-backed client when the service
// account file exists, otherwise a stub that fails with a clear error. Tests
// use NewStubFirebase instead.
func NewFirebaseClient(ctx context.Context, serviceAccountPath string) (Firebase, error) {
	if serviceAccountPath == "" {
		return nil, errFirebaseNotConfigured
	}
	if _, err := os.Stat(serviceAccountPath); err != nil {
		return nil, errFirebaseNotConfigured
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(serviceAccountPath))
	if err != nil {
		return nil, err
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return nil, err
	}
	return &firebaseClient{
		verify: func(ctx context.Context, token string) (*UserInfo, error) {
			tok, err := client.VerifyIDToken(ctx, token)
			if err != nil {
				return nil, err
			}
			phone, _ := tok.Claims["phone_number"].(string)
			return &UserInfo{UID: tok.UID, Name: tok.DisplayName, Email: tok.Email, PhoneNumber: phone}, nil
		},
		del: func(ctx context.Context, uid string) error {
			return client.DeleteUser(ctx, uid)
		},
	}, nil
}

// NewStubFirebase returns a fixed-info fake for tests.
func NewStubFirebase(info *UserInfo) Firebase {
	return &firebaseClient{
		verify: func(ctx context.Context, token string) (*UserInfo, error) { return info, nil },
		del:    func(ctx context.Context, uid string) error { return nil },
	}
}
```

```go
// service.go
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	Store   *db.Store
	Firebase Firebase
	JWT     *JWTService
	Refresh *RefreshTokenService
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

func NewService(store *db.Store, fb Firebase, jwt *JWTService, refresh *RefreshTokenService, a *audit.Recorder, m *metrics.Registry) *Service {
	return &Service{Store: store, Firebase: fb, JWT: jwt, Refresh: refresh, Audit: a, Metrics: m}
}

type LoginResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func (s *Service) CreateUser(ctx context.Context, uid, name, email, phone string) (*db.User, error) {
	return s.Store.Querier().CreateUser(ctx, db.CreateUserParams{
		AuthUid: &uid,
		Name:    name,
		Email:   &email,
		Phone:   &phone,
	})
}

func (s *Service) Login(ctx context.Context, firebaseIDToken, ip, userAgent string) (*LoginResponse, error) {
	info, err := s.Firebase.VerifyIDToken(ctx, firebaseIDToken)
	if err != nil {
		return nil, httpapi.InvalidToken("Invalid Firebase ID Token")
	}
	user, err := s.Store.Querier().GetUserByAuthUID(ctx, info.UID)
	if errors.Is(err, pgx.ErrNoRows) {
		user, err = s.CreateUser(ctx, info.UID, info.Name, info.Email, info.PhoneNumber)
	}
	if err != nil {
		return nil, err
	}
	accessToken, err := s.JWT.GenerateAccessToken(user.ID)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.Refresh.Create(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("auth.login.success")
	s.Audit.Record(ctx, user.ID.String(), "AUTH_LOGIN", "USER", user.ID, map[string]any{"email": strOrNil(user.Email)}, ip, userAgent)
	return &LoginResponse{
		ID: user.ID, Name: user.Name, Email: strOrNil(user.Email), Phone: strOrNil(user.Phone),
		AccessToken: accessToken, RefreshToken: refreshToken,
		CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string, userAgent string) (*RefreshTokenResponse, error) {
	// Rotate returns the user from the token row
	userID, err := s.rotateUserID(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	accessToken, err := s.JWT.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("auth.refresh.success")
	s.Audit.Record(ctx, userID.String(), "AUTH_REFRESH", "USER", userID, nil, "", userAgent)
	return &RefreshTokenResponse{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}
```

Note: `s.rotateUserID` must return the new refresh token too. Refine: the `Refresh` flow needs the rotated (new) refresh token to return. Change `RefreshTokenService.Rotate` to also expose the user. Simplest: have `Refresh` call `Refresh.Rotate(ctx, raw)` to get the new raw token, but `Rotate` doesn't return the user. Add a method `RotateWithUser(ctx, raw) (newRaw string, userID uuid.UUID, err error)` to `RefreshTokenService` (Task 8) that validates, revokes old, creates new, and returns the user ID from the old row. Implement `Rotate` in terms of it. Update the `Refresh` method to use `RotateWithUser`. Also update `Logout`/`LogoutAll`/`DeleteAccount` below accordingly.

```go
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if err := s.Refresh.Revoke(ctx, refreshToken); err != nil {
		return err
	}
	s.Metrics.Increment("auth.logout.success")
	return nil
}

func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, ip, userAgent string) error {
	if err := s.Refresh.RevokeAllForUser(ctx, userID); err != nil {
		return err
	}
	s.Metrics.Increment("auth.logout_all.success")
	s.Audit.Record(ctx, userID.String(), "AUTH_LOGOUT_ALL", "USER", userID, nil, ip, userAgent)
	return nil
}

func (s *Service) DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error {
	if user.DeletedAt != nil {
		return httpapi.Conflict("Account has already been deleted")
	}
	if err := s.Firebase.DeleteUser(ctx, strOrNil(user.AuthUid)); err != nil {
		return httpapi.Conflict("Failed to delete the user from the authentication provider")
	}
	if err := s.Refresh.RevokeAllForUser(ctx, user.ID); err != nil {
		return err
	}
	deleted, err := s.Store.Querier().DeleteUserRow(ctx, user.ID)
	if err != nil {
		return err
	}
	s.Metrics.Increment("user.deleted")
	s.Audit.Record(ctx, user.ID.String(), "USER_DELETED", "USER", user.ID, map[string]any{"historicalEmail": strOrNil(deleted.HistoricalEmail)}, ip, userAgent)
	return nil
}

func strOrNil(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

```go
// middleware.go
package auth

import (
	"net/http"
	"strings"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/jackc/pgx/v5"
)

func AttachUser(jwt *JWTService, store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hdr := r.Header.Get("Authorization")
			if strings.HasPrefix(hdr, "Bearer ") {
				token := strings.TrimPrefix(hdr, "Bearer ")
				if userID, err := jwt.ParseAccessToken(token); err == nil {
					if user, err := store.Querier().GetUserByID(r.Context(), userID); err == nil {
						next.ServeHTTP(w, httpapi.WithSubject(r, user))
						return
					} else if err != pgx.ErrNoRows {
						// transient DB error: fall through as anonymous
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpapi.SubjectFrom(r) == nil {
			w.Header().Set("WWW-Authenticate", "Bearer")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Note: `httpapi.InvalidToken` is used by `Require`? No — keep the empty-body 401 for parity with Spring's default unauthenticated response. Remove unused import `httpapi` if not used; here `httpapi.WithSubject`/`SubjectFrom` are used, keep it.

```go
// handlers.go
package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

type Handlers struct {
	Svc *Service
	Store *db.Store
}

type loginRequest struct {
	FirebaseIDToken string `json:"firebaseIdToken"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.FirebaseIDToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.FirebaseIDToken) > 8192 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 8192"))
		return
	}
	resp, err := h.Svc.Login(r.Context(), req.FirebaseIDToken, httpapi.ClientIP(r, true), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.RefreshToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.RefreshToken) > 512 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 512"))
		return
	}
	resp, err := h.Svc.Refresh(r.Context(), req.RefreshToken, r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.RefreshToken == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	if len(req.RefreshToken) > 512 {
		httpapi.WriteError(w, httpapi.Validation("size must be between 0 and 512"))
		return
	}
	if err := h.Svc.Logout(r.Context(), req.RefreshToken); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(httpapi.SubjectID(r))
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	if err := h.Svc.LogoutAll(r.Context(), userID, httpapi.ClientIP(r, true), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// decodeJSON decodes a JSON body bounded by the middleware body limit.
func decodeJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return httpapi.BadRequest("Malformed request body")
	}
	return nil
}
```

Note: `h.Store` and `errors` may be unused — drop them if so. The `decodeJSON` helper belongs in `httpapi` (used by all domains): add it to `httpapi/response.go` in Task 3 as `func DecodeJSON(r *http.Request, dst any) error` and use that instead. Update references accordingly.

- [ ] **Step 4: Wire auth into main.go**

In `main.go`, after `store := db.NewStore(pool)`:

```go
fb, _ := auth.NewFirebaseClient(ctx, cfg.FirebaseServiceAccountPath)
if fb == nil {
	logger.Warn("firebase not configured; using stub", "path", cfg.FirebaseServiceAccountPath)
}
jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.AccessTokenTTL)
refreshSvc := auth.NewRefreshTokenService(store, cfg.RefreshTokenTTL, cfg.RefreshRevokedRetention)
recorder := audit.NewRecorder(store)
authSvc := auth.NewService(store, fb, jwtSvc, refreshSvc, recorder, registry)
authHandlers := &auth.Handlers{Svc: authSvc}

apiMux.Handle("POST /api/v1/auth/login", authHandlers.Login)
apiMux.Handle("POST /api/v1/auth/refresh", authHandlers.Refresh)
apiMux.Handle("POST /api/v1/auth/logout", authHandlers.Logout)
apiMux.Handle("POST /api/v1/auth/logout-all", auth.Require(authHandlers.LogoutAll))

go refreshSvc.RunCleanupTicker(ctx, cfg.RefreshCleanupInterval)
```

Also update `httpapi.NewRouter` call signature if needed (it already takes `ready`); add `attached := auth.AttachUser(jwtSvc, store)` and wrap `apiMux` in the middleware chain. Change `NewRouter` to accept the attach middleware or apply it inside: simplest — in main, `apiMux` already has public routes; wrap the whole thing: `handler := httpapi.NewRouter(..., auth.AttachUser(jwtSvc, store)(apiMux))` won't work since NewRouter builds the mux. Adjust `NewRouter` to accept an `attach func(http.Handler) http.Handler` parameter applied around the mux before logging. Keep it simple: `NewRouter(logger, cfg, ready, metricsHandler, auth.AttachUser(jwtSvc, store), apiMux)` — update Task 6's router signature to `NewRouter(logger *slog.Logger, cfg config.Config, ready func(ctx context.Context) error, metricsHandler http.Handler, attach func(http.Handler) http.Handler, mux *http.ServeMux) http.Handler` and chain `attach(mux)` inside.

Note `NewFirebaseClient` returns `(Firebase, error)`; if the file is missing, main should proceed with a stub that returns `errFirebaseNotConfigured` on use → login then fails with `INVALID_TOKEN` "Invalid Firebase ID Token". That's acceptable for tests; in production the path is mounted.

- [ ] **Step 5: Build, test, commit**

```bash
cd go && go build ./... && go test ./auth/ ./httpapi/ && cd ..
```
Expected: PASS.
```bash
git add go/auth go/cmd go/httpapi
git commit -m "feat(go): firebase, auth service, middleware, handlers"
```

---

## Phase 2: Users

### Task 10: `user` package — account deletion handler

**Files:**
- Create: `go/user/service.go`
- Create: `go/user/handlers.go`

**Interfaces:**
- Produces:
  - `type AccountDeleter interface { DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error }` (implemented by `auth.Service`).
  - `type UserHandler struct { Deleter AccountDeleter; Store *db.Store }`
  - `func NewHandler(deleter AccountDeleter, store *db.Store) *UserHandler`
  - `func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request)` — reads subject from context, loads `db.User` by id, calls `Deleter.DeleteAccount`, 204.
- Consumes: `httpapi` (SubjectFrom, ClientIP), `db`, `auth` interface only (no import).

- [ ] **Step 1: Write the implementation**

```go
// service.go
package user

import (
	"context"

	"github.com/coderGtm/delta/go/db"
)

type AccountDeleter interface {
	DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error
}
```

```go
// handlers.go
package user

import (
	"net/http"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

type UserHandler struct {
	Deleter AccountDeleter
	Store   *db.Store
}

func NewHandler(deleter AccountDeleter, store *db.Store) *UserHandler {
	return &UserHandler{Deleter: deleter, Store: store}
}

func (h *UserHandler) DeleteMe(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(httpapi.SubjectID(r))
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	u, err := h.Store.Querier().GetUserByID(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if err := h.Deleter.DeleteAccount(r.Context(), u, httpapi.ClientIP(r, true), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Wire in main.go**

```go
userHandlers := user.NewHandler(authSvc, store)
apiMux.Handle("DELETE /api/v1/users/me", auth.Require(userHandlers.DeleteMe))
```

- [ ] **Step 3: Build, commit**

```bash
cd go && go build ./... && cd ..
git add go/user go/cmd
git commit -m "feat(go): delete account endpoint"
```

---

## Phase 3: Outlets

### Task 11: Outlet service — outlet CRUD + geofence

**Files:**
- Create: `go/outlet/service.go`

**Interfaces:**
- Produces:
  - `type Service struct { Store *db.Store; Audit *audit.Recorder; Metrics *metrics.Registry }`
  - `func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service`
  - `func (s *Service) CreateOutlet(ctx context.Context, userID uuid.UUID, req CreateOutletRequest) (*OutletResponse, error)` — auto-creates OWNER/ACCEPTED membership (displayName = user name) in one tx.
  - `func (s *Service) GetOutlet(ctx context.Context, userID, outletID uuid.UUID) (*OutletResponse, error)`
  - `func (s *Service) UpdateOutlet(ctx context.Context, userID, outletID uuid.UUID, req UpdateOutletRequest) (*OutletResponse, error)`
  - `func (s *Service) UpdateGeofence(ctx context.Context, userID, outletID uuid.UUID, enabled bool) (*OutletResponse, error)`
  - `func (s *Service) DeleteOutlet(ctx context.Context, userID, outletID uuid.UUID) error`
  - DTO types `CreateOutletRequest`, `UpdateOutletRequest` (fields `Name string`, `Latitude float64`, `Longitude float64`, `RadiusMeters int` with JSON tags `name,latitude,longitude,radiusMeters`), `OutletResponse` (JSON tags `id,name,latitude,longitude,radiusMeters,geofenceEnabled,createdAt,updatedAt`).
- Consumes: `db`, `httpapi` errors, `audit`, `metrics`.

- [ ] **Step 1: Write the failing tests (validation + authorization rules; pure)**

```go
package outlet

import (
	"testing"

	"github.com/coderGtm/delta/go/httpapi"
)

func TestValidateCreateOutlet(t *testing.T) {
	ok := CreateOutletRequest{Name: "Shop", Latitude: 10, Longitude: 20, RadiusMeters: 100}
	if err := validateOutlet(ok); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	bad := CreateOutletRequest{Name: "", Latitude: 200, Longitude: 20, RadiusMeters: -1}
	if err := validateOutlet(bad); err == nil {
		t.Fatal("invalid request accepted")
	}
	var apiErr *httpapi.APIError
	if !errors.As(validateOutlet(bad), &apiErr) || apiErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("wrong error: %v", err)
	}
}
```

Add the missing `"errors"` import. Then:

```go
func TestMembershipGuard(t *testing.T) {
	// owner-only rule: non-owner role must yield FORBIDDEN
	s := &Service{}
	err := s.assertOwnerRoleForTest("EMPLOYEE")
	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "FORBIDDEN" {
		t.Fatalf("err = %v", err)
	}
}
```

`assertOwnerRoleForTest` is a placeholder hooking the real guard; implement it to call the shared `assertOwner` helper described below.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./outlet/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package outlet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CreateOutletRequest struct {
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters int     `json:"radiusMeters"`
}

type UpdateOutletRequest struct {
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMeters int     `json:"radiusMeters"`
}

type OutletResponse struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	RadiusMeters    int       `json:"radiusMeters"`
	GeofenceEnabled bool      `json:"geofenceEnabled"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Service struct {
	Store   *db.Store
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service {
	return &Service{Store: store, Audit: a, Metrics: m}
}

func validateOutlet(name string, lat, lon float64, radius int) *httpapi.APIError {
	var msgs []string
	if strings.TrimSpace(name) == "" {
		msgs = append(msgs, "must not be blank")
	}
	if len(name) > 150 {
		msgs = append(msgs, "size must be between 1 and 150")
	}
	if lat < -90 || lat > 90 {
		msgs = append(msgs, "latitude must be between -90 and 90")
	}
	if lon < -180 || lon > 180 {
		msgs = append(msgs, "longitude must be between -180 and 180")
	}
	if radius <= 0 {
		msgs = append(msgs, "must be greater than 0")
	}
	if len(msgs) > 0 {
		return httpapi.Validation(strings.Join(msgs, ", "))
	}
	return nil
}

func (s *Service) getActiveMembership(ctx context.Context, outletID, userID uuid.UUID) (*db.OutletMembership, error) {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{OutletID: outletID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return nil, err
	}
	if m.Status != "ACCEPTED" {
		return nil, httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	return m, nil
}

func (s *Service) assertOwner(ctx context.Context, outletID, userID uuid.UUID) (*db.User, error) {
	m, err := s.getActiveMembership(ctx, outletID, userID)
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" {
		return nil, httpapi.Forbidden("Only outlet owners can perform this action")
	}
	u, err := s.Store.Querier().GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Service) getActiveOutlet(ctx context.Context, outletID uuid.UUID) (*db.Outlet, error) {
	o, err := s.Store.Querier().GetOutletByID(ctx, outletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet not found: " + outletID.String())
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

func toOutletResponse(o *db.Outlet) *OutletResponse {
	return &OutletResponse{
		ID: o.ID, Name: o.Name,
		Latitude: float64(o.Latitude), Longitude: float64(o.Longitude),
		RadiusMeters: int(o.RadiusMeters), GeofenceEnabled: o.GeofenceEnabled,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

func (s *Service) CreateOutlet(ctx context.Context, userID uuid.UUID, req CreateOutletRequest) (*OutletResponse, error) {
	if err := validateOutlet(req.Name, req.Latitude, req.Longitude, req.RadiusMeters); err != nil {
		return nil, err
	}
	user, err := s.Store.Querier().GetUserByID(ctx, userID)
	if err != nil {
		return nil, httpapi.NotFound("Authenticated user was not found")
	}
	var out *db.Outlet
	err = s.Store.Tx(ctx, func(q *db.Queries) error {
		o, err := q.CreateOutlet(ctx, db.CreateOutletParams{
			Name: strings.TrimSpace(req.Name), Latitude: float64(req.Latitude), Longitude: float64(req.Longitude), RadiusMeters: int32(req.RadiusMeters),
		})
		if err != nil {
			return err
		}
		_, err = q.CreateMembership(ctx, db.CreateMembershipParams{
			OutletID: o.ID, UserID: userID, Role: "OWNER", Status: "ACCEPTED", DisplayName: user.Name,
		})
		out = o
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.created")
	s.Audit.Record(ctx, userID.String(), "OUTLET_CREATED", "OUTLET", out.ID, map[string]any{"name": out.Name}, "", "")
	return toOutletResponse(out), nil
}

func (s *Service) GetOutlet(ctx context.Context, userID, outletID uuid.UUID) (*OutletResponse, error) {
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveMembership(ctx, outletID, userID); err != nil {
		return nil, err
	}
	out, err := s.Store.Querier().GetOutletByID(ctx, outletID)
	if err != nil {
		return nil, err
	}
	return toOutletResponse(out), nil
}

func (s *Service) UpdateOutlet(ctx context.Context, userID, outletID uuid.UUID, req UpdateOutletRequest) (*OutletResponse, error) {
	if err := validateOutlet(req.Name, req.Latitude, req.Longitude, req.RadiusMeters); err != nil {
		return nil, err
	}
	if _, err := s.assertOwner(ctx, outletID, userID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	out, err := s.Store.Querier().UpdateOutlet(ctx, db.UpdateOutletParams{
		ID: outletID, Name: strings.TrimSpace(req.Name), Latitude: float64(req.Latitude), Longitude: float64(req.Longitude), RadiusMeters: int32(req.RadiusMeters),
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.updated")
	s.Audit.Record(ctx, userID.String(), "OUTLET_UPDATED", "OUTLET", outletID, map[string]any{"name": out.Name}, "", "")
	return toOutletResponse(out), nil
}

func (s *Service) UpdateGeofence(ctx context.Context, userID, outletID uuid.UUID, enabled bool) (*OutletResponse, error) {
	if _, err := s.assertOwner(ctx, outletID, userID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	out, err := s.Store.Querier().UpdateOutletGeofence(ctx, db.UpdateOutletGeofenceParams{ID: outletID, GeofenceEnabled: enabled})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.geofence.updated", "enabled", strconv.FormatBool(out.GeofenceEnabled))
	s.Audit.Record(ctx, userID.String(), "OUTLET_GEOFENCE_UPDATED", "OUTLET", outletID, map[string]any{"geofenceEnabled": out.GeofenceEnabled}, "", "")
	return toOutletResponse(out), nil
}

func (s *Service) DeleteOutlet(ctx context.Context, userID, outletID uuid.UUID) error {
	owner, err := s.assertOwner(ctx, outletID, userID)
	if err != nil {
		return err
	}
	out, err := s.Store.Querier().DeleteOutlet(ctx, db.DeleteOutletParams{ID: outletID, RemovedByUserID: &owner.ID})
	if err != nil {
		return err
	}
	s.Metrics.Increment("outlet.deleted")
	s.Audit.Record(ctx, userID.String(), "OUTLET_DELETED", "OUTLET", outletID, map[string]any{"name": out.Name}, "", "")
	return nil
}
```

Add missing imports: `"strconv"`, `"fmt"` as needed; the sqlc-generated param structs use the exact column types — adjust `float64`/`int32`/`*uuid.UUID`/`*string` casts to match generated `Params` types (numeric columns generate `float64` or `string` depending on sqlc config; if sqlc emits `string` for numeric, convert with `strconv.ParseFloat` — check `db/models.go` and adapt). The plan's casts are guidance; make them compile against the generated types.

Note: the tests reference `s.assertOwnerRoleForTest`; implement it as:

```go
func (s *Service) assertOwnerRoleForTest(role string) error {
	if role != "OWNER" {
		return httpapi.Forbidden("Only outlet owners can perform this action")
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go test ./outlet/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/outlet
git commit -m "feat(go): outlet CRUD and geofence service"
```

---

### Task 12: Membership service — invite/accept/reject/leave/remove/display-name, listings

**Files:**
- Modify: `go/outlet/service.go`

**Interfaces:**
- Produces (additions to `Service`):
  - `type MembershipResponse struct` (JSON tags `membershipId,outlet,userId,userName,userEmail,displayName,role,status,invitedByUserId,invitedByUserName,createdAt,updatedAt`; `outlet` is `*OutletResponse`).
  - `func (s *Service) GetMyOutlets(ctx context.Context, userID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error)`
  - `func (s *Service) GetMyInvites(...)` same
  - `func (s *Service) GetOutletMemberships(ctx context.Context, ownerID, outletID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error)`
  - `func (s *Service) InviteMember(ctx context.Context, ownerID, outletID uuid.UUID, email string) (*MembershipResponse, error)`
  - `func (s *Service) AcceptInvite(ctx context.Context, userID, membershipID uuid.UUID) (*MembershipResponse, error)`
  - `func (s *Service) RejectInvite(...) (*MembershipResponse, error)`
  - `func (s *Service) LeaveOutlet(ctx context.Context, userID, outletID uuid.UUID) error`
  - `func (s *Service) RemoveMembership(ctx context.Context, ownerID, outletID, membershipID uuid.UUID) error`
  - `func (s *Service) UpdateDisplayName(ctx context.Context, ownerID, outletID, membershipID uuid.UUID, name string) (*MembershipResponse, error)`
- Consumes: `db`, `httpapi`, `audit`, `metrics`.

- [ ] **Step 1: Write the failing tests (membership state machine, pure)**

```go
func TestInviteConflictRules(t *testing.T) {
	// active ACCEPTED → CONFLICT
	// pending INVITED → CONFLICT
	// REJECTED/removed → reopen INVITED
	s := &Service{}
	if err := s.inviteConflictForTest("ACCEPTED", nil); err == nil {
		t.Fatal("expected conflict for accepted active member")
	}
	var apiErr *httpapi.APIError
	if !errors.As(s.inviteConflictForTest("ACCEPTED", nil), &apiErr) || apiErr.Code != "CONFLICT" {
		t.Fatalf("wrong error: %v", err)
	}
	if err := s.inviteConflictForTest("INVITED", nil); err == nil {
		t.Fatal("expected conflict for pending invite")
	}
	if err := s.inviteConflictForTest("REJECTED", nil); err != nil {
		t.Fatalf("rejected membership should be reopened, got %v", err)
	}
	removedAt := time.Now()
	if err := s.inviteConflictForTest("ACCEPTED", &removedAt); err != nil {
		t.Fatalf("removed membership should be reopened, got %v", err)
	}
}

func TestAcceptOnlyInvited(t *testing.T) {
	s := &Service{}
	if err := s.acceptGuardForTest("ACCEPTED"); err == nil {
		t.Fatal("expected error accepting non-INVITED")
	}
	if err := s.acceptGuardForTest("INVITED"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

Implement the two test hooks:
- `inviteConflictForTest(status string, removedAt *time.Time) error`: if `removedAt == nil && status == "ACCEPTED"` → `httpapi.Conflict("User is already an active member of this outlet")`; if `removedAt == nil && status == "INVITED"` → `httpapi.Conflict("User already has a pending invitation for this outlet")`; else nil (reopen path).
- `acceptGuardForTest(status string) error`: if status != "INVITED" → `httpapi.BadRequest("Only pending invitations can be accepted")`; else nil.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./outlet/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation (append to service.go)**

```go
type MembershipResponse struct {
	MembershipID     uuid.UUID      `json:"membershipId"`
	Outlet           *OutletResponse `json:"outlet"`
	UserID           uuid.UUID      `json:"userId"`
	UserName         string         `json:"userName"`
	UserEmail        string         `json:"userEmail"`
	DisplayName      string         `json:"displayName"`
	Role             string         `json:"role"`
	Status           string         `json:"status"`
	InvitedByUserID  *uuid.UUID     `json:"invitedByUserId"`
	InvitedByUserName string        `json:"invitedByUserName"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

func (s *Service) toMembershipResponse(m *db.OutletMembership, user *db.User, outlet *db.Outlet, inviter *db.User) *MembershipResponse {
	return &MembershipResponse{
		MembershipID: m.ID, Outlet: toOutletResponse(outlet),
		UserID: user.ID, UserName: user.Name, UserEmail: strOrNil(user.Email),
		DisplayName: m.DisplayName, Role: m.Role, Status: m.Status,
		InvitedByUserID: m.InvitedByUserID, InvitedByUserName: inviterName(inviter),
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

func inviterName(u *db.User) string {
	if u == nil {
		return ""
	}
	return u.Name
}

func strOrNil(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// loadMembershipDetails loads the membership plus its outlet, member, and inviter.
func (s *Service) loadMembershipDetails(ctx context.Context, membershipID uuid.UUID) (*db.OutletMembership, *db.Outlet, *db.User, *db.User, error) {
	m, err := s.Store.Querier().GetMembershipByIDIncludingRemoved(ctx, membershipID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, nil, httpapi.NotFound("Outlet membership not found: " + membershipID.String())
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}
	outlet, err := s.Store.Querier().GetOutletByID(ctx, m.OutletID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil, nil, err
	}
	user, err := s.Store.Querier().GetUserByID(ctx, m.UserID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var inviter *db.User
	if m.InvitedByUserID != nil {
		inviter, err = s.Store.Querier().GetUserByID(ctx, *m.InvitedByUserID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, nil, err
		}
	}
	return m, outlet, user, inviter, nil
}
```

Note: `GetOutletByID` filters `removed_at IS NULL`; for the memberships list the outlet must be active anyway (spec), but for historical display after outlet deletion a removed outlet must still render. Use `db.GetOutletByID` but handle `pgx.ErrNoRows` (outlet hard-deleted) by treating as nil — acceptable edge; the spec only requires reads to survive soft-deletion, which `GetOutletByID` fails for. To keep parity with the Java `getOutletMemberships`/`acceptInvite` (which only require active outlets), this is fine.

```go
func (s *Service) GetMyOutlets(ctx context.Context, userID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	return s.listUserMemberships(ctx, userID, "ACCEPTED", p)
}

func (s *Service) GetMyInvites(ctx context.Context, userID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	return s.listUserMemberships(ctx, userID, "INVITED", p)
}

func (s *Service) listUserMemberships(ctx context.Context, userID uuid.UUID, status string, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	rows, err := s.Store.Querier().ListMembershipsForUserByStatus(ctx, db.ListMembershipsForUserByStatusParams{
		UserID: userID, Status: status, Limit: int32(p.Size), Offset: int32(p.Page * p.Size),
	})
	if err != nil {
		return nil, err
	}
	total, err := s.Store.Querier().CountMembershipsForUserByStatus(ctx, db.CountMembershipsForUserByStatusParams{UserID: userID, Status: status})
	if err != nil {
		return nil, err
	}
	out := make([]MembershipResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, MembershipResponse{
			MembershipID: row.OutletMembership.ID,
			Outlet: &OutletResponse{ID: row.OutletID, Name: row.OutletName, Latitude: row.Latitude, Longitude: row.Longitude,
				RadiusMeters: int(row.RadiusMeters), GeofenceEnabled: row.GeofenceEnabled, CreatedAt: row.OutletCreatedAt, UpdatedAt: row.OutletUpdatedAt},
			UserID: row.OutletMembership.UserID, UserName: "", UserEmail: "",
			DisplayName: row.OutletMembership.DisplayName, Role: row.OutletMembership.Role, Status: row.OutletMembership.Status,
			CreatedAt: row.OutletMembership.CreatedAt, UpdatedAt: row.OutletMembership.UpdatedAt,
		})
	}
	return httpapi.NewPageResponse(out, total, p), nil
}
```

Note: the generated `ListMembershipsForUserByStatusRow` field names must be verified against sqlc output; the JOIN aliases above define them. Where user name/email aren't selected in that query, fill via `loadMembershipDetails` per row if the response requires them. Simpler and correct: change the `ListMembershipsForUserByStatus` query to also `LEFT JOIN users u ON u.id = m.user_id` selecting `u.name AS user_name, u.email AS user_email` and include them. Do that when finalizing the query file in Task 2.

```go
func (s *Service) GetOutletMemberships(ctx context.Context, ownerID, outletID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	if _, err := s.assertOwner(ctx, outletID, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	rows, err := s.Store.Querier().ListMembershipsForOutlet(ctx, db.ListMembershipsForOutletParams{OutletID: outletID, Limit: int32(p.Size), Offset: int32(p.Page * p.Size)})
	if err != nil {
		return nil, err
	}
	total, err := s.Store.Querier().CountMembershipsForOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	out := make([]MembershipResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, MembershipResponse{
			MembershipID: row.OutletMembership.ID,
			Outlet:       toOutletResponse(mustOutlet(ctx, s, outletID)),
			UserID:       row.UserID, UserName: row.UserName, UserEmail: row.UserEmail,
			DisplayName: row.OutletMembership.DisplayName, Role: row.OutletMembership.Role, Status: row.OutletMembership.Status,
			InvitedByUserID: row.InvitedByUserID, InvitedByUserName: row.InvitedByUserName,
			CreatedAt: row.OutletMembership.CreatedAt, UpdatedAt: row.OutletMembership.UpdatedAt,
		})
	}
	return httpapi.NewPageResponse(out, total, p), nil
}
```

For `GetOutletMemberships`, the query already joins outlet columns — build the OutletResponse from the row's outlet fields instead of a second query (`row` should embed outlet id/name/etc.; alias them like the first query). Update the query in Task 2 accordingly.

```go
func (s *Service) InviteMember(ctx context.Context, ownerID, outletID uuid.UUID, email string) (*MembershipResponse, error) {
	inviter, err := s.assertOwner(ctx, outletID, ownerID)
	if err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	invitee, err := s.Store.Querier().GetUserByEmailCaseInsensitive(ctx, strings.TrimSpace(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("No active user found for email: " + strings.TrimSpace(email))
	}
	if err != nil {
		return nil, err
	}

	existing, err := s.Store.Querier().GetMembershipByOutletAndUserIncludingRemoved(ctx, db.GetMembershipByOutletAndUserIncludingRemovedParams{OutletID: outletID, UserID: invitee.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	var m *db.OutletMembership
	if existing != nil {
		if existing.RemovedAt == nil && existing.Status == "ACCEPTED" {
			return nil, httpapi.Conflict("User is already an active member of this outlet")
		}
		if existing.RemovedAt == nil && existing.Status == "INVITED" {
			return nil, httpapi.Conflict("User already has a pending invitation for this outlet")
		}
		m, err = s.Store.Querier().UpdateMembershipInvite(ctx, db.UpdateMembershipInviteParams{
			ID: existing.ID, Role: "EMPLOYEE", Status: "INVITED", InvitedByUserID: &inviter.ID,
		})
	} else {
		m, err = s.Store.Querier().CreateMembership(ctx, db.CreateMembershipParams{
			OutletID: outletID, UserID: invitee.ID, Role: "EMPLOYEE", Status: "INVITED",
			DisplayName: invitee.Name, InvitedByUserID: &inviter.ID,
		})
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpapi.Conflict("User already has a membership record for this outlet")
		}
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.invited")
	s.Audit.Record(ctx, inviter.ID.String(), "OUTLET_MEMBER_INVITED", "OUTLET_MEMBERSHIP", m.ID,
		map[string]any{"outletId": outletID, "inviteeUserId": invitee.ID}, "", "")
	details, _, _, inv, err := s.loadMembershipDetails(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	u, err := s.Store.Querier().GetUserByID(ctx, invitee.ID)
	if err != nil {
		return nil, err
	}
	out, err := s.getActiveOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	return s.toMembershipResponse(details, u, out, inv), nil
}

func (s *Service) AcceptInvite(ctx context.Context, userID, membershipID uuid.UUID) (*MembershipResponse, error) {
	m, outlet, u, inv, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if m.UserID != userID {
		return nil, httpapi.Forbidden("You can only manage your own outlet invitations")
	}
	if m.Status != "INVITED" {
		return nil, httpapi.BadRequest("Only pending invitations can be accepted")
	}
	updated, err := s.Store.Querier().UpdateMembershipStatus(ctx, db.UpdateMembershipStatusParams{ID: membershipID, Status: "ACCEPTED"})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.accepted")
	s.Audit.Record(ctx, userID.String(), "OUTLET_INVITE_ACCEPTED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": m.OutletID}, "", "")
	return s.toMembershipResponse(updated, u, outlet, inv), nil
}

func (s *Service) RejectInvite(ctx context.Context, userID, membershipID uuid.UUID) (*MembershipResponse, error) {
	m, outlet, u, inv, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if m.UserID != userID {
		return nil, httpapi.Forbidden("You can only manage your own outlet invitations")
	}
	if m.Status != "INVITED" {
		return nil, httpapi.BadRequest("Only pending invitations can be rejected")
	}
	updated, err := s.Store.Querier().UpdateMembershipStatus(ctx, db.UpdateMembershipStatusParams{ID: membershipID, Status: "REJECTED"})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.rejected")
	s.Audit.Record(ctx, userID.String(), "OUTLET_INVITE_REJECTED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": m.OutletID}, "", "")
	return s.toMembershipResponse(updated, u, outlet, inv), nil
}

func (s *Service) LeaveOutlet(ctx context.Context, userID, outletID uuid.UUID) error {
	m, err := s.getActiveMembership(ctx, outletID, userID)
	if err != nil {
		return err
	}
	if m.Role == "OWNER" {
		return httpapi.BadRequest("Owners cannot leave an outlet through this endpoint")
	}
	if _, err := s.Store.Querier().RemoveMembership(ctx, db.RemoveMembershipParams{ID: m.ID, RemovedByUserID: &userID}); err != nil {
		return err
	}
	s.Metrics.Increment("outlet.membership.left")
	s.Audit.Record(ctx, userID.String(), "OUTLET_MEMBERSHIP_LEFT", "OUTLET_MEMBERSHIP", m.ID,
		map[string]any{"outletId": outletID}, "", "")
	return nil
}

func (s *Service) RemoveMembership(ctx context.Context, ownerID, outletID, membershipID uuid.UUID) error {
	owner, err := s.assertOwner(ctx, outletID, ownerID)
	if err != nil {
		return err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return err
	}
	m, _, u, _, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return err
	}
	if m.OutletID != outletID {
		return httpapi.BadRequest("The provided membership does not belong to the requested outlet")
	}
	if m.Role == "OWNER" {
		return httpapi.BadRequest("Owner memberships cannot be removed through this endpoint")
	}
	if _, err := s.Store.Querier().RemoveMembership(ctx, db.RemoveMembershipParams{ID: membershipID, RemovedByUserID: &owner.ID}); err != nil {
		return err
	}
	s.Metrics.Increment("outlet.membership.removed")
	s.Audit.Record(ctx, ownerID.String(), "OUTLET_MEMBERSHIP_REMOVED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": outletID, "removedUserId": u.ID}, "", "")
	return nil
}

func (s *Service) UpdateDisplayName(ctx context.Context, ownerID, outletID, membershipID uuid.UUID, name string) (*MembershipResponse, error) {
	if _, err := s.assertOwner(ctx, outletID, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	m, outlet, u, inv, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if m.OutletID != outletID {
		return nil, httpapi.BadRequest("The provided membership does not belong to the requested outlet")
	}
	updated, err := s.Store.Querier().UpdateMembershipDisplayName(ctx, db.UpdateMembershipDisplayNameParams{ID: membershipID, DisplayName: strings.TrimSpace(name)})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.display_name.updated")
	s.Audit.Record(ctx, ownerID.String(), "OUTLET_MEMBERSHIP_DISPLAY_NAME_UPDATED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": outletID, "userId": u.ID, "displayName": strings.TrimSpace(name)}, "", "")
	return s.toMembershipResponse(updated, u, outlet, inv), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
```

Add imports `"github.com/jackc/pgx/v5/pgconn"`. Also add `httpapi.NewPageResponse`:

```go
// httpapi/response.go addition
func NewPageResponse[T any](content []T, total int64, p PageParams) *PageResponse[T] {
	page := p.Page
	size := p.Size
	totalPages := int64(0)
	if size > 0 {
		totalPages = (total + int64(size) - 1) / int64(size)
	}
	return &PageResponse[T]{
		Content: content, Page: page, Size: size,
		TotalElements: total, TotalPages: int(totalPages),
		First: page == 0,
		Last:  page >= int(totalPages)-1 || totalPages == 0,
		Empty: len(content) == 0,
	}
}

type PageResponse[T any] struct {
	Content       []T   `json:"content"`
	Page          int   `json:"page"`
	Size          int   `json:"size"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	First         bool  `json:"first"`
	Last          bool  `json:"last"`
	Empty         bool  `json:"empty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go build ./... && go test ./outlet/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/outlet go/httpapi
git commit -m "feat(go): membership service (invite, accept, reject, leave, remove, display name, listings)"
```

---

### Task 13: Outlet + membership handlers and routes

**Files:**
- Create: `go/outlet/handlers.go`

**Interfaces:**
- Produces:
  - `type Handlers struct { Svc *Service }`
  - Methods (each `func(w http.ResponseWriter, r *http.Request)`): `CreateOutlet`, `GetOutlet`, `UpdateOutlet`, `UpdateGeofence`, `GetMyOutlets`, `GetMyInvites`, `GetOutletMemberships`, `DeleteOutlet`, `LeaveOutlet`, `InviteMember`, `RemoveMembership`, `UpdateDisplayName`, `AcceptInvite`, `RejectInvite`.
- Consumes: `httpapi` (DecodeJSON, SubjectID, ClientIP, ParsePageParams, WriteJSON, WritePage), `outlet.Service`, `auth.Require` (wired in main, not imported).

- [ ] **Step 1: Write the implementation**

```go
package outlet

import (
	"net/http"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

type Handlers struct{ Svc *Service }

func (h *Handlers) currentUserID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(httpapi.SubjectID(r))
}

func (h *Handlers) CreateOutlet(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	var req CreateOutletRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.CreateOutlet(r.Context(), uid, req)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

func (h *Handlers) GetOutlet(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, err := uuid.Parse(r.PathValue("outletId"))
	if err != nil {
		httpapi.WriteError(w, httpapi.NotFound("Outlet not found: "+r.PathValue("outletId")))
		return
	}
	out, err := h.Svc.GetOutlet(r.Context(), uid, outletID)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handlers) UpdateOutlet(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	var req UpdateOutletRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.UpdateOutlet(r.Context(), uid, outletID, req)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handlers) UpdateGeofence(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	var req struct {
		GeofenceEnabled bool `json:"geofenceEnabled"`
	}
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.UpdateGeofence(r.Context(), uid, outletID, req.GeofenceEnabled)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

func (h *Handlers) GetMyOutlets(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	page, err := h.Svc.GetMyOutlets(r.Context(), uid, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *Handlers) GetMyInvites(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	page, err := h.Svc.GetMyInvites(r.Context(), uid, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *Handlers) GetOutletMemberships(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	page, err := h.Svc.GetOutletMemberships(r.Context(), uid, outletID, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *Handlers) DeleteOutlet(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	if err := h.Svc.DeleteOutlet(r.Context(), uid, outletID); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) LeaveOutlet(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	if err := h.Svc.LeaveOutlet(r.Context(), uid, outletID); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) InviteMember(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	var req struct {
		Email string `json:"email"`
	}
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.Email == "" {
		httpapi.WriteError(w, httpapi.Validation("must not be blank"))
		return
	}
	resp, err := h.Svc.InviteMember(r.Context(), uid, outletID, req.Email)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handlers) RemoveMembership(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	membershipID, _ := uuid.Parse(r.PathValue("membershipId"))
	if err := h.Svc.RemoveMembership(r.Context(), uid, outletID, membershipID); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) UpdateDisplayName(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, _ := uuid.Parse(r.PathValue("outletId"))
	membershipID, _ := uuid.Parse(r.PathValue("membershipId"))
	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	resp, err := h.Svc.UpdateDisplayName(r.Context(), uid, outletID, membershipID, req.DisplayName)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	membershipID, _ := uuid.Parse(r.PathValue("membershipId"))
	resp, err := h.Svc.AcceptInvite(r.Context(), uid, membershipID)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handlers) RejectInvite(w http.ResponseWriter, r *http.Request) {
	uid, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	membershipID, _ := uuid.Parse(r.PathValue("membershipId"))
	resp, err := h.Svc.RejectInvite(r.Context(), uid, membershipID)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Wire routes in main.go**

```go
outletSvc := outlet.NewService(store, recorder, registry)
outletHandlers := &outlet.Handlers{Svc: outletSvc}

apiMux.Handle("POST /api/v1/outlets", auth.Require(outletHandlers.CreateOutlet))
apiMux.Handle("GET /api/v1/outlets/mine", auth.Require(outletHandlers.GetMyOutlets))
apiMux.Handle("GET /api/v1/outlets/invites", auth.Require(outletHandlers.GetMyInvites))
apiMux.Handle("GET /api/v1/outlets/{outletId}", auth.Require(outletHandlers.GetOutlet))
apiMux.Handle("PUT /api/v1/outlets/{outletId}", auth.Require(outletHandlers.UpdateOutlet))
apiMux.Handle("PUT /api/v1/outlets/{outletId}/geofence", auth.Require(outletHandlers.UpdateGeofence))
apiMux.Handle("DELETE /api/v1/outlets/{outletId}", auth.Require(outletHandlers.DeleteOutlet))
apiMux.Handle("POST /api/v1/outlets/{outletId}/leave", auth.Require(outletHandlers.LeaveOutlet))
apiMux.Handle("GET /api/v1/outlets/{outletId}/memberships", auth.Require(outletHandlers.GetOutletMemberships))
apiMux.Handle("POST /api/v1/outlets/{outletId}/memberships/invite", auth.Require(outletHandlers.InviteMember))
apiMux.Handle("DELETE /api/v1/outlets/{outletId}/memberships/{membershipId}", auth.Require(outletHandlers.RemoveMembership))
apiMux.Handle("PUT /api/v1/outlets/{outletId}/memberships/{membershipId}/display-name", auth.Require(outletHandlers.UpdateDisplayName))
apiMux.Handle("POST /api/v1/memberships/{membershipId}/accept", auth.Require(outletHandlers.AcceptInvite))
apiMux.Handle("POST /api/v1/memberships/{membershipId}/reject", auth.Require(outletHandlers.RejectInvite))
```

- [ ] **Step 3: Build and commit**

```bash
cd go && go build ./... && cd ..
git add go/outlet/handlers.go go/cmd
git commit -m "feat(go): outlet and membership HTTP handlers"
```

---

## Phase 4: Attendance

### Task 14: Geofence (haversine) utility

**Files:**
- Create: `go/attendance/geofence.go`
- Create: `go/attendance/geofence_test.go`

**Interfaces:**
- Produces: `func IsWithinRadiusMeters(centerLat, centerLon, requestLat, requestLon float64, radiusMeters int) bool`
- Consumes: nothing (math only).

- [ ] **Step 1: Write the failing test**

```go
package attendance

import "testing"

func TestHaversine(t *testing.T) {
	// Approx 100m north of origin.
	if !IsWithinRadiusMeters(0, 0, 0.001, 0, 200) {
		t.Error("expected within radius")
	}
	if IsWithinRadiusMeters(0, 0, 0.1, 0, 100) {
		t.Error("expected outside radius")
	}
	// Same point.
	if !IsWithinRadiusMeters(12.34, 56.78, 12.34, 56.78, 1) {
		t.Error("same point should be within any radius")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./attendance/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package attendance

import "math"

const earthRadiusMeters = 6371000.0

func IsWithinRadiusMeters(centerLat, centerLon, requestLat, requestLon float64, radiusMeters int) bool {
	return distanceMeters(centerLat, centerLon, requestLat, requestLon) <= float64(radiusMeters)
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./attendance/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/attendance
git commit -m "feat(go): attendance geofence haversine utility"
```

---

### Task 15: Attendance service

**Files:**
- Create: `go/attendance/service.go`
- Create: `go/attendance/service_test.go`

**Interfaces:**
- Produces:
  - `type Service struct { Store *db.Store; Clock func() time.Time; Audit *audit.Recorder; Metrics *metrics.Registry }`
  - `func NewService(store *db.Store, clock func() time.Time, a *audit.Recorder, m *metrics.Registry) *Service` (default clock = `time.Now`)
  - `type EntryType string` with `const (EntryTypeClockIn EntryType = "CLOCK_IN"; EntryTypeClockOut EntryType = "CLOCK_OUT")`
  - `type CreateOwnRequest struct { Type EntryType; Latitude float64; Longitude float64 }` (JSON `type,latitude,longitude`)
  - `type ManageRequest struct { UserID uuid.UUID; Type EntryType; EntryTime time.Time; Latitude float64; Longitude float64 }` (JSON `userId,type,entryTime,latitude,longitude`)
  - `type UpdateRequest struct { Type EntryType; EntryTime time.Time; Latitude float64; Longitude float64 }` (JSON `type,entryTime,latitude,longitude`)
  - `type EntryResponse struct` (JSON tags `id,outletId,userId,userName,userEmail,displayName,type,entryTime,latitude,longitude,createdByUserId,updatedByUserId,createdAt,updatedAt`)
  - Methods: `CreateOwn`, `CreateManaged`, `List`, `Get`, `Update`, `Delete` (signatures per route; list takes `userID, outletID, targetUserID *uuid.UUID, p httpapi.PageParams`).
- Consumes: `db`, `httpapi`, `audit`, `metrics`, `attendance.IsWithinRadiusMeters`.

- [ ] **Step 1: Write the failing tests (pure rules)**

```go
package attendance

import (
	"errors"
	"testing"
	"time"

	"github.com/coderGtm/delta/go/httpapi"
)

func TestGeofenceRejection(t *testing.T) {
	s := &Service{}
	err := s.enforceGeofenceForTest(true, 0, 0, 0.1, 0, 100)
	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "FORBIDDEN" {
		t.Fatalf("err = %v", err)
	}
	if err := s.enforceGeofenceForTest(false, 0, 0, 0.1, 0, 100); err != nil {
		t.Fatalf("disabled geofence should pass, got %v", err)
	}
	if err := s.enforceGeofenceForTest(true, 0, 0, 0.0001, 0, 100); err != nil {
		t.Fatalf("inside radius should pass, got %v", err)
	}
}

func TestRoleGuards(t *testing.T) {
	s := &Service{}
	if err := s.requireEmployeeForTest("OWNER"); err == nil {
		t.Fatal("owner should not self-clock-in")
	}
	var apiErr *httpapi.APIError
	if !errors.As(s.requireEmployeeForTest("OWNER"), &apiErr) || apiErr.Code != "FORBIDDEN" {
		t.Fatalf("err = %v", err)
	}
	if err := s.requireEmployeeForTest("EMPLOYEE"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestSelfEntryUsesServerClock(t *testing.T) {
	fixed := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	s := &Service{Clock: func() time.Time { return fixed }}
	if got := s.Clock(); !got.Equal(fixed) {
		t.Fatalf("clock = %v", got)
	}
}
```

Test hooks:
- `enforceGeofenceForTest(enabled bool, clat, clon, rlat, rlon float64, radius int) error` — if !enabled return nil; if !IsWithinRadiusMeters(...) → `httpapi.Forbidden("Attendance location is outside the outlet geofence")`; nil.
- `requireEmployeeForTest(role string) error` — if role != "EMPLOYEE" → `httpapi.Forbidden("Only accepted employees can create their own attendance entries")`; nil.

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./attendance/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package attendance

import (
	"context"
	"errors"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type EntryType string

const (
	EntryTypeClockIn  EntryType = "CLOCK_IN"
	EntryTypeClockOut EntryType = "CLOCK_OUT"
)

type CreateOwnRequest struct {
	Type      EntryType `json:"type"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
}

type ManageRequest struct {
	UserID    uuid.UUID `json:"userId"`
	Type      EntryType `json:"type"`
	EntryTime time.Time `json:"entryTime"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
}

type UpdateRequest struct {
	Type      EntryType `json:"type"`
	EntryTime time.Time `json:"entryTime"`
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
}

type EntryResponse struct {
	ID              uuid.UUID `json:"id"`
	OutletID        uuid.UUID `json:"outletId"`
	UserID          uuid.UUID `json:"userId"`
	UserName        string    `json:"userName"`
	UserEmail       string    `json:"userEmail"`
	DisplayName     string    `json:"displayName"`
	Type            EntryType `json:"type"`
	EntryTime       time.Time `json:"entryTime"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	CreatedByUserID *uuid.UUID `json:"createdByUserId"`
	UpdatedByUserID *uuid.UUID `json:"updatedByUserId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type Service struct {
	Store   *db.Store
	Clock   func() time.Time
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

func NewService(store *db.Store, clock func() time.Time, a *audit.Recorder, m *metrics.Registry) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{Store: store, Clock: clock, Audit: a, Metrics: m}
}

func (s *Service) getActiveMembership(ctx context.Context, outletID, userID uuid.UUID) (*db.OutletMembership, *db.Outlet, error) {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{OutletID: outletID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return nil, nil, err
	}
	if m.Status != "ACCEPTED" {
		return nil, nil, httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	o, err := s.Store.Querier().GetOutletByID(ctx, outletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, httpapi.NotFound("Outlet not found: " + outletID.String())
	}
	if err != nil {
		return nil, nil, err
	}
	return m, o, nil
}

func (s *Service) enforceGeofence(o *db.Outlet, lat, lon float64) *httpapi.APIError {
	if !o.GeofenceEnabled {
		return nil
	}
	if !IsWithinRadiusMeters(float64(o.Latitude), float64(o.Longitude), lat, lon, int(o.RadiusMeters)) {
		s.Metrics.Increment("attendance.geofence.rejected", "outletId", o.ID.String())
		return httpapi.Forbidden("Attendance location is outside the outlet geofence")
	}
	return nil
}

func (s *Service) toEntryResponse(e *db.AttendanceEntry, user *db.User, displayName string) *EntryResponse {
	return &EntryResponse{
		ID: e.ID, OutletID: e.OutletID, UserID: e.UserID,
		UserName: user.Name, UserEmail: strOrNil(user.Email),
		DisplayName: displayName, Type: EntryType(e.Type),
		EntryTime: e.EntryTime, Latitude: float64(e.Latitude), Longitude: float64(e.Longitude),
		CreatedByUserID: e.CreatedBy, UpdatedByUserID: e.UpdatedBy,
		CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

// memberDisplayName resolves displayName from the membership row (even if
// removed), falling back to the user account name.
func (s *Service) memberDisplayName(ctx context.Context, outletID, userID uuid.UUID, fallback string) (string, error) {
	rows, err := s.Store.Querier().ListMembershipsForOutletByUser(ctx, db.ListMembershipsForOutletByUserParams{OutletID: outletID, UserID: userID})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return fallback, nil
	}
	return rows[0].DisplayName, nil
}

func (s *Service) CreateOwn(ctx context.Context, userID, outletID uuid.UUID, req CreateOwnRequest) (*EntryResponse, error) {
	m, o, err := s.getActiveMembership(ctx, outletID, userID)
	if err != nil {
		return nil, err
	}
	if m.Role != "EMPLOYEE" {
		return nil, httpapi.Forbidden("Only accepted employees can create their own attendance entries")
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	entry, err := s.Store.Querier().CreateAttendanceEntry(ctx, db.CreateAttendanceEntryParams{
		UserID: userID, OutletID: outletID, Type: string(req.Type), EntryTime: s.Clock().UTC(),
		Latitude: req.Latitude, Longitude: req.Longitude, CreatedBy: &userID,
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.created", "mode", "self")
	s.Audit.Record(ctx, userID.String(), "ATTENDANCE_CREATED", "ATTENDANCE_ENTRY", entry.ID,
		map[string]any{"outletId": outletID, "userId": userID, "type": entry.Type, "mode": "self"}, "", "")
	u, err := s.Store.Querier().GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(entry, u, m.DisplayName), nil
}

func (s *Service) CreateManaged(ctx context.Context, ownerID, outletID uuid.UUID, req ManageRequest) (*EntryResponse, error) {
	m, o, err := s.getActiveMembership(ctx, outletID, ownerID)
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" {
		return nil, httpapi.Forbidden("Only outlet owners can perform this action")
	}
	target, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{OutletID: outletID, UserID: req.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the requested user")
	}
	if err != nil {
		return nil, err
	}
	if target.Status != "ACCEPTED" || target.Role != "EMPLOYEE" {
		return nil, httpapi.BadRequest("Attendance can only be created for accepted employee memberships")
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	entry, err := s.Store.Querier().CreateAttendanceEntry(ctx, db.CreateAttendanceEntryParams{
		UserID: req.UserID, OutletID: outletID, Type: string(req.Type), EntryTime: req.EntryTime.UTC(),
		Latitude: req.Latitude, Longitude: req.Longitude, CreatedBy: &ownerID,
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.created", "mode", "managed")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_CREATED", "ATTENDANCE_ENTRY", entry.ID,
		map[string]any{"outletId": outletID, "userId": req.UserID, "type": entry.Type, "mode": "managed"}, "", "")
	u, err := s.Store.Querier().GetUserByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(entry, u, target.DisplayName), nil
}

func (s *Service) List(ctx context.Context, callerID, outletID uuid.UUID, targetUserID *uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[EntryResponse], error) {
	m, _, err := s.getActiveMembership(ctx, outletID, callerID)
	if err != nil {
		return nil, err
	}
	var rows []db.AttendanceEntry
	var total int64
	if m.Role == "OWNER" {
		if targetUserID == nil {
			rows, err = s.Store.Querier().ListAttendanceByOutlet(ctx, db.ListAttendanceByOutletParams{OutletID: outletID, Limit: int32(p.Size), Offset: int32(p.Page * p.Size)})
			if err == nil {
				total, err = s.Store.Querier().CountAttendanceByOutlet(ctx, outletID)
			}
		} else {
			rows, err = s.Store.Querier().ListAttendanceByOutletAndUser(ctx, db.ListAttendanceByOutletAndUserParams{OutletID: outletID, UserID: *targetUserID, Limit: int32(p.Size), Offset: int32(p.Page * p.Size)})
			if err == nil {
				total, err = s.Store.Querier().CountAttendanceByOutletAndUser(ctx, db.CountAttendanceByOutletAndUserParams{OutletID: outletID, UserID: *targetUserID})
			}
		}
	} else {
		if targetUserID != nil && *targetUserID != callerID {
			return nil, httpapi.Forbidden("Employees can only view their own attendance entries")
		}
		rows, err = s.Store.Querier().ListAttendanceByOutletAndUser(ctx, db.ListAttendanceByOutletAndUserParams{OutletID: outletID, UserID: callerID, Limit: int32(p.Size), Offset: int32(p.Page * p.Size)})
		if err == nil {
			total, err = s.Store.Querier().CountAttendanceByOutletAndUser(ctx, db.CountAttendanceByOutletAndUserParams{OutletID: outletID, UserID: callerID})
		}
	}
	if err != nil {
		return nil, err
	}
	out := make([]EntryResponse, 0, len(rows))
	for _, e := range rows {
		u, err := s.Store.Querier().GetUserByID(ctx, e.UserID)
		if err != nil {
			return nil, err
		}
		dn, err := s.memberDisplayName(ctx, outletID, e.UserID, u.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, *s.toEntryResponse(&e, u, dn))
	}
	return httpapi.NewPageResponse(out, total, p), nil
}

func (s *Service) Get(ctx context.Context, callerID, outletID, entryID uuid.UUID) (*EntryResponse, error) {
	m, _, err := s.getActiveMembership(ctx, outletID, callerID)
	if err != nil {
		return nil, err
	}
	e, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{ID: entryID, OutletID: outletID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Attendance entry not found: " + entryID.String())
	}
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" && e.UserID != callerID {
		return nil, httpapi.Forbidden("Employees can only view their own attendance entries")
	}
	u, err := s.Store.Querier().GetUserByID(ctx, e.UserID)
	if err != nil {
		return nil, err
	}
	dn, err := s.memberDisplayName(ctx, outletID, e.UserID, u.Name)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(e, u, dn), nil
}

func (s *Service) Update(ctx context.Context, ownerID, outletID, entryID uuid.UUID, req UpdateRequest) (*EntryResponse, error) {
	m, o, err := s.getActiveMembership(ctx, outletID, ownerID)
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" {
		return nil, httpapi.Forbidden("Only outlet owners can perform this action")
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	e, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{ID: entryID, OutletID: outletID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Attendance entry not found: " + entryID.String())
	}
	if err != nil {
		return nil, err
	}
	updated, err := s.Store.Querier().UpdateAttendanceEntry(ctx, db.UpdateAttendanceEntryParams{
		ID: entryID, Type: string(req.Type), EntryTime: req.EntryTime.UTC(), Latitude: req.Latitude, Longitude: req.Longitude, UpdatedBy: &ownerID,
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.updated")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_UPDATED", "ATTENDANCE_ENTRY", entryID,
		map[string]any{"outletId": outletID, "userId": updated.UserID, "type": updated.Type}, "", "")
	u, err := s.Store.Querier().GetUserByID(ctx, updated.UserID)
	if err != nil {
		return nil, err
	}
	dn, err := s.memberDisplayName(ctx, outletID, updated.UserID, u.Name)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(updated, u, dn), nil
}

func (s *Service) Delete(ctx context.Context, ownerID, outletID, entryID uuid.UUID) error {
	m, _, err := s.getActiveMembership(ctx, outletID, ownerID)
	if err != nil {
		return err
	}
	if m.Role != "OWNER" {
		return httpapi.Forbidden("Only outlet owners can perform this action")
	}
	e, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{ID: entryID, OutletID: outletID})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.NotFound("Attendance entry not found: " + entryID.String())
	}
	if err != nil {
		return err
	}
	if err := s.Store.Querier().DeleteAttendanceEntry(ctx, entryID); err != nil {
		return err
	}
	s.Metrics.Increment("attendance.deleted")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_DELETED", "ATTENDANCE_ENTRY", entryID,
		map[string]any{"outletId": outletID, "userId": e.UserID, "type": e.Type}, "", "")
	return nil
}

func strOrNil(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

Note: `db.Querier()` in this service is `s.Store.Querier()`. The `getActiveMembership` returns the outlet too, satisfying the "writes rejected for deleted outlets" rule (GetOutletByID filters removed).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go build ./... && go test ./attendance/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/attendance
git commit -m "feat(go): attendance service with geofence enforcement"
```

---

### Task 16: Attendance handlers and routes

**Files:**
- Create: `go/attendance/handlers.go`

**Interfaces:**
- Produces: `type Handlers struct { Svc *Service }` with methods `CreateOwn`, `CreateManaged`, `List`, `Get`, `Update`, `Delete` (`func(w http.ResponseWriter, r *http.Request)`).
- Consumes: `httpapi`, `attendance.Service`.

- [ ] **Step 1: Write the implementation (mirror Task 13 handler patterns)**

Handlers parse `outletId`/`attendanceEntryId` from `r.PathValue`, `userId` query param (owner filter), decode JSON bodies with `httpapi.DecodeJSON`, call the service, and write results (201 for creates, 204 for delete). Employee list-with-other-user-id is rejected inside the service.

Validation parity: `CreateOwn`/`ManageRequest`/`UpdateRequest` require valid `type` (CLOCK_IN|CLOCK_OUT); invalid → `httpapi.Validation("must be one of: CLOCK_IN, CLOCK_OUT")`. `ManageRequest` requires `userId`, `entryTime`, `latitude`, `longitude` present — match Java messages ("User ID is required" is for reports; attendance manage uses DTO validation — use `httpapi.Validation("must not be null")` for missing fields). Confirm exact strings against Java DTOs before finalizing.

- [ ] **Step 2: Wire routes in main.go**

```go
attSvc := attendance.NewService(store, nil, recorder, registry)
attHandlers := &attendance.Handlers{Svc: attSvc}

apiMux.Handle("POST /api/v1/outlets/{outletId}/attendance", auth.Require(attHandlers.CreateOwn))
apiMux.Handle("POST /api/v1/outlets/{outletId}/attendance/manage", auth.Require(attHandlers.CreateManaged))
apiMux.Handle("GET /api/v1/outlets/{outletId}/attendance", auth.Require(attHandlers.List))
apiMux.Handle("GET /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(attHandlers.Get))
apiMux.Handle("PUT /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(attHandlers.Update))
apiMux.Handle("DELETE /api/v1/outlets/{outletId}/attendance/{attendanceEntryId}", auth.Require(attHandlers.Delete))
```

- [ ] **Step 3: Build and commit**

```bash
cd go && go build ./... && cd ..
git add go/attendance/handlers.go go/cmd
git commit -m "feat(go): attendance HTTP handlers"
```

---

## Phase 5: Reports

### Task 17: Salary report service (pairing + math)

**Files:**
- Create: `go/report/service.go`
- Create: `go/report/service_test.go`

**Interfaces:**
- Produces:
  - `type Service struct { Store *db.Store; Audit *audit.Recorder; Metrics *metrics.Registry }`
  - `func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service`
  - `func (s *Service) Calculate(ctx context.Context, ownerID, outletID, employeeID uuid.UUID, start, end time.Time, timezone string, hourlyRate float64) (*SalaryReport, error)`
  - `func (s *Service) BuildExcel(report *SalaryReport) ([]byte, error)` (delegates to `excel.go`)
  - DTOs: `Pair{ClockIn, ClockOut time.Time; Hours float64}`, `Day{Date time.Time; Pairs []Pair; TotalHours, HourlyRate, Salary float64}`, `SalaryReport{OutletID uuid.UUID; OutletName string; UserID uuid.UUID; UserName, UserEmail, DisplayName string; StartTime, EndTime time.Time; Timezone string; HourlyRate, TotalHours, TotalSalary float64; Days []Day}` with JSON tags `outletId,outletName,userId,userName,userEmail,displayName,startTime,endTime,timezone,hourlyRate,totalHours,totalSalary,days`; `Day` JSON tags `date,attendancePairs,totalHours,hourlyRate,salary`; `Pair` JSON tags `clockIn,clockOut,hours`.
  - `func ValidateReportRequest(start, end time.Time, timezone string, hourlyRate float64) (*time.Location, error)` — messages: "Start time and end time are required", "End time must be after start time", "Timezone must be a valid IANA timezone", "Salary reports can cover at most 366 local days", "Hourly rate must be greater than zero".
  - Pure helpers for tests: `func CompletedPairs(entries []db.AttendanceEntry) []Pair`, `func round2(f float64) float64` (HALF_UP, 2 decimals), `func sanitizeExcelValue(v string) string`.
- Consumes: `db` (ListAttendanceByOutletUserRange), `httpapi`, `audit`, `metrics`.

- [ ] **Step 1: Write the failing tests (pure math)**

```go
package report

import (
	"testing"
	"time"

	"github.com/coderGtm/delta/go/db"
)

func TestCompletedPairs(t *testing.T) {
	base := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	in1 := db.AttendanceEntry{Type: "CLOCK_IN", EntryTime: base}
	out1 := db.AttendanceEntry{Type: "CLOCK_OUT", EntryTime: base.Add(2 * time.Hour)}
	in2 := db.AttendanceEntry{Type: "CLOCK_IN", EntryTime: base.Add(3 * time.Hour)}
	out2 := db.AttendanceEntry{Type: "CLOCK_OUT", EntryTime: base.Add(4 * time.Hour)}
	orphanOut := db.AttendanceEntry{Type: "CLOCK_OUT", EntryTime: base.Add(5 * time.Hour)}
	pairs := CompletedPairs([]db.AttendanceEntry{in1, out1, in2, orphanOut, out2})
	if len(pairs) != 2 {
		t.Fatalf("pairs = %d, want 2", len(pairs))
	}
	if pairs[0].Hours != 2.0 || pairs[1].Hours != 1.0 {
		t.Fatalf("hours = %+v", pairs)
	}
}

func TestCompletedPairsIgnoresOutOfOrderClockOut(t *testing.T) {
	base := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	in := db.AttendanceEntry{Type: "CLOCK_IN", EntryTime: base}
	earlyOut := db.AttendanceEntry{Type: "CLOCK_OUT", EntryTime: base.Add(-time.Hour)}
	if pairs := CompletedPairs([]db.AttendanceEntry{in, earlyOut}); len(pairs) != 0 {
		t.Fatalf("expected no pairs, got %+v", pairs)
	}
}

func TestRound2HalfUp(t *testing.T) {
	if round2(1.005) != 1.01 {
		t.Fatalf("round2(1.005) = %v", round2(1.005))
	}
	if round2(2.0) != 2.0 {
		t.Fatalf("round2(2.0) = %v", round2(2.0))
	}
}

func TestSanitizeExcelValue(t *testing.T) {
	if got := sanitizeExcelValue("=cmd()"); got != "'=cmd()" {
		t.Errorf("got %q", got)
	}
	if got := sanitizeExcelValue("plain"); got != "plain" {
		t.Errorf("got %q", got)
	}
	if got := sanitizeExcelValue(""); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestValidateReportRequest(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	if _, err := ValidateReportRequest(start, end, "Asia/Kolkata", 10); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if _, err := ValidateReportRequest(end, start, "Asia/Kolkata", 10); err == nil {
		t.Fatal("expected end-before-start error")
	}
	if _, err := ValidateReportRequest(start, end, "Not/AZone", 10); err == nil {
		t.Fatal("expected invalid tz error")
	}
	if _, err := ValidateReportRequest(start, end, "Asia/Kolkata", 0); err == nil {
		t.Fatal("expected zero-rate error")
	}
	farEnd := start.Add(400 * 24 * time.Hour)
	if _, err := ValidateReportRequest(start, farEnd, "Asia/Kolkata", 10); err == nil {
		t.Fatal("expected 366-day cap error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./report/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package report

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const maxReportDays = 366

type Pair struct {
	ClockIn  time.Time `json:"clockIn"`
	ClockOut time.Time `json:"clockOut"`
	Hours    float64   `json:"hours"`
}

type Day struct {
	Date     time.Time `json:"date"`
	Pairs    []Pair    `json:"attendancePairs"`
	TotalHours float64 `json:"totalHours"`
	HourlyRate float64 `json:"hourlyRate"`
	Salary    float64  `json:"salary"`
}

type SalaryReport struct {
	OutletID   uuid.UUID `json:"outletId"`
	OutletName string    `json:"outletName"`
	UserID     uuid.UUID `json:"userId"`
	UserName   string    `json:"userName"`
	UserEmail  string    `json:"userEmail"`
	DisplayName string   `json:"displayName"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Timezone   string    `json:"timezone"`
	HourlyRate float64   `json:"hourlyRate"`
	TotalHours float64   `json:"totalHours"`
	TotalSalary float64  `json:"totalSalary"`
	Days       []Day     `json:"days"`
}

type Service struct {
	Store   *db.Store
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service {
	return &Service{Store: store, Audit: a, Metrics: m}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func hoursBetween(in, out time.Time) float64 {
	secs := out.Sub(in).Seconds()
	return round2(secs / 3600)
}

// CompletedPairs pairs each CLOCK_IN with the next strictly-later CLOCK_OUT.
func CompletedPairs(entries []db.AttendanceEntry) []Pair {
	var pairs []Pair
	var pending *time.Time
	for i := range entries {
		e := &entries[i]
		if e.Type == "CLOCK_IN" {
			t := e.EntryTime
			pending = &t
			continue
		}
		if e.Type == "CLOCK_OUT" && pending != nil && e.EntryTime.After(*pending) {
			pairs = append(pairs, Pair{ClockIn: *pending, ClockOut: e.EntryTime, Hours: hoursBetween(*pending, e.EntryTime)})
			pending = nil
		}
	}
	return pairs
}

func ValidateReportRequest(start, end time.Time, timezone string, hourlyRate float64) (*time.Location, error) {
	if start.IsZero() || end.IsZero() {
		return nil, httpapi.BadRequest("Start time and end time are required")
	}
	if !end.After(start) {
		return nil, httpapi.BadRequest("End time must be after start time")
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, httpapi.BadRequest("Timezone must be a valid IANA timezone")
	}
	startDate := start.In(loc)
	endDate := end.Add(-time.Nanosecond).In(loc)
	days := int(endDate.Sub(startDate).Hours()/24) + 1
	if days > maxReportDays {
		return nil, httpapi.BadRequest("Salary reports can cover at most 366 local days")
	}
	if hourlyRate <= 0 {
		return nil, httpapi.BadRequest("Hourly rate must be greater than zero")
	}
	return loc, nil
}

func (s *Service) assertOwner(ctx context.Context, outletID, ownerID uuid.UUID) (*db.User, error) {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{OutletID: outletID, UserID: ownerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return nil, err
	}
	if m.Status != "ACCEPTED" {
		return nil, httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	if m.Role != "OWNER" {
		return nil, httpapi.Forbidden("Only outlet owners can perform this action")
	}
	return s.Store.Querier().GetUserByID(ctx, ownerID)
}

func (s *Service) Calculate(ctx context.Context, ownerID, outletID, employeeID uuid.UUID, start, end time.Time, timezone string, hourlyRate float64) (*SalaryReport, error) {
	loc, err := ValidateReportRequest(start, end, timezone, hourlyRate)
	if err != nil {
		return nil, err
	}
	owner, err := s.assertOwner(ctx, outletID, ownerID)
	if err != nil {
		return nil, err
	}
	_ = owner
	outlet, err := s.Store.Querier().GetOutletByID(ctx, outletID)
	if err != nil {
		return nil, err
	}
	employeeMembership, err := s.Store.Querier().GetMembershipByOutletAndUserIncludingRemoved(ctx, db.GetMembershipByOutletAndUserIncludingRemovedParams{OutletID: outletID, UserID: employeeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the requested employee")
	}
	if err != nil {
		return nil, err
	}
	employee, err := s.Store.Querier().GetUserByID(ctx, employeeID)
	if err != nil {
		return nil, err
	}
	entries, err := s.Store.Querier().ListAttendanceByOutletUserRange(ctx, db.ListAttendanceByOutletUserRangeParams{
		OutletID: outletID, UserID: employeeID, EntryTime: start, EntryTime_2: end,
	})
	if err != nil {
		return nil, err
	}

	displayName := employeeMembership.DisplayName
	startDate := start.In(loc)
	endDate := end.Add(-time.Nanosecond).In(loc)

	byDate := map[time.Time][]db.AttendanceEntry{}
	for _, e := range entries {
		d := e.EntryTime.In(loc)
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		byDate[dayStart] = append(byDate[dayStart], e)
	}

	var days []Day
	var totalHours, totalSalary float64
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
		pairs := CompletedPairs(byDate[dayStart])
		var dayHours float64
		for _, p := range pairs {
			dayHours += p.Hours
		}
		dayHours = round2(dayHours)
		daySalary := round2(dayHours * hourlyRate)
		days = append(days, Day{Date: dayStart, Pairs: pairs, TotalHours: dayHours, HourlyRate: hourlyRate, Salary: daySalary})
		totalHours += dayHours
		totalSalary += daySalary
	}

	report := &SalaryReport{
		OutletID: outlet.ID, OutletName: outlet.Name,
		UserID: employee.ID, UserName: employee.Name, UserEmail: strOrNil(employee.Email),
		DisplayName: displayName,
		StartTime: start, EndTime: end, Timezone: loc.String(),
		HourlyRate: hourlyRate, TotalHours: round2(totalHours), TotalSalary: round2(totalSalary),
		Days: days,
	}
	s.Metrics.Increment("report.salary.generated", "format", "json")
	s.Audit.Record(ctx, ownerID.String(), "SALARY_REPORT_GENERATED", "OUTLET", outletID,
		map[string]any{"employeeUserId": employeeID, "startTime": start.String(), "endTime": end.String(), "timezone": loc.String(), "format": "json"}, "", "")
	return report, nil
}

func sanitizeExcelValue(v string) string {
	if v == "" {
		return v
	}
	first := v[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first <= 0x20 || first == 0x7f {
		return "'" + v
	}
	return v
}

func strOrNil(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

Note: `endDate` local-day difference should be computed via calendar arithmetic (`for d := startDate; !d.After(endDate); d = d.AddDate(...)`) rather than the hour division; the `days` check above uses calendar iteration too. Replace the `days` computation with a calendar loop count for correctness (DST-safe). In `ValidateReportRequest`, iterate:

```go
days := 0
for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
	days++
	if days > maxReportDays {
		return nil, httpapi.BadRequest("Salary reports can cover at most 366 local days")
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go build ./... && go test ./report/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/report
git commit -m "feat(go): salary report calculation"
```

---

### Task 18: Excel export

**Files:**
- Create: `go/report/excel.go`
- Create: `go/report/excel_test.go`

**Interfaces:**
- Produces: `func (s *Service) BuildExcel(report *SalaryReport) ([]byte, error)`
- Consumes: `excelize/v2`, `report.SalaryReport`.

- [ ] **Step 1: Write the failing test**

```go
package report

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildExcel(t *testing.T) {
	base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	rep := &SalaryReport{
		OutletID: uuid.New(), OutletName: "Shop",
		UserID: uuid.New(), UserName: "Bob", UserEmail: "b@x.com", DisplayName: "Bobby",
		StartTime: base, EndTime: base.Add(24 * time.Hour), Timezone: "UTC",
		HourlyRate: 10, TotalHours: 2, TotalSalary: 20,
		Days: []Day{{Date: base, Pairs: []Pair{{ClockIn: base.Add(9 * time.Hour), ClockOut: base.Add(11 * time.Hour), Hours: 2}}, TotalHours: 2, HourlyRate: 10, Salary: 20}},
	}
	s := &Service{}
	data, err := s.BuildExcel(rep)
	if err != nil {
		t.Fatalf("BuildExcel: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty workbook")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./report/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package report

import (
	"fmt"
	"time"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/xuri/excelize/v2"
)

// BuildExcel reproduces the Java workbook layout: title row, metadata row,
// header row (Date, Clock In n, Clock Out n, ..., Total Hours, Hourly Rate,
// Salary), one row per day, then a TOTAL row. Cell values are sanitized
// against formula injection (CWE-1236).
func (s *Service) BuildExcel(report *SalaryReport) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	sheet := "Salary Report"
	f.SetSheetName("Sheet1", sheet)

	loc, err := time.LoadLocation(report.Timezone)
	if err != nil {
		return nil, httpapi.BadRequest("Timezone must be a valid IANA timezone")
	}

	maxPairs := 0
	for _, d := range report.Days {
		if len(d.Pairs) > maxPairs {
			maxPairs = len(d.Pairs)
		}
	}

	row := 1
	f.SetCellValue(sheet, cell(1, row), "Salary Report") // A1
	row++
	// Metadata row: Outlet, <name>, Employee, "displayName <email>", Period, ..., Timezone
	f.SetCellValue(sheet, cell(1, row), "Outlet")
	f.SetCellValue(sheet, cell(2, row), report.OutletName)
	f.SetCellValue(sheet, cell(3, row), "Employee")
	f.SetCellValue(sheet, cell(4, row), fmt.Sprintf("%s <%s>", report.DisplayName, report.UserEmail))
	f.SetCellValue(sheet, cell(5, row), "Period")
	f.SetCellValue(sheet, cell(6, row), report.StartTime.In(loc).String()+" to "+report.EndTime.In(loc).String())
	f.SetCellValue(sheet, cell(7, row), "Timezone")
	f.SetCellValue(sheet, cell(8, row), report.Timezone)
	row += 2

	col := 1
	f.SetCellValue(sheet, cell(col, row), "Date")
	col++
	for i := 1; i <= maxPairs; i++ {
		f.SetCellValue(sheet, cell(col, row), fmt.Sprintf("Clock In %d", i))
		col++
		f.SetCellValue(sheet, cell(col, row), fmt.Sprintf("Clock Out %d", i))
		col++
	}
	f.SetCellValue(sheet, cell(col, row), "Total Hours")
	col++
	f.SetCellValue(sheet, cell(col, row), "Hourly Rate")
	col++
	f.SetCellValue(sheet, cell(col, row), "Salary")
	row++

	for _, d := range report.Days {
		col = 1
		f.SetCellValue(sheet, cell(col, row), sanitizeExcelValue(d.Date.Format("2006-01-02")))
		col++
		for _, p := range d.Pairs {
			f.SetCellValue(sheet, cell(col, row), p.ClockIn.In(loc).Format("15:04:05"))
			col++
			f.SetCellValue(sheet, cell(col, row), p.ClockOut.In(loc).Format("15:04:05"))
			col++
		}
		for col < 1+(maxPairs*2) {
			f.SetCellValue(sheet, cell(col, row), "")
			col++
		}
		f.SetCellValue(sheet, cell(col, row), d.TotalHours)
		col++
		f.SetCellValue(sheet, cell(col, row), d.HourlyRate)
		col++
		f.SetCellValue(sheet, cell(col, row), d.Salary)
		row++
	}

	col = 1
	f.SetCellValue(sheet, cell(col, row), "TOTAL")
	col++
	for col < 1+(maxPairs*2) {
		f.SetCellValue(sheet, cell(col, row), "")
		col++
	}
	f.SetCellValue(sheet, cell(col, row), report.TotalHours)
	col++
	f.SetCellValue(sheet, cell(col, row), report.HourlyRate)
	col++
	f.SetCellValue(sheet, cell(col, row), report.TotalSalary)

	return f.WriteToBuffer()
}

func cell(col, row int) string {
	return fmt.Sprintf("%s%d", columnName(col), row)
}

func columnName(col int) string {
	s := ""
	for col > 0 {
		col--
		s = string(rune('A'+col%26)) + s
		col /= 26
	}
	return s
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./report/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go/report
git commit -m "feat(go): salary report Excel export"
```

---

### Task 19: Report handlers and routes

**Files:**
- Create: `go/report/handlers.go`

**Interfaces:**
- Produces: `type Handlers struct { Svc *Service }` with `Salary` and `SalaryXLSX` (`func(w http.ResponseWriter, r *http.Request)`).
- Consumes: `httpapi`, `report.Service`.

- [ ] **Step 1: Write the implementation**

Both handlers parse query params with Spring/ISO parsing:
- `userId` → uuid (`httpapi.Validation("User ID is required")` if missing/invalid).
- `startTime`/`endTime` → `time.Parse(time.RFC3339Nano, ...)` (`httpapi.Validation("Start time is required")`/`"End time is required"` if missing).
- `timezone` → string (`httpapi.Validation("Timezone is required")` if blank).
- `hourlyRate` → float64 (`httpapi.Validation("Hourly rate must be greater than zero")` if missing or ≤ 0).
- `Salary` writes JSON. `SalaryXLSX` sets `Content-Type:
  application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` and
  `Content-Disposition: attachment; filename="salary-report-<userId>-<start>-<end>.xlsx"`
  with `:` replaced by `-` in the instants, then writes the workbook bytes.
- Both call `Svc.Calculate` first (the Excel one increments `report.salary.generated{format=xlsx}` + audit `SALARY_REPORT_EXCEL_GENERATED` — add those in `Calculate` via a `format` argument: change `Calculate(ctx, ..., format string)`; update Task 17 to accept `"json"`/`"xlsx"`).

- [ ] **Step 2: Wire routes in main.go**

```go
reportSvc := report.NewService(store, recorder, registry)
reportHandlers := &report.Handlers{Svc: reportSvc}

apiMux.Handle("GET /api/v1/outlets/{outletId}/reports/salary", auth.Require(reportHandlers.Salary))
apiMux.Handle("GET /api/v1/outlets/{outletId}/reports/salary.xlsx", auth.Require(reportHandlers.SalaryXLSX))
```

- [ ] **Step 3: Build and commit**

```bash
cd go && go build ./... && cd ..
git add go/report/handlers.go go/cmd
git commit -m "feat(go): salary report HTTP handlers"
```

---

## Phase 6: Cross-cutting, contract tests, docs

### Task 20: Rate limiting middleware

**Files:**
- Create: `go/httpapi/ratelimit.go`
- Create: `go/httpapi/ratelimit_test.go`

**Interfaces:**
- Produces:
  - `type RateLimiter struct { mu sync.Mutex; windows map[string]*window; policies []policy; trustProxy bool }`
  - `func NewRateLimiter(trustProxy bool) *RateLimiter`
  - `func (r *RateLimiter) Middleware(next http.Handler) http.Handler` — on exceed: 429, `Retry-After`, envelope `RATE_LIMIT_EXCEEDED` "Too many requests. Please retry later.".
  - Policy table from the spec §Rate limiting (copy verbatim).
- Consumes: `httpapi.ClientIP`, `httpapi.SubjectID`, `httpapi.RateLimitExceeded`.

- [ ] **Step 1: Write the failing test**

```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimitExceeds(t *testing.T) {
	rl := NewRateLimiter(false)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	// login allows 10/min per IP
	statuses := map[int]int{}
	for i := 0; i < 12; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
		req.RemoteAddr = "203.0.113.7:9999"
		handler.ServeHTTP(rec, req)
		statuses[rec.Code]++
	}
	if statuses[429] < 1 {
		t.Fatalf("expected some 429s, got %v", statuses)
	}
}

func TestRateLimitDistinctKeys(t *testing.T) {
	rl := NewRateLimiter(false)
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for i := 0; i < 12; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
		req.RemoteAddr = "203.0.113.9:9999"
		handler.ServeHTTP(rec, req)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
	req.RemoteAddr = "198.51.100.1:9999"
	handler.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("distinct IP should not be limited, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./httpapi/`
Expected: FAIL.

- [ ] **Step 3: Write the implementation**

```go
package httpapi

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type policy struct {
	method, path string
	limit        int
	window       time.Duration
	byUser       bool
}

type window struct {
	endsAt time.Time
	count  int
}

type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*window
	policies []policy
	trust    bool
}

var rateLimitPolicies = []policy{
	{"POST", "/api/v1/auth/login", 10, time.Minute, false},
	{"POST", "/api/v1/auth/refresh", 30, time.Minute, false},
	{"POST", "/api/v1/auth/logout", 30, time.Minute, false},
	{"POST", "/api/v1/auth/logout-all", 30, time.Minute, true},
	{"POST", "/api/v1/outlets/*/memberships/invite", 20, time.Minute, true},
	{"POST", "/api/v1/outlets/*/attendance", 20, time.Minute, true},
	{"POST", "/api/v1/outlets/*/attendance/manage", 60, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/attendance/*", 60, time.Minute, true},
	{"PUT", "/api/v1/outlets/*/geofence", 20, time.Minute, true},
	{"GET", "/api/v1/outlets/*/reports/salary", 30, time.Minute, true},
	{"GET", "/api/v1/outlets/*/reports/salary.xlsx", 10, time.Minute, true},
}

func NewRateLimiter(trustProxy bool) *RateLimiter {
	return &RateLimiter{windows: map[string]*window{}, policies: rateLimitPolicies, trust: trustProxy}
}

// matchPath implements single-segment '*' wildcards (Ant-style) for the
// policy table.
func matchPath(pattern, path string) bool {
	pParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pparts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pParts) != len(pparts) {
		return false
	}
	for i := range pParts {
		if pParts[i] == "*" {
			continue
		}
		if pParts[i] != pparts[i] {
			return false
		}
	}
	return true
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var p *policy
		for i := range r.policies {
			if req.Method == r.policies[i].method && matchPath(r.policies[i].path, req.URL.Path) {
				p = &r.policies[i]
				break
			}
		}
		if p == nil {
			next.ServeHTTP(w, req)
			return
		}
		key := ClientIP(req, r.trust)
		if p.byUser {
			if uid := SubjectID(req); uid != "" {
				key = uid
			}
		}
		now := time.Now()
		r.mu.Lock()
		wnd := r.windows[p.method+p.path+key]
		if wnd == nil || now.After(wnd.endsAt) {
			wnd = &window{endsAt: now.Add(p.window), count: 1}
			r.windows[p.method+p.path+key] = wnd
			r.mu.Unlock()
			next.ServeHTTP(w, req)
			return
		}
		wnd.count++
		over := wnd.count > p.limit
		retry := int(time.Until(wnd.endsAt).Seconds()) + 1
		r.mu.Unlock()

		if over {
			w.Header().Set("Retry-After", strconv.Itoa(retry))
			WriteJSON(w, http.StatusTooManyRequests, ErrorResponse{"RATE_LIMIT_EXCEEDED", "Too many requests. Please retry later.", now})
			return
		}
		next.ServeHTTP(w, req)
	})
}
```

Add `"strconv"` to imports.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go && go test ./httpapi/`
Expected: PASS.

- [ ] **Step 5: Wire into the middleware chain in main.go**

`handler := httpapi.NewRouter(...)` → add `rateLimiter.Middleware` inside the chain (after auth attach, before request log): update `NewRouter` signature to accept `rate http.Handler` or a `middlewares ...func(http.Handler) http.Handler` slice. Simplest: change `NewRouter` to accept `attach func(http.Handler) http.Handler` and `rate func(http.Handler) http.Handler`, chaining `rate(attach(mux))`.

- [ ] **Step 6: Commit**

```bash
git add go/httpapi go/cmd
git commit -m "feat(go): rate limiting middleware"
```

---

### Task 21: `openapi.yaml` + `/docs` Swagger UI

**Files:**
- Create: `go/openapi.yaml`
- Create: `go/httpapi/docs.go`
- Create: `go/web/swagger-ui` (embedded vendored swagger-ui dist, see step 2)
- Modify: `go/httpapi/router.go`

**Interfaces:**
- Produces: `GET /docs` → HTML serving the embedded Swagger UI pointed at `/docs/openapi.yaml`; `GET /docs/openapi.yaml` → serves `go/openapi.yaml`.
- Consumes: nothing new.

- [ ] **Step 1: Author `openapi.yaml`**

Document every `/api/v1/*` endpoint from the spec §API contract: paths, methods, params (incl. Spring-style pagination), request/response schemas (exact field names), `components.schemas` for `LoginResponse`, `RefreshTokenResponse`, `OutletResponse`, `OutletMembershipResponse`, `AttendanceEntryResponse`, `SalaryReportResponse`, `DailySalaryReportResponse`, `AttendancePairReportResponse`, `PageResponse`, `ErrorResponse`, plus request DTOs. Include the bearer security scheme. This file is the contract oracle; the contract tests (Task 22) assert the runtime matches it.

- [ ] **Step 2: Embed Swagger UI**

```bash
mkdir -p /tmp/swagger-ui && cd /tmp/swagger-ui
# download swagger-ui dist (e.g. from the swagger-ui GitHub release) into go/web/swagger-ui
cp -r <dist>/go/web/...
```

Use a recent `swagger-ui` dist (from the official GitHub release tarball). Place the `index.html`, `swagger-ui-bundle.js`, `swagger-ui-standalone-preset.js`, and CSS under `go/web/swagger-ui/`. Rewrite `index.html` to fetch `/docs/openapi.yaml`.

- [ ] **Step 3: Implement `docs.go` + wire routes**

```go
// docs.go
package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/swagger-ui
var swaggerUI embed.FS
//go:embed openapi.yaml
var openapiYAML []byte

func DocsHandler() http.Handler {
	sub, err := fs.Sub(swaggerUI, "web/swagger-ui")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /docs/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		w.Write(openapiYAML)
	})
	return mux
}
```

Wait: `openapi.yaml` lives at `go/openapi.yaml`, so the embed path must be `go:embed openapi.yaml` relative to the package directory (`go/httpapi`). Move `openapi.yaml` into `go/httpapi/openapi.yaml` OR reference with `//go:embed ../openapi.yaml` (embed does not allow `..`). Simplest: place the spec at `go/httpapi/openapi.yaml`. Update the File Structure note accordingly.

Register in `NewRouter`:

```go
mux.Handle("GET /docs/", DocsHandler())
```

Serve `/docs` (redirect) and `/docs/openapi.yaml` too.

- [ ] **Step 4: Build and commit**

```bash
cd go && go build ./... && cd ..
git add go/httpapi/openapi.yaml go/httpapi/docs.go go/web
git commit -m "feat(go): hand-maintained OpenAPI spec and Swagger UI at /docs"
```

---

### Task 22: Contract + integration test suite

**Files:**
- Create: `go/contract/contract_test.go`
- Create: `go/contract/testdb.go` (testcontainers helper)
- Create: `go/auth/service_integration_test.go`
- Create: `go/outlet/service_integration_test.go`
- Create: `go/attendance/service_integration_test.go`
- Create: `go/report/service_integration_test.go`

**Interfaces:**
- Produces: A `testdb` package helper `func Setup(t *testing.T) *db.Store` that starts a Postgres container (`postgres:17`), runs `db.Migrate`, returns `*db.Store`. A `httptest` server wiring the real handlers (a `func BuildTestServer(t *testing.T) (*httptest.Server, *db.Store)` in the contract package) so endpoint tests can run real requests end-to-end with a stub Firebase.

- [ ] **Step 1: Write `testdb.go`**

```go
package contract

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func Setup(t *testing.T) *db.Store {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "delta",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").WithStartupTimeout(60 * time.Second),
	}
	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	port, err := pg.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}
	url := fmt.Sprintf("postgres://postgres:postgres@localhost:%s/delta?sslmode=disable", port.Port())
	if err := db.Migrate(ctx, url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return db.NewStore(pool)
}
```

- [ ] **Step 2: Write integration tests**

In `go/auth/service_integration_test.go`, using `Setup(t)`:
- Seed a user (CreateUser), then exercise `Create`/`Rotate`/`Revoke`/`RevokeAllForUser`/`Cleanup` on `RefreshTokenService` against the real DB. Assert rotation revokes the old token (subsequent `Revoke` of old → `INVALID_TOKEN`).

In `go/outlet/service_integration_test.go`:
- Full lifecycle: owner creates outlet → auto OWNER/ACCEPTED membership; invite a second user by email → INVITED; employee accepts → ACCEPTED; employee lists `GetMyOutlets`; owner renames display name; employee leaves; owner re-invites (reopened); owner removes; delete outlet; historical reads still work.

In `go/attendance/service_integration_test.go`:
- Self clock-in as employee (server clock), owner managed entry, employee list-only-own, owner list-all/filter, update, delete, geofence rejection (outlet with geofence enabled and a far coordinate → FORBIDDEN).

In `go/report/service_integration_test.go`:
- Seed clock-in/out pairs across two days, `Calculate` → assert pair counts, day totals, grand totals, zero-activity day included, Excel bytes non-empty.

- [ ] **Step 3: Write `contract_test.go` — endpoint parity**

Build the server with all handlers wired (mirror `main.go` wiring, using a stub Firebase that returns a fixed `UserInfo`). Use `httptest.NewServer` and `http.Client`. For each endpoint assert exact JSON field names/values and error codes/messages from the spec. Coverage list (each a `t.Run`):
- login success shape (with stub Firebase) + 401 invalid token (stub returns error)
- refresh rotation + logout + logout-all
- outlet create → 201 shape; duplicate/conflict paths; owner-only 403s
- invite/accept/reject/leave/remove/display-name flows
- attendance self/managed/list/get/update/delete + geofence 403
- salary report JSON shape + xlsx content-type/filename
- delete account flow
- unauthenticated protected route → 401 empty body with `WWW-Authenticate`
- rate limit → 429 envelope after exceeding login limit from one IP

Use `go-cmp/cmp` for struct assertions; use raw JSON decode into `map[string]any` + key-presence checks for field-name parity.

- [ ] **Step 4: Run the full suite**

Run: `cd go && go vet ./... && go test ./...`
Expected: all pass. Requires Docker (testcontainers).

- [ ] **Step 5: Commit**

```bash
git add go/contract go/auth go/outlet go/attendance go/report
git commit -m "test(go): contract and integration test suite"
```

---

### Task 23: Docker, Compose, monitoring, Makefile, `.env.example`

**Files:**
- Create: `go/Dockerfile`
- Create: `go/Makefile`
- Modify: `docker-compose.yml` (root)
- Modify: `.env.example` (root)
- Modify: `monitoring/prometheus/prometheus.yml`

**Interfaces:**
- Produces: `make build`/`make test`/`make lint`/`make run`; compose builds the Go image and healthchecks `/healthz`+`/readyz`; Prometheus scrapes `/metrics`.

- [ ] **Step 1: Write `go/Dockerfile`**

```dockerfile
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/delta ./cmd/delta

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/delta /app/delta
USER app
EXPOSE 8080
ENTRYPOINT ["/app/delta"]
```

- [ ] **Step 2: Write `go/Makefile`**

```make
.PHONY: build test lint vet fmt run

build:
	go build ./cmd/delta

test:
	go test ./...

lint:
	go vet ./... && gofmt -l .

vet:
	go vet ./...

fmt:
	gofmt -w .

run:
	go run ./cmd/delta
```

- [ ] **Step 3: Update `.env.example`**

Replace Java vars with the Go env table from the spec §Config/environment (keep `POSTGRES_DB/USER/PASSWORD`, `JWT_SECRET`, `PROMETHEUS_BEARER_TOKEN`, `TRUST_PROXY_HEADERS`, `GRAFANA_ADMIN_USER/PASSWORD`; add `DATABASE_URL`, `PORT`, `AUTO_MIGRATE`, `LOG_LEVEL`, `LOG_FORMAT`; remove `JAVA_OPTS`).

- [ ] **Step 4: Update `docker-compose.yml`**

`app` service: `build: { context: ./go }`; env: `DATABASE_URL: postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD:-postgres}@postgres:5432/${POSTGRES_DB:-delta}`, `JWT_SECRET`, `PROMETHEUS_BEARER_TOKEN`, `FIREBASE_SERVICE_ACCOUNT_PATH: /app/firebase/service-account.json`, `TRUST_PROXY_HEADERS`, `PORT: 8080`; healthcheck → `wget -qO- http://localhost:8080/readyz || exit 1` (alpine has `wget`), interval 15s, start_period 40s. Drop `JAVA_OPTS`. Postgres/Prometheus/Grafana services unchanged.

- [ ] **Step 5: Update `monitoring/prometheus/prometheus.yml`**

Change `metrics_path: /actuator/prometheus` → `metrics_path: /metrics`.

- [ ] **Step 6: Validate and commit**

```bash
docker compose config && docker build -t delta-go ./go
git add go/Dockerfile go/Makefile docker-compose.yml .env.example monitoring/prometheus/prometheus.yml
git commit -m "feat(go): Docker, compose, prometheus, Makefile"
```

---

### Task 24: Docs — README, SETUP, STRUCTURE, AGENTS, go/README

**Files:**
- Modify: `README.md`
- Modify: `SETUP.md`
- Modify: `STRUCTURE.md`
- Modify: `AGENTS.md`
- Create: `go/README.md`

- [ ] **Step 1: Rewrite root docs for the Go app**

- `README.md`: point at the Go app; note the Java sources remain as reference; link `SETUP.md`, `STRUCTURE.md`, `AGENTS.md`, `go/README.md`.
- `SETUP.md`: Go prerequisites (Go 1.25, Docker), env table from the spec, `docker compose up --build`, ops endpoints (`/healthz`, `/readyz`, `/metrics`, `/docs`), k6 load-test usage unchanged, Prometheus/Grafana notes with the new `/metrics` path.
- `STRUCTURE.md`: replace the Java package listing with the Go package tree from this plan's File Structure section and the domain descriptions.
- `AGENTS.md`: rewrite the Project summary, Commands (`make test`, `make build`, `docker compose config`), Architectural rules (flat Go packages, no cycles, `db` owns models/queries, sqlc workflow, migrations embedded + auto-run), Persistence rules (CHECK constraints, soft-delete semantics preserved), Security/domain rules (keep the business rules verbatim from the current file — they still apply), Testing expectations (`go test ./...`, testcontainers integration, contract tests).
- Create `go/README.md`: quickstart (prereqs, `cp .env.example .env`, `make run`, `make test`), package map, ops endpoints, link to the spec/plan.

- [ ] **Step 2: Update AGENTS.md domain rules faithfully**

Copy the existing "Domain rules", "Pagination", "Cross-cutting concerns", "Docker/deployment notes" content into the new Go-oriented AGENTS.md, changing only tooling references (Java→Go commands, Flyway→golang-migrate, `application.properties`→env, springdoc→openapi.yaml). Preserve every business rule sentence exactly.

- [ ] **Step 3: Commit**

```bash
git add README.md SETUP.md STRUCTURE.md AGENTS.md go/README.md
git commit -m "docs: rewrite project docs for Go backend"
```

---

### Task 25: Final verification + k6 load tests

**Files:**
- Modify: (none required — run only)
- Optional: `go/README.md` (record verification results)

- [ ] **Step 1: Full Go verification**

```bash
cd go && gofmt -l . && go vet ./... && go test ./... && cd ..
```
Expected: gofmt prints nothing; vet clean; all tests pass.

- [ ] **Step 2: Boot the compose stack**

```bash
cp .env.example .env
# place firebase/service-account.json (or leave it out; login will 401)
docker compose up --build -d
curl -fsS http://localhost:8080/healthz          # {"status":"UP"}
curl -fsS http://localhost:8080/readyz           # {"status":"UP"}
curl -fsS http://localhost:8080/docs/            # Swagger UI
```

- [ ] **Step 3: Run the k6 load tests unchanged**

```bash
./loadtest/seed.sh
docker run --rm -i -e BASE_URL=http://localhost:8080 grafana/k6 run - < loadtest/smoke.js
docker run --rm -i -e BASE_URL=http://localhost:8080 grafana/k6 run - < loadtest/capacity.js
docker run --rm -i -e BASE_URL=http://localhost:8080 grafana/k6 run - < loadtest/rate-limit.js
```
Expected: smoke and capacity pass; rate-limit script observes 429s as designed. If the k6 scripts reference `/actuator/prometheus` or Java-only endpoints, verify against `loadtest/` sources and adjust only the k6 config (never the Go app contract).

- [ ] **Step 4: Verify metrics scrape**

```bash
curl -fsS -H "Authorization: Bearer $(cat monitoring/prometheus/prometheus-token.txt)" http://localhost:8080/metrics | head
# confirm metric names like auth.login.success, outlet.created appear after exercising the API
```

- [ ] **Step 5: Commit any fixes**

If verification surfaced fixes, commit them with clear messages. Then summarize results in `go/README.md` (optional).

---

## Self-Review Summary (run at plan end)

1. **Spec coverage** — every spec section maps to a task: §Data model → Task 2; §HTTP layer/errors/pagination → Tasks 3–4, 20; §Auth → Tasks 7–9; §API contract auth/users → Tasks 9–10; outlets → Tasks 11–13; attendance → Tasks 14–16; reports → Tasks 17–19; §Rate limiting → Task 20; §Request logging/headers → Task 4; §Audit → Task 5; §Metrics → Task 5; §Health → Task 6; §API docs → Task 21; §Config → Task 1 + 23; §Docker/Compose/monitoring → Task 23; §Testing → Task 22; §Deliverables/phases → the whole plan; §Out of scope → respected (no Java deletion).
2. **Placeholder scan** — no "TBD"/"TODO"/"similar to Task N". Where sqlc output names must be matched (params structs, JOIN row fields), the plan says to verify against generated code, which is an instruction, not a placeholder.
3. **Type consistency** — shared names used consistently: `httpapi.NewPageResponse`, `httpapi.DecodeJSON`, `db.Store`, `db.Querier`, `auth.Require`, `SubjectID()`, `apiMux.Handle(...)`; `SubjectID()` implemented by `db.User` via `db/subject.go`; `NewRouter` signature updated once (Task 6/9/20) to accept attach + rate middlewares.
