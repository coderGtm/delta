# Design: Rewrite `delta` backend in Go

Date: 2026-08-14
Status: Approved (design sections 1–3 accepted by human partner)
Branch: `go-rewrite` (off `main`; `main` keeps the working Java app untouched)

## Purpose

Replace the Java/Spring Boot attendance backend with a Go implementation that is
a **drop-in replacement for all client-facing behavior**: the `/api/v1/*` API
contract and the business rules stay identical. The database is empty and the
app has not been deployed, so the schema may be authored fresh. The motivation
is developer preference for Go, a smaller memory footprint / cheaper hosting,
and easier long-term maintenance by AI agents.

Ops-only endpoints that only the owner uses (`/actuator/*`, `/docs`) are
replaced with Go-native equivalents.

## Scope / parity rules

- **Client-facing (`/api/v1/*`): byte-for-byte parity of contract and behavior.**
  - Same routes, HTTP methods, query params, JSON field names (camelCase), JSON
    shapes, error codes, error messages, validation messages, pagination
    behavior, and default sorts.
  - Same business rules for auth, users, outlets, memberships, attendance,
    geofencing, and salary reports.
  - Same counter/tag names for Prometheus metrics and same audit action strings.
- **Ops endpoints (owner-only): free to be Go-native.**
  - `/healthz` (liveness), `/readyz` (readiness), `/metrics` (Prometheus,
    bearer-token protected), `/docs` (Swagger UI over a hand-maintained
    `openapi.yaml`).
- The Java source stays at the repo root on this branch as the reference
  implementation and is never modified by this work.

## Tech stack

| Concern | Choice |
| --- | --- |
| Language | Go 1.25 |
| HTTP | stdlib `net/http` `ServeMux` with method+path patterns (Go 1.22+) |
| DB driver | `github.com/jackc/pgx/v5` (pgxpool) |
| SQL codegen | `sqlc` (`emit_interface: true`, `emit_json_tags: false`) |
| Migrations | `github.com/golang-migrate/migrate/v4`, `source/iofs`, embedded, auto-run on boot |
| Firebase | `firebase.google.com/go/v4` (admin SDK) |
| JWT | `github.com/golang-jwt/jwt/v5` (HS256) |
| Excel | `github.com/xuri/excelize/v2` |
| Metrics | `github.com/prometheus/client_golang` |
| Logging | stdlib `log/slog` (structured) |
| Tests | stdlib `testing`, table-driven; `github.com/testcontainers/testcontainers-go` for Postgres integration; `github.com/google/go-cmp/cmp` for diffs |
| UUIDs | `github.com/google/uuid` |

## Repository layout

Go app lives in `go/`. Java stays at the repo root. Shared files updated only
where the Go app needs them (see Docker/Compose/monitoring section).

```
delta/
├── go/                         # Go rewrite
│   ├── cmd/delta/main.go       # wiring only, no business logic
│   ├── config/config.go        # single Config struct, env-based
│   ├── httpapi/                # router, middleware, error/response envelopes, pagination, validation helpers
│   ├── db/                     # pgx pool, sqlc generated code, embedded migrations
│   │   ├── migrations/         # golang-migrate .sql files (embedded via go:embed)
│   │   ├── queries/            # per-domain .sql files (auth, user, outlet, attendance, report, audit)
│   │   └── (sqlc generated: models.go, queries.go, db.go)
│   ├── auth/                   # Firebase verify/delete, JWT, refresh tokens, handlers
│   ├── user/                   # account deletion, handlers
│   ├── outlet/                 # outlets + memberships domain + handlers
│   ├── attendance/             # attendance + geofence + handlers
│   ├── report/                 # salary reports + Excel + handlers
│   ├── audit/                  # audit event recording
│   ├── metrics/                # Prometheus counter registry
│   ├── openapi.yaml            # hand-maintained API contract (also contract-test oracle)
│   ├── Dockerfile
│   ├── Makefile
│   └── go.mod
├── docker-compose.yml          # updated: builds go/, healthchecks → /healthz,/readyz
├── monitoring/prometheus/prometheus.yml  # scrape path → /metrics
├── loadtest/                   # k6 scripts reused unchanged
├── src/ …                      # Java reference (untouched)
└── docs/superpowers/specs/     # this spec
```

Rules from the Go skill that apply:

