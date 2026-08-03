# Project Structure

Root package:

```text
com.coderGtm.delta
```

## Application entrypoint

```text
DeltaApplication.java
```

Enables:
- Spring Boot application bootstrapping
- JPA auditing
- scheduled refresh-token cleanup

## Feature packages

### `auth`

Authentication and token lifecycle.

```text
auth
 ├── config
 │    └── SecurityConfig
 ├── controller
 │    └── AuthController
 ├── dto
 │    ├── FirebaseUserInfo
 │    ├── LoginRequest
 │    ├── LoginResponse
 │    ├── LogoutRequest
 │    ├── RefreshTokenRequest
 │    └── RefreshTokenResponse
 ├── entity
 │    └── RefreshToken
 ├── filter
 │    └── JwtAuthenticationFilter
 ├── mapper
 │    └── AuthMapper
 ├── repository
 │    └── RefreshTokenRepository
 └── service
      ├── AuthService
      ├── FirebaseService
      ├── JwtService
      └── RefreshTokenService
```

Notes:
- Firebase login issues local JWT access tokens and refresh tokens.
- JWT authentication loads active local users only.
- Refresh tokens are stored as hashes.

### `user`

Local application user domain.

```text
user
 ├── dto
 ├── User
 ├── UserRepository
 └── UserService
```

### `outlet`

Outlet management, owner/employee memberships, invitations, membership soft-removal, and geofence configuration.

```text
outlet
 ├── controller
 │    └── OutletController
 ├── dto
 │    ├── CreateOutletRequest
 │    ├── InviteOutletMemberRequest
 │    ├── OutletMembershipResponse
 │    ├── OutletResponse
 │    ├── UpdateOutletGeofenceRequest
 │    └── UpdateOutletRequest
 ├── entity
 │    ├── Outlet
 │    ├── OutletMembership
 │    ├── OutletMembershipStatus
 │    └── OutletRole
 ├── mapper
 │    └── OutletMapper
 ├── repository
 │    ├── OutletMembershipRepository
 │    └── OutletRepository
 └── service
      └── OutletService
```

Notes:
- Outlet memberships are explicit join entities.
- Membership removal is soft removal via `removedAt` / `removedBy`.
- Owners can toggle `geofenceEnabled` per outlet.

### `attendance`

Attendance entry CRUD and geofence enforcement.

```text
attendance
 ├── controller
 │    └── AttendanceController
 ├── dto
 │    ├── AttendanceEntryResponse
 │    ├── CreateAttendanceEntryRequest
 │    ├── ManageAttendanceEntryRequest
 │    └── UpdateAttendanceEntryRequest
 ├── entity
 │    ├── AttendanceEntry
 │    └── AttendanceEntryType
 ├── mapper
 │    └── AttendanceMapper
 ├── repository
 │    └── AttendanceEntryRepository
 └── service
      └── AttendanceService
```

Notes:
- Employees can create their own attendance using server UTC time only.
- Owners can manage employee attendance entries.
- Attendance keeps direct user/outlet references for historical reporting.
- If outlet geofencing is enabled, attendance writes outside the radius are rejected.

### `report`

Owner-facing attendance/payroll reports.

```text
report
 ├── controller
 │    └── ReportController
 ├── dto
 │    ├── AttendancePairReportResponse
 │    ├── DailySalaryReportResponse
 │    └── SalaryReportResponse
 └── service
      └── SalaryReportService
```

Notes:
- Owners can generate salary reports for one employee in one outlet.
- Reports accept exact `startTime`, `endTime`, `timezone`, and `hourlyRate`.
- Report rows are grouped by the selected IANA timezone.
- Reports are available as JSON and `.xlsx` Excel export.

## Shared packages

### `common`

Shared cross-cutting concerns.

```text
common
 ├── audit
 │    ├── entity
 │    │    └── AuditEvent
 │    ├── repository
 │    │    └── AuditEventRepository
 │    └── service
 │         └── AuditService
 ├── dto
 │    ├── ErrorResponse
 │    └── PageResponse
 ├── entity
 │    └── BaseEntity
 ├── exception
 │    ├── ApiException
 │    ├── BadRequestException
 │    ├── ConflictException
 │    ├── ForbiddenException
 │    ├── GlobalExceptionHandler
 │    ├── InvalidTokenException
 │    └── ResourceNotFoundException
 ├── health
 │    └── FirebaseHealthIndicator
 ├── metrics
 │    └── ApplicationMetrics
 ├── util
 │    ├── GeoUtils
 │    └── PaginationUtils
 └── web
      ├── ApiPaths
      ├── ClientIpUtils
      ├── RateLimitingFilter
      └── RequestLoggingFilter
```

### `config`

Application infrastructure configuration.

```text
config
 ├── FirebaseConfig
 ├── PersistenceConfig
 └── WebPaginationConfig
```

## Resources

```text
src/main/resources
 ├── application.properties
 └── db/migration
      └── V1__init_schema.sql
```

Notes:
- Database schema is managed by Flyway.
- Hibernate uses schema validation, not automatic schema updates.

## Containerization

```text
Dockerfile
docker-compose.yml
.dockerignore
.env.example
```

Notes:
- Multi-stage Docker build creates the Spring Boot jar inside a builder image.
- Runtime container runs as a non-root user.
- Compose starts Postgres and the app container.
