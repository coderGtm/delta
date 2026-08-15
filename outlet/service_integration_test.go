// Package outlet_test contains integration tests for the outlet package that
// run against a real PostgreSQL container.
package outlet_test

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
	"github.com/coderGtm/delta/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func intPtr(v int) *outlet.FlexInt { return (*outlet.FlexInt)(&v) }

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

// TestMembershipLifecycle drives the full membership lifecycle against the
// database: create with an auto owner membership, invite, accept, rename,
// leave, re-invite, remove, and finally outlet deletion, then verifies
// historical attendance reads still work.
func TestMembershipLifecycle(t *testing.T) {
	store := contract.Setup(t)
	ctx := context.Background()
	registry := metrics.NewRegistry()
	recorder := audit.NewRecorder(store)
	outletSvc := outlet.NewService(store, recorder, registry)
	attSvc := attendance.NewService(store, nil, recorder, registry)
	reportSvc := report.NewService(store, recorder, registry)

	ownerID := createUser(t, store, "owner-uid", "Owner User", "owner@example.com")
	empID := createUser(t, store, "emp-uid", "Employee User", "employee@example.com")

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

	ownerPage, err := outletSvc.GetMyOutlets(ctx, ownerID, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("owner mine: %v", err)
	}
	if len(ownerPage.Content) != 1 {
		t.Fatalf("owner mine = %d outlets, want 1", len(ownerPage.Content))
	}
	ownerMembership := ownerPage.Content[0]
	if ownerMembership.Role != "OWNER" || ownerMembership.Status != "ACCEPTED" || ownerMembership.DisplayName != "Owner User" {
		t.Errorf("owner membership = %+v, want OWNER/ACCEPTED with account name", ownerMembership)
	}
	if ownerMembership.Outlet.ID != outletID {
		t.Errorf("owner membership outlet id = %s, want %s", ownerMembership.Outlet.ID, outletID)
	}

	invited, err := outletSvc.InviteMember(ctx, ownerID, outletID, "employee@example.com", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if invited.Status != "INVITED" || invited.Role != "EMPLOYEE" {
		t.Errorf("invite = %+v, want INVITED/EMPLOYEE", invited)
	}
	membershipID := invited.MembershipID

	accepted, err := outletSvc.AcceptInvite(ctx, empID, membershipID, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if accepted.Status != "ACCEPTED" {
		t.Errorf("accept = %+v, want ACCEPTED", accepted)
	}

	empPage, err := outletSvc.GetMyOutlets(ctx, empID, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("employee mine: %v", err)
	}
	if len(empPage.Content) != 1 {
		t.Fatalf("employee mine = %d outlets, want 1", len(empPage.Content))
	}

	renamed, err := outletSvc.UpdateDisplayName(ctx, ownerID, outletID, membershipID, "Nick", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("update display name: %v", err)
	}
	if renamed.DisplayName != "Nick" {
		t.Errorf("display name = %q, want Nick", renamed.DisplayName)
	}

	clockIn := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	clockOut := time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC)
	if _, err := attSvc.CreateManaged(ctx, ownerID, outletID, attendance.ManageRequest{
		UserID:    &empID,
		Type:      attendance.EntryTypeClockIn,
		EntryTime: clockIn,
		Latitude:  mustDec(t, "40.7128"),
		Longitude: mustDec(t, "-74.0060"),
	}, "127.0.0.1", "test"); err != nil {
		t.Fatalf("seed clock-in: %v", err)
	}
	if _, err := attSvc.CreateManaged(ctx, ownerID, outletID, attendance.ManageRequest{
		UserID:    &empID,
		Type:      attendance.EntryTypeClockOut,
		EntryTime: clockOut,
		Latitude:  mustDec(t, "40.7128"),
		Longitude: mustDec(t, "-74.0060"),
	}, "127.0.0.1", "test"); err != nil {
		t.Fatalf("seed clock-out: %v", err)
	}

	if err := outletSvc.LeaveOutlet(ctx, empID, outletID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := outletSvc.GetOutlet(ctx, empID, outletID); !isAPIError(err, "NOT_FOUND") {
		t.Errorf("GetOutlet after leave err = %v, want NOT_FOUND", err)
	}
	page, err := attSvc.List(ctx, ownerID, outletID, nil, httpapi.PageParams{Page: 0, Size: 20})
	if err != nil {
		t.Fatalf("historical list after leave: %v", err)
	}
	if len(page.Content) != 2 {
		t.Errorf("historical list after leave = %d entries, want 2", len(page.Content))
	}
	if page.Content[0].DisplayName != "Nick" {
		t.Errorf("historical entry display name = %q, want Nick", page.Content[0].DisplayName)
	}

	reopened, err := outletSvc.InviteMember(ctx, ownerID, outletID, "employee@example.com", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("re-invite: %v", err)
	}
	if reopened.MembershipID != membershipID || reopened.Status != "INVITED" {
		t.Errorf("re-invite = %+v, want the same membership reopened as INVITED", reopened)
	}
	if _, err := outletSvc.AcceptInvite(ctx, empID, membershipID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("re-accept: %v", err)
	}

	if err := outletSvc.RemoveMembership(ctx, ownerID, outletID, membershipID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
	if err := outletSvc.RemoveMembership(ctx, ownerID, outletID, ownerMembership.MembershipID, "127.0.0.1", "test"); !isAPIError(err, "BAD_REQUEST") {
		t.Errorf("remove owner membership err = %v, want BAD_REQUEST", err)
	}

	if err := outletSvc.DeleteOutlet(ctx, ownerID, outletID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("delete outlet: %v", err)
	}
	if _, err := outletSvc.GetOutlet(ctx, ownerID, outletID); !isAPIError(err, "NOT_FOUND") {
		t.Errorf("GetOutlet after delete err = %v, want NOT_FOUND", err)
	}

	rate, err := decimal.Parse([]byte("20"))
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 10, 23, 59, 59, 999999999, time.UTC)
	rep, err := reportSvc.Calculate(ctx, ownerID, outletID, empID, start, end, "UTC", rate, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("historical report after delete: %v", err)
	}
	if rep.TotalHours.Format(2) != "8.00" || rep.DisplayName != "Nick" {
		t.Errorf("historical report = %s hours, display name %q; want 8.00 / Nick", rep.TotalHours.Format(2), rep.DisplayName)
	}
}