- Flat domain packages, one clear purpose each. No `internal/` layering, no
  `utils/`/`helpers/`/`common/` bags.
- Domain packages do not import each other sideways; `main` is the wiring point.
- Handlers live in their domain package; `httpapi` holds only shared concerns.
- Accept small interfaces where needed (sqlc `Querier`), return concrete structs.
- No heavy worker pools, no BDD/mocking frameworks. Fakes/stubs via interfaces.

## Data model

DB is empty. Author fresh migrations. The final schema preserves the Java
schema's semantics plus the approved improvements. One init migration is enough
(`000001_init.up.sql` / `000001_init.down.sql`).

Improvements applied vs the Java schema:

1. CHECK constraints: `outlets.radius_meters > 0`; latitude within
   [-90, 90] and longitude within [-180, 180] on `outlets` and
   `attendance_entries`; `outlet_memberships.status` in
   (ACCEPTED, INVITED, REJECTED); `outlet_memberships.role` in (OWNER, EMPLOYEE);
   `attendance_entries.type` in (CLOCK_IN, CLOCK_OUT).
2. Drop redundant indexes on `users(auth_uid)` and `users(email)` (unique
   constraints already index them).
3. FKs: `attendance_entries.created_by` → `users(id)`,
   `attendance_entries.updated_by` → `users(id)`.
4. `audit_events.metadata_json` type is `jsonb`.
5. `id uuid primary key default gen_random_uuid()` on all tables; `created_at`
   and `updated_at` are `timestamptz not null default now()`.

Schema (final):

```sql
create table users (
    id uuid primary key default gen_random_uuid(),
    auth_uid varchar(255) unique,
    name varchar(255) not null,
    email varchar(255) unique,
    phone varchar(255),
    historical_email varchar(255),
    deleted_at timestamptz,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table outlets (
    id uuid primary key default gen_random_uuid(),
    name varchar(150) not null,
    latitude numeric(10,7) not null check (latitude between -90 and 90),
    longitude numeric(10,7) not null check (longitude between -180 and 180),
    radius_meters integer not null check (radius_meters > 0),
    geofence_enabled boolean not null default false,
    removed_at timestamptz,
    removed_by_user_id uuid,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint fk_outlet_removed_by foreign key (removed_by_user_id) references users(id)
);

create table outlet_memberships (
    id uuid primary key default gen_random_uuid(),
    outlet_id uuid not null references outlets(id),
    user_id uuid not null references users(id),
    role varchar(20) not null check (role in ('OWNER','EMPLOYEE')),
    status varchar(20) not null check (status in ('ACCEPTED','INVITED','REJECTED')),
    display_name varchar(255) not null,
    invited_by_user_id uuid references users(id),
    removed_at timestamptz,
    removed_by_user_id uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    constraint uk_outlet_membership_outlet_user unique (outlet_id, user_id)
);

create table refresh_tokens (
    id uuid primary key default gen_random_uuid(),
    token_hash varchar(255) not null unique,
    expires_at timestamptz not null,
    revoked boolean not null,
    user_id uuid not null references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table attendance_entries (
    id uuid primary key default gen_random_uuid(),
    user_id uuid not null references users(id),
    outlet_id uuid not null references outlets(id),
    type varchar(20) not null check (type in ('CLOCK_IN','CLOCK_OUT')),
    entry_time timestamptz not null,
    latitude numeric(10,7) not null check (latitude between -90 and 90),
    longitude numeric(10,7) not null check (longitude between -180 and 180),
    created_by uuid references users(id),
    updated_by uuid references users(id),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table audit_events (
    id uuid primary key default gen_random_uuid(),
    actor_user_id uuid,
    action varchar(100) not null,
    entity_type varchar(100) not null,
    entity_id uuid,
    metadata_json jsonb,
    ip_address varchar(100),
    user_agent varchar(500),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create index idx_outlet_memberships_lookup on outlet_memberships(outlet_id, user_id, removed_at);
create index idx_outlet_memberships_user_status on outlet_memberships(user_id, status, removed_at);
create index idx_outlet_memberships_outlet_removed on outlet_memberships(outlet_id, removed_at);
create index idx_refresh_tokens_user_revoked on refresh_tokens(user_id, revoked);
create index idx_refresh_tokens_expires_at on refresh_tokens(expires_at);
create index idx_attendance_entries_outlet_entry_time on attendance_entries(outlet_id, entry_time);
create index idx_attendance_entries_outlet_user_entry_time on attendance_entries(outlet_id, user_id, entry_time);
create index idx_audit_events_actor_created_at on audit_events(actor_user_id, created_at);
create index idx_audit_events_entity_created_at on audit_events(entity_type, entity_id, created_at);
create index idx_outlets_removed_at on outlets(removed_at);
```

