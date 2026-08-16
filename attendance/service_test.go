package attendance

import (
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/coderGtm/delta/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// decimalPtr parses s into a decimal pointer for use in request payloads.
func decimalPtr(s string) *decimal.Decimal {
	d, err := decimal.Parse([]byte(s))
	if err != nil {
		panic(err)
	}
	return d
}

// mustUUID parses s into a UUID, panicking on failure.
func mustUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

// assertAPIError checks that err is an API error with the given status code
// and message.
func assertAPIError(t *testing.T, err error, wantStatus int, wantMsg string) {
	t.Helper()
	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected API error, got %v", err)
	}
	if apiErr.Status != wantStatus {
		t.Errorf("status = %d, want %d", apiErr.Status, wantStatus)
	}
	if apiErr.Message != wantMsg {
		t.Errorf("message = %q, want %q", apiErr.Message, wantMsg)
	}
}

func TestValidateCreateOwn(t *testing.T) {
	t.Run("all missing", func(t *testing.T) {
		err := validateCreateOwn(CreateOwnRequest{})
		assertAPIError(t, err, http.StatusBadRequest,
			"Attendance type is required, Latitude is required, Longitude is required")
	})
	t.Run("valid", func(t *testing.T) {
		if err := validateCreateOwn(CreateOwnRequest{Type: EntryTypeClockIn, Latitude: decimalPtr("12.3"), Longitude: decimalPtr("-4.5")}); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
	t.Run("latitude too large", func(t *testing.T) {
		err := validateCreateOwn(CreateOwnRequest{Type: EntryTypeClockOut, Latitude: decimalPtr("200"), Longitude: decimalPtr("0")})
		assertAPIError(t, err, http.StatusBadRequest, "Latitude must be less than or equal to 90")
	})
	t.Run("latitude too small", func(t *testing.T) {
		err := validateCreateOwn(CreateOwnRequest{Type: EntryTypeClockIn, Latitude: decimalPtr("-91"), Longitude: decimalPtr("0")})
		assertAPIError(t, err, http.StatusBadRequest, "Latitude must be greater than or equal to -90")
	})
	t.Run("longitude bound", func(t *testing.T) {
		err := validateCreateOwn(CreateOwnRequest{Type: EntryTypeClockIn, Latitude: decimalPtr("0"), Longitude: decimalPtr("181")})
		assertAPIError(t, err, http.StatusBadRequest, "Longitude must be less than or equal to 180")
	})
	t.Run("unsupported type", func(t *testing.T) {
		err := validateCreateOwn(CreateOwnRequest{Type: "PAUSE", Latitude: decimalPtr("0"), Longitude: decimalPtr("0")})
		assertAPIError(t, err, http.StatusBadRequest, "Unsupported attendance type: PAUSE")
	})
}

func TestValidateManage(t *testing.T) {
	t.Run("all missing", func(t *testing.T) {
		err := validateManage(ManageRequest{})
		assertAPIError(t, err, http.StatusBadRequest,
			"User ID is required, Attendance type is required, Entry time is required, Latitude is required, Longitude is required")
	})
	t.Run("bound and missing fields", func(t *testing.T) {
		err := validateManage(ManageRequest{Type: EntryTypeClockIn, Latitude: decimalPtr("200")})
		assertAPIError(t, err, http.StatusBadRequest,
			"User ID is required, Entry time is required, Latitude must be less than or equal to 90, Longitude is required")
	})
	t.Run("valid", func(t *testing.T) {
		uid := mustUUID("01234567-89ab-cdef-0123-456789abcdef")
		req := ManageRequest{
			UserID:    &uid,
			Type:      EntryTypeClockOut,
			EntryTime: time.Now(),
			Latitude:  decimalPtr("12.3"),
			Longitude: decimalPtr("-4.5"),
		}
		if err := validateManage(req); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
	t.Run("unsupported type", func(t *testing.T) {
		err := validateManage(ManageRequest{Type: "PAUSE", Latitude: decimalPtr("0"), Longitude: decimalPtr("0")})
		assertAPIError(t, err, http.StatusBadRequest, "Unsupported attendance type: PAUSE")
	})
}

func TestValidateUpdate(t *testing.T) {
	t.Run("missing fields", func(t *testing.T) {
		err := validateUpdate(UpdateRequest{Type: EntryTypeClockIn})
		assertAPIError(t, err, http.StatusBadRequest,
			"Entry time is required, Latitude is required, Longitude is required")
	})
	t.Run("all missing", func(t *testing.T) {
		err := validateUpdate(UpdateRequest{})
		assertAPIError(t, err, http.StatusBadRequest,
			"Attendance type is required, Entry time is required, Latitude is required, Longitude is required")
	})
	t.Run("valid", func(t *testing.T) {
		req := UpdateRequest{
			Type:      EntryTypeClockIn,
			EntryTime: time.Now(),
			Latitude:  decimalPtr("12.3"),
			Longitude: decimalPtr("-4.5"),
		}
		if err := validateUpdate(req); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
	t.Run("unsupported type", func(t *testing.T) {
		err := validateUpdate(UpdateRequest{Type: "PAUSE", Latitude: decimalPtr("0"), Longitude: decimalPtr("0")})
		assertAPIError(t, err, http.StatusBadRequest, "Unsupported attendance type: PAUSE")
	})
}

func TestRequireRoleGuards(t *testing.T) {
	if err := requireEmployeeRole("EMPLOYEE"); err != nil {
		t.Errorf("employee role should pass, got %v", err)
	}
	err := requireEmployeeRole("OWNER")
	assertAPIError(t, err, http.StatusForbidden, "Only accepted employees can create their own attendance entries")

	if err := requireOwnerRole("OWNER"); err != nil {
		t.Errorf("owner role should pass, got %v", err)
	}
	err = requireOwnerRole("EMPLOYEE")
	assertAPIError(t, err, http.StatusForbidden, "Only outlet owners can perform this action")
}

func TestEnforceGeofence(t *testing.T) {
	outletID := mustUUID("01234567-89ab-cdef-0123-456789abcdef")
	outlet := &db.Outlet{
		ID:              pgtype.UUID{Bytes: outletID, Valid: true},
		GeofenceEnabled: true,
		Latitude:        pgtype.Numeric{Int: big.NewInt(0), Exp: -7, Valid: true},
		Longitude:       pgtype.Numeric{Int: big.NewInt(0), Exp: -7, Valid: true},
		RadiusMeters:    100,
	}
	svc := &Service{Metrics: metrics.NewRegistry()}

	if err := svc.enforceGeofence(outlet, decimalPtr("0.0005"), decimalPtr("0")); err != nil {
		t.Errorf("point ~56 m north should be inside a 100 m radius, got %v", err)
	}

	err := svc.enforceGeofence(outlet, decimalPtr("0.01"), decimalPtr("0"))
	assertAPIError(t, err, http.StatusForbidden, "Attendance location is outside the outlet geofence")

	disabled := &db.Outlet{GeofenceEnabled: false}
	if err := svc.enforceGeofence(disabled, decimalPtr("5"), decimalPtr("100")); err != nil {
		t.Errorf("disabled geofence should allow any point, got %v", err)
	}
}

func TestToEntryResponse(t *testing.T) {
	entry := &db.AttendanceEntry{
		ID:        pgtype.UUID{Bytes: mustUUID("01234567-89ab-cdef-0123-456789abcdef"), Valid: true},
		OutletID:  pgtype.UUID{Bytes: mustUUID("12345678-9abc-def0-1234-56789abcdef0"), Valid: true},
		UserID:    pgtype.UUID{Bytes: mustUUID("23456789-abcd-ef01-2345-6789abcdef01"), Valid: true},
		Type:      "CLOCK_IN",
		EntryTime: pgtype.Timestamptz{Time: time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC), Valid: true},
		Latitude:  pgtype.Numeric{Int: big.NewInt(407128000), Exp: -7, Valid: true},
		Longitude: pgtype.Numeric{Int: big.NewInt(-741050000), Exp: -7, Valid: true},
		CreatedBy: pgtype.UUID{Bytes: mustUUID("34567890-abcd-ef01-2345-6789abcdef02"), Valid: true},
		UpdatedBy: pgtype.UUID{},
		CreatedAt: pgtype.Timestamptz{Time: time.Date(2026, 8, 15, 9, 0, 1, 0, time.UTC), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: time.Date(2026, 8, 15, 9, 0, 2, 0, time.UTC), Valid: true},
	}
	creator := mustUUID("34567890-abcd-ef01-2345-6789abcdef02")

	t.Run("null user email", func(t *testing.T) {
		user := &db.User{ID: entry.UserID, Name: "Ada Lovelace", Email: pgtype.Text{}}
		resp, err := (&Service{}).toEntryResponse(entry, user, "Ada at HQ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		b, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := string(b)
		assertFieldOrder(t, got)
		assertContains(t, got, `"latitude":40.7128000`)
		assertContains(t, got, `"longitude":-74.1050000`)
		assertContains(t, got, `"userEmail":null`)
		assertContains(t, got, `"createdByUserId":"`+creator.String()+`"`)
		assertContains(t, got, `"updatedByUserId":null`)
		assertContains(t, got, `"displayName":"Ada at HQ"`)
		assertContains(t, got, `"userName":"Ada Lovelace"`)
	})

	t.Run("present user email", func(t *testing.T) {
		user := &db.User{ID: entry.UserID, Name: "Ada Lovelace", Email: pgtype.Text{String: "ada@example.com", Valid: true}}
		resp, err := (&Service{}).toEntryResponse(entry, user, "Ada at HQ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		b, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertContains(t, string(b), `"userEmail":"ada@example.com"`)
	})
}

// assertFieldOrder checks that the JSON keys of an attendance response appear
// in the documented field order.
func assertFieldOrder(t *testing.T, got string) {
	t.Helper()
	order := []string{
		`"id"`, `"outletId"`, `"userId"`, `"userName"`, `"userEmail"`, `"displayName"`,
		`"type"`, `"entryTime"`, `"latitude"`, `"longitude"`,
		`"createdByUserId"`, `"updatedByUserId"`, `"createdAt"`, `"updatedAt"`,
	}
	last := -1
	for _, key := range order {
		idx := strings.Index(got, key)
		if idx < 0 {
			t.Errorf("missing key %s in %s", key, got)
			continue
		}
		if idx <= last {
			t.Errorf("key %s out of order in %s", key, got)
		}
		last = idx
	}
}

// assertContains fails the test when got does not contain want.
func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("missing %s in %s", want, got)
	}
}

func TestNewServiceDefaultClock(t *testing.T) {
	svc := NewService(nil, nil, nil, metrics.NewRegistry())
	if svc.Clock == nil {
		t.Error("NewService with a nil clock should fall back to the system clock")
	}
}

func TestSortClause(t *testing.T) {
	p := httpapi.PageParams{Sort: []httpapi.SortOrder{{Field: "entryTime", Desc: true}}, Sorted: true}
	got, _ := p.OrderClause(attendanceSortable)
	if got != " ORDER BY entry_time DESC" {
		t.Errorf("order = %q, want %q", got, " ORDER BY entry_time DESC")
	}

	unknown := httpapi.PageParams{Sort: []httpapi.SortOrder{{Field: "latitude", Desc: true}}, Sorted: true}
	got, _ = unknown.OrderClause(attendanceSortable)
	if got != "" {
		t.Errorf("unknown sort field should yield no clause, got %q", got)
	}
}
