# AGENTS.md

Guidance for coding agents working in this repository.

## Project summary

`delta` is a Go employee attendance backend.

Stack:

- Go 1.25
- `net/http` with stdlib `ServeMux` (method + path patterns)
- pgx/v5 (`pgxpool`)
- SQLC (generated query layer)
- golang-migrate (embedded migrations)
- PostgreSQL
- Firebase Admin SDK
- Local JWT access tokens and refresh tokens
- Prometheus metrics
- Docker / Docker Compose

Current API version prefix:

```text
/api/v1
```

## API documentation

- The OpenAPI spec is hand-maintained at `go/httpapi/openapi.yaml` and served at `/docs` (embedded Swagger UI) and `/docs/openapi.yaml`.
- Changes to handlers and DTOs must be reflected in the spec manually.
- Contract tests in `go/contract` assert the runtime matches the spec.

## Important commands

Run tests:

```bash
cd go && make test
```

Build binary:

```bash
cd go && make build
```

Lint:

```bash
cd go && make lint
```

Validate Docker Compose:

```bash
docker compose config
```

Run stack locally:

```bash
cp .env.example .env
# edit .env and provide Firebase service account file

docker compose up --build
```

Regenerate sqlc code after editing `go/db/queries/*.sql`:

```bash
cd go/db && sqlc generate
```

## Architectural rules

- Keep handlers thin.
- Put business logic in services.
- Use DTOs for request/response payloads.
- Do not expose `db` models directly from handlers; map to response DTOs.
- Keep persistence/query concerns in `go/db`; services use `db.Store` / `db.Querier`.
- Use flat packages with one clear purpose and no import cycles; domain packages must not import each other sideways — `cmd/delta/main.go` is the wiring point.
- Keep changes minimal and consistent with existing style.
- Document every exported identifier with a doc comment; comment non-obvious behavior.
- Prefer enums over lookup tables unless there is a strong reason otherwise.
- No Java/Spring/JPA references in Go code.

## Persistence rules

- Database schema is managed by golang-migrate migrations in:

```text
go/db/migrations
```

- Do not rely on any ORM to create/update production schema.
- `AUTO_MIGRATE=true` applies migrations at startup (the default).
- When changing entities, add a new golang-migrate migration.
- Existing base auditing provides `createdAt` / `updatedAt`.
- Attendance also has `createdBy` / `updatedBy` using the authenticated user ID.

## Authentication/security context

- Authenticated principal is the local `db.User` row.
- JWT middleware loads only active users (by id with `deleted_at` unset).
- Public auth endpoints:
  - `/api/v1/auth/login`
  - `/api/v1/auth/refresh`
  - `/api/v1/auth/logout`
- Most other endpoints require authentication (`auth.Require`).
- When no Firebase service account is configured, a stub client is used and login returns 401 `INVALID_TOKEN` ("Invalid Firebase ID Token").

## Domain rules

### Outlets and memberships

- Creating an outlet auto-creates an accepted `OWNER` membership for the creator.
- Owners can invite existing users by email.
- Memberships carry an owner-controlled `displayName` that is initialized to the user's account name when the membership record is first created.
- Owners can update a member's `displayName` via `PUT /api/v1/outlets/{outletId}/memberships/{membershipId}/display-name`.
- The membership `displayName` is the human-facing identifier; it is forwarded to membership, attendance, and salary report responses (and the Excel export).
- Historical/read paths resolve `displayName` from the membership row even after soft removal, so attendance logs keep showing the custom display name; it only falls back to the user's account name when no membership row exists.
- Invited employees can accept or reject invites.
- Employees can leave an outlet on their own via `POST /api/v1/outlets/{outletId}/leave`; this soft-removes their membership exactly like owner-driven removal and is reversible by an owner re-invite.
- Owners cannot leave an outlet through the leave endpoint (they delete the outlet instead).
- Re-invite reuses existing membership and resets status to `INVITED`.
- Membership removal is soft removal with `removedAt` / `removedBy`.
- Removed memberships should disappear from active access/invite/listing checks.
- Historical attendance must remain valid after membership removal.
- Owner memberships cannot be removed through the current remove endpoint.

### Outlet deletion