`loadtest/seed.sql` inserts fixed UUIDs directly, so it continues to work
unchanged against this schema.

## HTTP layer

- stdlib `net/http` `ServeMux` with method-scoped patterns, e.g.
  `mux.HandleFunc("POST /api/v1/auth/login", h.login)`. Path params via
  `r.PathValue`. Literal segments (`/mine`, `/invites`) take precedence over
  wildcards automatically.
- Middleware as `func(http.Handler) http.Handler` composition, outermost-first:
  1. Request-ID + slog request logging
  2. Panic recovery (→ 500 envelope)
  3. JWT authentication
  4. Rate limiting
  5. Security headers
  6. Body size limit (`http.MaxBytesReader`, 2 MiB) applied per request in
     handlers or middleware.
- `http.Server` with timeouts: `ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`,
  `WriteTimeout: 30s`, `IdleTimeout: 120s`.
- Graceful shutdown on SIGINT/SIGTERM: stop accepting, drain in-flight,
  close DB pool, stop background tickers.
- JSON encoding: struct tags produce exact camelCase field names from the Java
  records (see API contract section). Timestamps encode as ISO-8601 UTC with
  `Z` (RFC 3339, e.g. `2026-08-14T12:00:00Z`). Decoding rejects body size
  beyond 2 MiB. Malformed JSON returns 400 with our standard envelope
  (`BAD_REQUEST`, message `"Malformed request body"`). This is a deliberate,
  documented deviation: Java lets Spring's default handler answer with a
  non-standard error body; we keep the consistent envelope.
- Pagination: parse Spring-style query params `page` (0-based, default 0),
  `size` (default 20), `sort=field,dir` (repeatable, comma-separated). If no
  `sort` param, apply the endpoint's default sort. Endpoint default sorts:
  - `GET /outlets/mine`, `GET /outlets/invites`: `updatedAt DESC`
  - `GET /outlets/{id}/memberships`: `createdAt ASC`
  - `GET /outlets/{id}/attendance`: `entryTime DESC, createdAt DESC`
  Responses use the `PageResponse` envelope (below).
- Validation: hand-written per-DTO functions replicating the Java DTO
  constraints and messages; on failure return 400 with code `VALIDATION_ERROR`
  and message = the joined field messages with `", "` separator (matching
  Java). See API contract for each endpoint's messages.

## Error handling

One shared error type in `httpapi`:

```go
type APIError struct {
    Status  int
    Code    string
    Message string
}
```

JSON envelope (exact):

```json
{ "code": "NOT_FOUND", "message": "Outlet not found: <id>", "timestamp": "2026-08-14T12:00:00Z" }
```

Codes and statuses:

| Code | HTTP | Java source |
| --- | --- | --- |
| `BAD_REQUEST` | 400 | `BadRequestException` |
| `CONFLICT` | 409 | `ConflictException` |
| `FORBIDDEN` | 403 | `ForbiddenException` |
| `NOT_FOUND` | 404 | `ResourceNotFoundException` |
| `INVALID_TOKEN` | 401 | `InvalidTokenException` |
| `VALIDATION_ERROR` | 400 | Spring `MethodArgumentNotValidException` |
| `RATE_LIMIT_EXCEEDED` | 429 | rate-limit filter |

Service code wraps errors with `fmt.Errorf("...: %w", err)`; handlers translate
to `APIError` at the boundary. Postgres error code 23505 (unique_violation)
maps to `CONFLICT` with message `"User already has a membership record for this
outlet"` where the Java code catches `DataIntegrityViolationException`.

## Auth

- Firebase Admin SDK verifies ID tokens (`auth.VerifyIDToken`). Extract
  `uid`, `name`, `email`, and claim `phone_number` into a `FirebaseUserInfo`
  struct. On failure → `INVALID_TOKEN` (401), message `"Invalid Firebase ID
  Token"`.
