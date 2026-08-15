// Package attendance_test contains integration tests for the attendance
// package that run against a real PostgreSQL container.
package attendance_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coderGtm/delta/attendance"
	"github.com/coderGtm/delta/audit"
	"github.com/coderGtm/delta/contract"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/coderGtm/delta/metrics"
	"github.com/coderGtm/delta/outlet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func intPtr(v int) *int { return &v }

func mustDec(t *testing.T, s string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.Parse([]byte(s))
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return d
}

func toUUID(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

func createUser(t *testing.T, store *db.Store, uid, name, email string) uuid.UUID {
	t.Helper()
	user, err := store.Querier().CreateUser(context.Background(), db.CreateUserParams{
		AuthUid: pgtype.Text{String: uid, Valid: true},
		Name:    name,
		Email:   pgtype.Text{String: email, Valid: true},
	})
	if err != nil {
		t.Fatalf("create user %s: %v", name, err)
	}
	return toUUID(user.ID)
}

func isAPIError(err error, code string) bool {
	var apiErr *httpapi.APIError
	return errors.As(err, &apiErr) && apiErr.Code == code
}

// addEmployee invites and accepts an employee membership for empID, returning
// the membership id.
func addEmployee(t *testing.T, store *db.Store, outletSvc *outlet.Service, ownerID, outletID, empID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	mem, err := outletSvc.InviteMember(ctx, ownerID, outletID, email, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite %s: %v", email, err)
	}
	accepted, err := outletSvc.AcceptInvite(ctx, empID, mem.MembershipID, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("accept %s: %v", email, err)
	}
	if accepted.Status != "ACCEPTED" {
		t.Fatalf("accept %s = %+v, want ACCEPTED", email, accepted)
	}
	return mem.MembershipID
}

// TestAttendanceLifecycle drives the self-service and owner-managed attendance
// paths: create, list with role-based filtering, get, update, and delete.
func TestAttendanceLifecycle(t *testing.T) {
	store := contract.Setup(t)
	ctx := context.Background()
	registry := metrics.NewRegistry()
	recorder := audit.NewRecorder(store)
	outletSvc := outlet.NewService(store, recorder, registry)
	attSvc := attendance.NewService(store, nil, recorder, registry)

	ownerID := createUser(t, store, "owner-uid", "Owner User", "owner@example.com")
	empID := createUser(t, store, "emp-uid", "Employee User", "employee@example.com")
	secondID := createUser(t, store, "second-uid", "Second Employee", "second@example.com")

	out, err := outletSvc.CreateOutlet(ctx, ownerID, outlet.CreateOutletRequest{
		Name:         "HQ",
		Latitude:     mustDec(t, "40.7128"),
		Longitude:    mustDec(t, "-74.0060"),
		RadiusMeters: intPtr(100),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	outletID := out.ID
	addEmployee(t, store, outletSvc, ownerID, outletID, empID, "employee@example.com")
	addEmployee(t, store, outletSvc, ownerID, outletID, secondID, "second@example.com")

	own, err := attSvc.CreateOwn(ctx, empID, outletID, attendance.CreateOwnRequest{
		Type:      attendance.EntryTypeClockIn,
		Latitude:  mustDec(t, "40.7128"),
		Longitude: mustDec(t, "-74.0060"),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create own: %v", err)
	}
	if own.Type != attendance.EntryTypeClockIn || own.UserID != empID || own.CreatedByUserID == nil || *own.CreatedByUserID != empID {
		t.Errorf("own entry = %+v, want CLOCK_IN for the employee created by the employee", own)
	}
	if own.DisplayName != "Employee User" {
		t.Errorf("own entry display name = %q, want account name", own.DisplayName)
	}
	if d := time.Since(own.EntryTime); d < -time.Minute || d > 5*time.Minute {
		t.Errorf("own entry time = %s, not near server now", own.EntryTime)
	}
	ownID := own.ID

	managedTime := time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC)
	managed, err := attSvc.CreateManaged(ctx, ownerID, outletID, attendance.ManageRequest{
		UserID:    &empID,
		Type:      attendance.EntryTypeClockOut,
		EntryTime: managedTime,
		Latitude:  mustDec(t, "40.7128"),
		Longitude: mustDec(t, "-74.0060"),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create managed: %v", err)
	}
	if managed.Type != attendance.EntryTypeClockOut || managed.UserID != empID || managed.CreatedByUserID == nil || *managed.CreatedByUserID != ownerID {
		t.Errorf("managed entry = %+v, want CLOCK_OUT created by the owner", managed)
	}

	page, err := attSvc.List(ctx, empID, outletID, nil, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("employee list: %v", err)
	}
	if len(page.Content) != 2 {
		t.Errorf("employee list = %d entries, want 2", len(page.Content))
	}

	page, err = attSvc.List(ctx, secondID, outletID, nil, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("second employee list: %v", err)
	}
	if len(page.Content) != 0 {
		t.Errorf("second employee list = %d entries, want 0", len(page.Content))
	}

	page, err = attSvc.List(ctx, ownerID, outletID, nil, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(page.Content) != 2 {
		t.Errorf("owner list = %d entries, want 2", len(page.Content))
	}

	if _, err := attSvc.List(ctx, empID, outletID, &secondID, httpapi.PageParams{Page: 0, Size: 20}); !isAPIError(err, "FORBIDDEN") {
		t.Errorf("employee filtered list err = %v, want FORBIDDEN", err)
	}

	got, err := attSvc.Get(ctx, ownerID, outletID, ownID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != ownID || got.Type != attendance.EntryTypeClockIn {
		t.Errorf("get = %+v, want the created clock-in entry", got)
	}

	updated, err := attSvc.Update(ctx, ownerID, outletID, ownID, attendance.UpdateRequest{
		Type:      attendance.EntryTypeClockOut,
		EntryTime: managedTime.Add(-time.Hour),
		Latitude:  mustDec(t, "40.7128"),
		Longitude: mustDec(t, "-74.0060"),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Type != attendance.EntryTypeClockOut || updated.UpdatedByUserID == nil || *updated.UpdatedByUserID != ownerID {
		t.Errorf("updated entry = %+v, want CLOCK_OUT updated by the owner", updated)
	}

	if err := attSvc.Delete(ctx, ownerID, outletID, ownID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := attSvc.Get(ctx, ownerID, outletID, ownID); !isAPIError(err, "NOT_FOUND") {
		t.Errorf("get after delete err = %v, want NOT_FOUND", err)
	}
}

// TestGeofenceRejectsFarClockIn verifies that attendance writes are rejected
// when the outlet geofence is enabled and the coordinates fall outside the
// radius.
func TestGeofenceRejectsFarClockIn(t *testing.T) {
	store := contract.Setup(t)
	ctx := context.Background()
	registry := metrics.NewRegistry()
	recorder := audit.NewRecorder(store)
	outletSvc := outlet.NewService(store, recorder, registry)
	attSvc := attendance.NewService(store, nil, recorder, registry)

	ownerID := createUser(t, store, "owner-uid", "Owner User", "owner@example.com")
	empID := createUser(t, store, "emp-uid", "Employee User", "employee@example.com")

	out, err := outletSvc.CreateOutlet(ctx, ownerID, outlet.CreateOutletRequest{
		Name:         "Fenced",
		Latitude:     mustDec(t, "0"),
		Longitude:    mustDec(t, "0"),
		RadiusMeters: intPtr(1000),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("create geofenced outlet: %v", err)
	}
	outletID := out.ID
	if _, err := outletSvc.UpdateGeofence(ctx, ownerID, outletID, true, "127.0.0.1", "test"); err != nil {
		t.Fatalf("enable geofence: %v", err)
	}
	addEmployee(t, store, outletSvc, ownerID, outletID, empID, "employee@example.com")

	_, err = attSvc.CreateOwn(ctx, empID, outletID, attendance.CreateOwnRequest{
		Type:      attendance.EntryTypeClockIn,
		Latitude:  mustDec(t, "10"),
		Longitude: mustDec(t, "10"),
	}, "127.0.0.1", "test")
	if !isAPIError(err, "FORBIDDEN") {
		t.Fatalf("far clock-in err = %v, want FORBIDDEN", err)
	}
	if err != nil && err.Error() != "FORBIDDEN: Attendance location is outside the outlet geofence" {
		t.Errorf("far clock-in err message = %q, want the geofence message", err)
	}

	inside, err := attSvc.CreateOwn(ctx, empID, outletID, attendance.CreateOwnRequest{
		Type:      attendance.EntryTypeClockIn,
		Latitude:  mustDec(t, "0"),
		Longitude: mustDec(t, "0"),
	}, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("clock-in at center: %v", err)
	}
	if inside.UserID != empID {
		t.Errorf("center clock-in = %+v, want the employee's entry", inside)
	}
}