- Outlets are soft-deleted via `DELETE /api/v1/outlets/{outletId}` (owner-only), setting `removedAt` / `removedBy` on the outlet row.
- Deleting an outlet does NOT soft-remove its memberships; those remain so a later data-retention cleanup job can identify the outlet's membership and attendance records by `removed_at`.
- Soft-deleted outlets are excluded from active access checks (`OutletRepository.findByIdAndRemovedAtIsNull`) and hidden from `GET /outlets/mine` and `GET /outlets/invites`.
- Historical attendance reads remain readable for deleted outlets; attendance writes (create/update/delete) are rejected for deleted outlets.

### Attendance

- Entity name is `AttendanceEntry`.
- Attendance belongs to one user and one outlet.
- Entry type is enum-based: `CLOCK_IN`, `CLOCK_OUT`.
- Employees can create their own entries only with server UTC timestamp.
- Owners can create/update/delete employee attendance entries with custom times.
- If outlet geofencing is enabled, attendance write coordinates must be within outlet radius.
- Historical report/read/update behavior must not depend on the employee still being an active member.

### Reports

- Owners can generate salary reports for an employee in an outlet.
- Reports accept `startTime`, `endTime`, `timezone`, and `hourlyRate`.
- DB filtering uses exact instants.
- Daily grouping and Excel display use the provided IANA timezone.
- Excel exports use Apache POI.

### User accounts

- Users can delete their own account via `DELETE /api/v1/users/me`.
- Deleting an account removes the Firebase Auth record (`FirebaseService.deleteUser`) so a future sign-in with the same provider email receives a fresh UID, then soft-deletes the local user (`deletedAt`).
- On deletion the user's email is moved to `historicalEmail` and the active `email` column is cleared, so the unique email constraint does not block a brand-new account using the same email.
- All active refresh tokens are revoked on deletion.
- Historical attendance and membership rows are preserved after user deletion; attendance continues to resolve names from membership rows.
- Login only resolves active users (`findByAuthUidAndDeletedAtIsNull`), so a deleted account is never resurrected.

## Pagination

Collection endpoints should use DB-level pagination with `httpapi.PageParams` and return `PageResponse<T>`.

Shared helpers:

```text
httpapi.ParsePageParams / PageParams (go/httpapi/pagination.go)
httpapi.NewPageResponse / WritePage (go/httpapi/response.go)
```

## Cross-cutting concerns

### Auditing

Use `audit.Recorder` for business events worth investigating later:

```text
go/audit
```

Examples:

- auth login/refresh/logout-all
- user account deleted
- outlet created/updated/geofence toggled/deleted
- membership invited/accepted/rejected/removed/left
- attendance created/updated/deleted
- report generated/exported

### Metrics

Use `metrics.Registry` for business counters:

```text
go/metrics
```

Metrics are exposed via Prometheus format at:

```text
/metrics
```

Keep that endpoint private to monitoring systems.

### Request logging

`httpapi` middleware (`RequestID` + `RequestLog`) adds request IDs and request completion logs.

Do not log:

- access tokens
- refresh tokens
- Firebase ID tokens
- full Authorization headers
- unnecessary PII

### Rate limiting

`httpapi.RateLimiter` provides in-memory single-instance rate limiting.

For horizontally scaled deployments, move rate limiting to Redis or the gateway/ingress layer.

## Testing expectations

For behavior changes:

- Add/update service tests for business logic.
- Add handler/integration/contract tests for auth, authorization, validation, and endpoint response shape when relevant.
- Run `make test` (`go test ./...`) before finalizing.

## Docker/deployment notes

- Docker build uses the Go toolchain via `go/Dockerfile` (multi-stage: `golang:1.25-alpine` builder, non-root `alpine` runtime).
- The builder stage runs `go mod download` before copying sources; the module cache is not persisted between builds.
- Runtime image runs as non-root user.
- Local secrets should be in `.env` and `firebase/service-account.json`; both are ignored by Git.
- Production secrets should come from the deployment platform secret manager.

## Note

Once the changes are implemented, determine whether any documentation in this file, README.md, STRUCTURE.md or SETUP.md is required and make it. Always inform the user explicitly for any of these changes and highlight them in the response chat.