- Login flow: verify token → find user by `auth_uid AND deleted_at IS NULL`;
  if missing, create local user (`name`, `email`, `phone`) → issue access
  token + create refresh token → respond.
- Access JWT: HS256, `sub` = user UUID, `iat`, `exp`. Secret from `JWT_SECRET`
  (HMAC key from the secret bytes, same as Java `Keys.hmacShaKeyFor`).
  Expiration default 900000 ms (15 min).
- JWT middleware: parse `Authorization: Bearer <token>`, verify, extract `sub`,
  load active user (`id AND deleted_at IS NULL`), attach user to request
  context. Any failure → treat as anonymous (do not reject here). A separate
  `requireAuth` guard returns 401 for protected routes.
- Refresh tokens: 32 random bytes via `crypto/rand`, encoded
  `base64.RawURLEncoding`. Stored as SHA-256 hex. Column `token_hash` unique.
  - `create(user)` → persist `revoked=false`, `expires_at = now + 30d`.
  - `validate(raw)` → lookup by hash; error if missing / revoked / expired →
    `INVALID_TOKEN`.
  - `rotate(raw)` → validate, mark old revoked, issue new.
  - `revoke(raw)`, `revokeAllForUser(userID)`.
  - `cleanup()` background ticker (default interval 24 h): delete expired;
    delete revoked with `updated_at` older than retention (default 7 d). Runs
    via goroutine with context cancellation on shutdown.
- Logout: `POST /auth/logout` (public, revokes one token), `POST
  /auth/logout-all` (authenticated, revokes all for the user).

## API contract (`/api/v1`) — exact routes and shapes

All routes, methods, params, request/response JSON, error codes, and messages
must match the Java controllers. Reference: `src/main/java/com/coderGtm/delta`
controllers + DTOs. The `openapi.yaml` in `go/` is the canonical written copy;
contract tests enforce it.

### Auth

- `POST /auth/login` — body `{ "firebaseIdToken": string (1..8192 chars, required) }`.
  - 200 `{ id, name, email, phone, accessToken, refreshToken, createdAt, updatedAt }`
  - 401 `INVALID_TOKEN` "Invalid Firebase ID Token"
  - 400 `VALIDATION_ERROR` "must not be blank" / "size must be between 0 and 8192"
- `POST /auth/refresh` — body `{ "refreshToken": string (1..512, required) }`.
  - 200 `{ accessToken, refreshToken }`
  - 401 `INVALID_TOKEN` ("Invalid refresh token" / "Refresh token has been revoked" / "Refresh token has expired")
- `POST /auth/logout` — body `{ "refreshToken": string (1..512, required) }`. 204.
- `POST /auth/logout-all` — authenticated. 204.

### Users

- `DELETE /users/me` — authenticated. 204.
  - Order: Firebase `DeleteUser(authUid)` → revoke all refresh tokens → set
    `historicalEmail = email`, clear `email`, set `deleted_at`. If already
    deleted → 409 `CONFLICT` "Account has already been deleted". If Firebase
    delete fails → 409 `CONFLICT` "Failed to delete the user from the
    authentication provider".

### Outlets

- `POST /outlets` — body `{ name (1..150), latitude, longitude, radiusMeters (>0) }`.
  - 201 `OutletResponse`. Creates outlet + `OWNER`/`ACCEPTED` membership with
    `display_name = user.name`.
- `GET /outlets/{outletId}` — accepted member only. 200 `OutletResponse`.
- `PUT /outlets/{outletId}` — owner. Body like create. 200.
- `PUT /outlets/{outletId}/geofence` — owner. Body `{ geofenceEnabled: bool }`. 200.
- `GET /outlets/mine` — paginated `PageResponse<OutletMembershipResponse>`,
  accepted memberships, active outlets only.
- `GET /outlets/invites` — paginated, `INVITED` memberships, active outlets only.
- `GET /outlets/{outletId}/memberships` — owner. Paginated, non-removed.
- `DELETE /outlets/{outletId}` — owner. Soft delete outlet (`removed_at`,
  `removed_by`). Does NOT touch memberships. 204.
- `POST /outlets/{outletId}/leave` — employee only. Soft-removes own
  membership. Owner → 400 "Owners cannot leave an outlet through this endpoint". 204.
