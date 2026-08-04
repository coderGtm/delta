# AGENTS.md

Guidance for coding agents working in this repository.

## Project summary

`delta` is a Spring Boot employee attendance backend.

Stack:

- Java 25
- Spring Boot 4
- Spring Security
- Spring Data JPA
- PostgreSQL
- Flyway
- Firebase Admin SDK
- Local JWT access tokens and refresh tokens
- Docker / Docker Compose

Current API version prefix:

```text
/api/v1
```

## API documentation

- API docs are generated at runtime by `springdoc-openapi` (Swagger UI at `/docs`, spec at `/docs/openapi.yaml`).
- Do not maintain a hand-written OpenAPI spec; the document is derived from controllers and DTOs.
- Controller and DTO changes are reflected in the docs automatically.

## Important commands

Run tests:

```bash
./gradlew test
```

Build jar:

```bash
./gradlew bootJar
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

## Architectural rules

- Keep controllers thin.
- Put business logic in services.
- Use DTOs for request/response payloads.
- Do not expose JPA entities directly from controllers.
- Use repositories only for persistence/query concerns.
- Keep changes minimal and consistent with existing style.
- Add Javadocs/comments for new public classes and non-obvious behavior.
- Prefer enums over lookup tables unless there is a strong reason otherwise.

## Persistence rules

- Database schema is managed by Flyway migrations in:

```text
src/main/resources/db/migration
```

- Do not rely on Hibernate to create/update production schema.
- `spring.jpa.hibernate.ddl-auto=validate` is expected.
- When changing entities, add a new Flyway migration.
- Existing base auditing provides `createdAt` / `updatedAt`.
- Attendance also has `createdBy` / `updatedBy` using the authenticated user ID.

## Authentication/security context

- Authenticated principal is the local JPA `User` entity.
- JWT filter loads only active users via `UserRepository.findByIdAndDeletedAtIsNull(...)`.
- Public auth endpoints:
  - `/api/v1/auth/login`
  - `/api/v1/auth/refresh`
  - `/api/v1/auth/logout`
- Most other endpoints require authentication.

## Domain rules

### Outlets and memberships

- Creating an outlet auto-creates an accepted `OWNER` membership for the creator.
- Owners can invite existing users by email.
- Invited employees can accept or reject invites.
- Re-invite reuses existing membership and resets status to `INVITED`.
- Membership removal is soft removal with `removedAt` / `removedBy`.
- Removed memberships should disappear from active access/invite/listing checks.
- Historical attendance must remain valid after membership removal.
- Owner memberships cannot be removed through the current remove endpoint.

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

## Pagination

Collection endpoints should use DB-level pagination with Spring Data `Pageable` and return `PageResponse<T>`.

Shared helpers:

```text
common/dto/PageResponse.java
common/util/PaginationUtils.java
config/WebPaginationConfig.java
```

## Cross-cutting concerns

### Auditing

Use `AuditService` for business events worth investigating later:

```text
common/audit/service/AuditService.java
```

Examples:

- auth login/refresh/logout-all
- outlet created/updated/geofence toggled
- membership invited/accepted/rejected/removed
- attendance created/updated/deleted
- report generated/exported

### Metrics

Use `ApplicationMetrics` for business counters:

```text
common/metrics/ApplicationMetrics.java
```

Metrics are exposed via Prometheus format at:

```text
/actuator/prometheus
```

Keep that endpoint private to monitoring systems.

### Request logging

`RequestLoggingFilter` adds request IDs and request completion logs.

Do not log:

- access tokens
- refresh tokens
- Firebase ID tokens
- full Authorization headers
- unnecessary PII

### Rate limiting

`RateLimitingFilter` provides in-memory single-instance rate limiting.

For horizontally scaled deployments, move rate limiting to Redis or the gateway/ingress layer.

## Testing expectations

For behavior changes:

- Add/update service tests for business logic.
- Add controller/integration tests for auth, authorization, validation, and endpoint response shape when relevant.
- Run `./gradlew test` before finalizing.

## Docker/deployment notes

- Docker build uses Gradle wrapper.
- Wrapper timeout/retries are configured in `gradle/wrapper/gradle-wrapper.properties`.
- Runtime image runs as non-root user.
- Local secrets should be in `.env` and `firebase/service-account.json`; both are ignored by Git.
- Production secrets should come from the deployment platform secret manager.

## Note

Once the changes are implemented, determine whether any documentation in this file, README.md, STRUCTURE.md or SETUP.md is required and make it. Always inform the user explicitly for any of these changes and highlight them in the response chat.