- `POST /outlets/{outletId}/memberships/invite` — owner. Body `{ email }`.
  - Existing active accepted → 409 "User is already an active member of this outlet".
  - Existing pending invited → 409 "User already has a pending invitation for this outlet".
  - Existing removed/rejected → reopen: role EMPLOYEE, status INVITED, clear
    removed_at/by, set invited_by.
  - New membership: display_name = invitee.name, role EMPLOYEE, status INVITED.
  - Unique violation → 409 "User already has a membership record for this outlet".
  - Email lookup is case-insensitive on active users.
- `DELETE /outlets/{outletId}/memberships/{membershipId}` — owner. Soft-remove;
  never for OWNER role (400 "Owner memberships cannot be removed through this
  endpoint"); membership must belong to outlet (400 "The provided membership
  does not belong to the requested outlet"). 204.
- `PUT /outlets/{outletId}/memberships/{membershipId}/display-name` — owner.
  Body `{ displayName }`. 200 `OutletMembershipResponse`.
- `POST /memberships/{membershipId}/accept` — own invite only (else 403).
  Only `INVITED` (else 400 "Only pending invitations can be accepted"). 200.
- `POST /memberships/{membershipId}/reject` — same guards, sets `REJECTED`,
  message "Only pending invitations can be rejected". 200.

`OutletResponse`: `{ id, name, latitude, longitude, radiusMeters,
geofenceEnabled, createdAt, updatedAt }`.

`OutletMembershipResponse`: `{ membershipId, outlet: OutletResponse, userId,
userName, userEmail, displayName, role, status, invitedByUserId,
invitedByUserName, createdAt, updatedAt }`.

### Attendance

- `POST /outlets/{outletId}/attendance` — employee only (owner → 403 "Only
  accepted employees can create their own attendance entries"). Body `{
  type: CLOCK_IN|CLOCK_OUT, latitude, longitude }`. `entry_time` = server UTC
  clock. Geofence enforced if enabled. 201 `AttendanceEntryResponse`.
- `POST /outlets/{outletId}/attendance/manage` — owner. Body `{ userId, type,
  entryTime, latitude, longitude }`. Target must be ACCEPTED EMPLOYEE (400
  "Attendance can only be created for accepted employee memberships"). 201.
- `GET /outlets/{outletId}/attendance` — owner: all, or filtered by `userId`.
  Employee: own only; requesting another userId → 403 "Employees can only view
  their own attendance entries". Paginated, default sort `entryTime desc,
  createdAt desc`.
- `GET /outlets/{outletId}/attendance/{attendanceEntryId}` — owner or own (else
  403). 200.
- `PUT /outlets/{outletId}/attendance/{attendanceEntryId}` — owner. Body `{
  type, entryTime, latitude, longitude }`. 200.
- `DELETE /outlets/{outletId}/attendance/{attendanceEntryId}` — owner. 204.

Rules:
- Writes (create/update/delete) rejected for removed outlets (treat as
  not-found: 404 "Outlet not found: <id>").
- Reads still work for removed outlets and after membership removal.
- Geofence: if `geofence_enabled`, haversine distance from outlet center to
  request coords must be ≤ `radius_meters`; else 403 "Attendance location is
  outside the outlet geofence" and increment
  `attendance.geofence.rejected{outletId}`.
- `displayName` in responses: resolved from the membership row even if removed;
  falls back to the user account name when no membership row exists.

`AttendanceEntryResponse`: `{ id, outletId, userId, userName, userEmail,
displayName, type, entryTime, latitude, longitude, createdByUserId,
updatedByUserId, createdAt, updatedAt }`.

### Reports

- `GET /outlets/{outletId}/reports/salary` — owner. Query params:
  `userId` (required), `startTime` (required, ISO instant), `endTime`
  (required, ISO instant), `timezone` (required, IANA), `hourlyRate`
  (required, > 0). 200 `SalaryReportResponse`.
- `GET /outlets/{outletId}/reports/salary.xlsx` — owner. Same params. 200
  `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` with
  `Content-Disposition: attachment; filename="salary-report-<userId>-<start>-<end>.xlsx"`
  where `<start>`/`<end>` are the ISO instant strings with `:` → `-`.

Validation messages: "User ID is required", "Start time is required", "End
time is required", "Timezone is required", "Hourly rate must be greater than
zero", "End time must be after start time", "Timezone must be a valid IANA
timezone", "Salary reports can cover at most 366 local days".

Report logic:
- Entries fetched for `(outlet_id, user_id)` with `entry_time >= startTime AND
  entry_time < endTime`, ordered ascending.
- Group by local date in the provided IANA zone.
- `completedPairs`: iterate entries; track pending CLOCK_IN; on CLOCK_OUT that
  is strictly after the pending CLOCK_IN, emit a pair and clear pending.
- Hours = (clockOut - clockIn) seconds / 3600, rounded 2 decimals HALF_UP.
- Day hours = sum of pair hours, scale 2 HALF_UP. Day salary = dayHours ×
  hourlyRate, scale 2 HALF_UP. Totals = sums, scale 2.
- Rows for every local date in range (including zero-activity days) up to 366
  days.
- Employee info from the membership row (displayName) and user row
  (name/email); employee membership may be removed/removed outlet — history
  stays readable.
- Owner authorization: current user must have an active ACCEPTED OWNER
  membership for the outlet, else 404 / 403 as in Java.

`SalaryReportResponse`: `{ outletId, outletName, userId, userName, userEmail,
displayName, startTime, endTime, timezone, hourlyRate, totalHours,
totalSalary, days: [DailySalaryReportResponse] }`.

`DailySalaryReportResponse`: `{ date, attendancePairs:
[AttendancePairReportResponse], totalHours, hourlyRate, salary }`.

`AttendancePairReportResponse`: `{ clockIn, clockOut, hours }`.

Excel workbook layout (must match `writeWorkbook` in
`report/service/SalaryReportService.java`): sheet "Salary Report"; title row;
metadata row (Outlet, Employee "displayName <email>", Period, Timezone);
header row (Date, "Clock In 1", "Clock Out 1", …, "Total Hours", "Hourly
Rate", "Salary"); one row per day; TOTAL row. `maxPairs` drives column count.
Cell values sanitized against formula injection (CWE-1236): prefix `'` when the
first char is `=`, `+`, `-`, `@`, `<= 0x20`, or `0x7F`.

### Common envelopes

`PageResponse<T>`: `{ content: [], page, size, totalElements, totalPages,
first, last, empty }`.

`ErrorResponse`: `{ code, message, timestamp }`.

## Rate limiting

Replicate the Java policy table exactly (limits, windows, key strategies):

| Method | Path pattern | Limit | Window | Key |
| --- | --- | --- | --- | --- |
| POST | /api/v1/auth/login | 10 | 1m | IP |
| POST | /api/v1/auth/refresh | 30 | 1m | IP |
| POST | /api/v1/auth/logout | 30 | 1m | IP |
| POST | /api/v1/auth/logout-all | 30 | 1m | USER_OR_IP |
| POST | /api/v1/outlets/*/memberships/invite | 20 | 1m | USER_OR_IP |
| POST | /api/v1/outlets/*/attendance | 20 | 1m | USER_OR_IP |
| POST | /api/v1/outlets/*/attendance/manage | 60 | 1m | USER_OR_IP |
| PUT | /api/v1/outlets/*/attendance/* | 60 | 1m | USER_OR_IP |
| PUT | /api/v1/outlets/*/geofence | 20 | 1m | USER_OR_IP |
| GET | /api/v1/outlets/*/reports/salary | 30 | 1m | USER_OR_IP |
| GET | /api/v1/outlets/*/reports/salary.xlsx | 10 | 1m | USER_OR_IP |

- In-memory fixed-window counters (`map[string]*windowCounter` + `sync.Mutex`),
  matching Java semantics: counter resets when the window has passed.
- Key strategies: `IP` = client IP (see below); `USER_OR_IP` = authenticated
  user ID if present, else client IP.
- On exceed: 429, `Retry-After: <seconds-until-window-end>`, body
  `{ code: "RATE_LIMIT_EXCEEDED", message: "Too many requests. Please retry
  later.", timestamp }`.
- Client IP resolution: if `TRUST_PROXY_HEADERS=true`, honor `X-Forwarded-For`
  (first entry) then `X-Real-IP`, else `r.RemoteAddr`.
- Path matching: single-segment `*` wildcard matcher (custom, small) matching
  Java Ant-style patterns above.
- Same caveat as Java: single-instance only; move to Redis/gateway if scaled
  horizontally.

## Request logging & headers

- Request ID: use `X-Request-Id` header if present else generate a UUID;
  echo back in the response header. Attach to slog context.
- One completion log line per request (INFO): method, path, status,
  durationMs, clientIp, requestId, userId. Never log tokens, Authorization
  headers, or PII beyond what the existing line includes.
- Security headers (mirror Java):
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: no-referrer`
  - `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=()`
  - CSP: `default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'`
  - HSTS: `includeSubDomains`, max-age 31536000
  - `X-Content-Type-Options: nosniff` (Go default for Content-Type-less; set explicitly)
- Body limit: 2 MiB via `http.MaxBytesReader`.

## Audit

- Table `audit_events`, inserted in its **own** transaction (best-effort;
  failure is logged, never fails the business write).
- Fields: actor_user_id, action, entity_type, entity_id, metadata_json
  (jsonb), ip_address, user_agent.
- Actions/entity types must match Java:
  - `AUTH_LOGIN`/`AUTH_REFRESH`/`AUTH_LOGOUT_ALL` on `USER`
  - `USER_DELETED` on `USER`
  - `OUTLET_CREATED`/`OUTLET_UPDATED`/`OUTLET_GEOFENCE_UPDATED`/`OUTLET_DELETED` on `OUTLET`
  - `OUTLET_MEMBER_INVITED`/`OUTLET_INVITE_ACCEPTED`/`OUTLET_INVITE_REJECTED`/
    `OUTLET_MEMBERSHIP_REMOVED`/`OUTLET_MEMBERSHIP_LEFT`/
    `OUTLET_MEMBERSHIP_DISPLAY_NAME_UPDATED` on `OUTLET_MEMBERSHIP`
  - `ATTENDANCE_CREATED`/`ATTENDANCE_UPDATED`/`ATTENDANCE_DELETED` on `ATTENDANCE_ENTRY`
  - `SALARY_REPORT_GENERATED`/`SALARY_REPORT_EXCEL_GENERATED` on `OUTLET`
- Metadata keys per action as in Java services (e.g. `{email}` for login,
  `{outletId,userId,type,mode}` for attendance, `{employeeUserId,startTime,
  endTime,timezone,format}` for reports).

## Metrics

`prometheus/client_golang` counters with identical names/tags:

- `auth.login.success`, `auth.refresh.success`, `auth.logout.success`,
  `auth.logout_all.success`
- `user.deleted`
- `outlet.created`, `outlet.updated`, `outlet.deleted`
- `outlet.geofence.updated{enabled=true|false}`
- `outlet.membership.invited`, `.accepted`, `.rejected`, `.removed`, `.left`,
  `.display_name.updated`
- `attendance.created{mode=self|managed}`, `attendance.updated`,
  `attendance.deleted`, `attendance.geofence.rejected{outletId}`
- `report.salary.generated{format=json|xlsx}`

Exposed at `/metrics`, gated by `Authorization: Bearer <token>` matching
`PROMETHEUS_BEARER_TOKEN` (constant-time compare). Also export standard Go
runtime + process collectors.

## Health

- `GET /healthz` — liveness: 200 `{"status":"UP"}` (always when serving).
- `GET /readyz` — readiness: checks DB ping and Firebase (SDK init state);
  200 `{"status":"UP"}` or 503 `{"status":"DOWN","checks":{...}}`.
- `GET /info` — optional; `{"app":{"name":"delta","api-version":"v1"}}`.

## API docs

- Hand-maintained `go/openapi.yaml` describing every `/api/v1/*` endpoint,
  schemas, error responses, and the bearer security scheme.
- Served at `/docs` with embedded Swagger UI (embed the `swagger-ui` dist
  assets via `go:embed`; `/docs/openapi.yaml` serves the spec).
- `openapi.yaml` is the contract oracle: contract tests assert the runtime
  matches it.

## Config / environment

Replace Java properties with Go env vars (compose + `.env.example` updated):

| Env var | Default | Notes |
| --- | --- | --- |
| `PORT` | 8080 | HTTP listen addr |
| `DATABASE_URL` | postgres://postgres:postgres@localhost:5432/delta | replaces JDBC `DATASOURCE_URL` |
| `AUTO_MIGRATE` | true | run migrations on boot |
| `JWT_SECRET` | (no safe default; required in prod) | HS256 key |
| `JWT_ACCESS_TOKEN_TTL` | 900000 ms (15m) | |
| `JWT_REFRESH_TOKEN_TTL` | 2592000000 ms (30d) | |
| `JWT_REFRESH_CLEANUP_INTERVAL` | 86400000 ms (24h) | |
| `JWT_REFRESH_REVOKED_RETENTION` | 604800000 ms (7d) | |
| `FIREBASE_SERVICE_ACCOUNT_PATH` | firebase/service-account.json | |
| `PROMETHEUS_BEARER_TOKEN` | (empty) | /metrics gate |
| `TRUST_PROXY_HEADERS` | true | client-IP resolution |
| `LOG_LEVEL` | info | slog level |
| `LOG_FORMAT` | text | text or json |

Dropped (Java-only): `JAVA_OPTS`, `JPA_*`, `FLYWAY_*`, `springdoc.*`,
`management.*`.

## Docker / Compose / monitoring

- `go/Dockerfile`: multi-stage — build `golang:1.25-alpine`, runtime
  `alpine:3.x` with `ca-certificates`, `tzdata`, non-root user, minimal size.
  Include the `-trimpath`/`-ldflags` build; default env passthrough.
- `docker-compose.yml`: `app` service builds from `go/`; healthcheck uses
  `/healthz` (and readiness via `/readyz`); env vars per table above;
  `FIREBASE_SERVICE_ACCOUNT_PATH=/app/firebase/service-account.json` mounted
  from `./firebase`. Postgres, Prometheus, Grafana services unchanged except
  where noted.
- `monitoring/prometheus/prometheus.yml`: `metrics_path: /metrics` (bearer
  token file unchanged).
- Grafana dashboard: unchanged (metric names preserved).
- `loadtest/`: unchanged (k6 mints HS256 tokens with `sub` = user UUID).

## Testing

- **Unit (table-driven)**: salary pairing/math, haversine geofence, pagination
  param parsing, rate limiter windowing, JWT generate/parse, refresh-token
  hashing, Excel filename + formula-injection sanitization, validation
  messages, email normalization.
- **Integration (testcontainers-go, real Postgres)**: mirror the Java suite —
  auth token flow (login/refresh/logout/logout-all), outlet + membership
  lifecycle (create/get/update/geofence/invite/accept/reject/leave/remove/
  display-name/delete), attendance + geofence enforcement, salary report JSON +
  Excel, account deletion, rate limiting, readiness.
- **Contract tests**: for every endpoint, assert exact JSON field names and
  values, pagination envelope, and error code/message on failure. Compare
  against `openapi.yaml` where feasible.
- Firebase in tests: stub the SDK verify/delete behind a small interface
  (auth service), since real Firebase is unavailable in CI.
- Commands: `make test` (go test ./...), `make lint` (golangci-lint if
  adopted; at minimum `go vet ./...` + `gofmt`), `make build` (go build
  ./cmd/delta).

## Deliverables / phases

1. **Scaffold**: `go/` module, config, DB pool + migrations, healthz/readyz,
   metrics endpoint, slog logging, error envelope, security headers, Dockerfile,
   Makefile, compose/prometheus updates, `go/README.md`.
2. **Auth**: Firebase verify/delete, JWT, refresh tokens, login/refresh/
   logout/logout-all, account deletion, audit + metrics hooks.
3. **Outlets**: full outlet + membership domain.
4. **Attendance**: entries + geofence.
5. **Reports**: salary JSON + Excel.
6. **Cross-cutting + docs**: rate limiting, audit service wiring, openapi.yaml
   + /docs, contract tests, SETUP/README/STRUCTURE/AGENTS updates, loadtest
   verification against the Go app.

Each phase lands with its unit + integration tests; `make test` and
`make lint` must pass before moving on.

## Verification

- `go test ./...` green; `go vet ./...` clean.
- k6 `smoke.js`/`capacity.js`/`rate-limit.js` run against the Go app with
  unchanged seeds.
- Contract tests assert endpoint parity against the Java DTO/controller
  shapes.
- Compose stack boots; Prometheus scrapes `/metrics`; Grafana dashboard shows
  data.

## Out of scope

- Java code deletion (decided at merge time on `main`).
- Horizontal scaling / Redis rate limiting.
- Changing the `/api/v1` contract.
