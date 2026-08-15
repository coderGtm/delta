// Package report_test contains integration tests for the report package that
// run against a real PostgreSQL container.
package report_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/coderGtm/delta/attendance"
	"github.com/coderGtm/delta/audit"
	"github.com/coderGtm/delta/contract"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/metrics"
	"github.com/coderGtm/delta/outlet"
	"github.com/coderGtm/delta/report"
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

// TestSalaryReportAgainstDatabase seeds attendance pairs across two days and
// verifies the daily grouping, totals, the inclusion of a zero-activity day,
// and non-empty Excel export bytes.
func TestSalaryReportAgainstDatabase(t *testing.T) {
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
	mem, err := outletSvc.InviteMember(ctx, ownerID, outletID, "employee@example.com", "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if _, err := outletSvc.AcceptInvite(ctx, empID, mem.MembershipID, "127.0.0.1", "test"); err != nil {
		t.Fatalf("accept: %v", err)
	}

	seedPair := func(in, out time.Time) {
		t.Helper()
		if _, err := attSvc.CreateManaged(ctx, ownerID, outletID, attendance.ManageRequest{
			UserID:    &empID,
			Type:      attendance.EntryTypeClockIn,
			EntryTime: in,
			Latitude:  mustDec(t, "40.7128"),
			Longitude: mustDec(t, "-74.0060"),
		}, "127.0.0.1", "test"); err != nil {
			t.Fatalf("seed clock-in: %v", err)
		}
		if _, err := attSvc.CreateManaged(ctx, ownerID, outletID, attendance.ManageRequest{
			UserID:    &empID,
			Type:      attendance.EntryTypeClockOut,
			EntryTime: out,
			Latitude:  mustDec(t, "40.7128"),
			Longitude: mustDec(t, "-74.0060"),
		}, "127.0.0.1", "test"); err != nil {
			t.Fatalf("seed clock-out: %v", err)
		}
	}
	seedPair(time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 10, 17, 0, 0, 0, time.UTC))
	seedPair(time.Date(2026, 8, 11, 9, 0, 0, 0, time.UTC), time.Date(2026, 8, 11, 13, 0, 0, 0, time.UTC))

	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 12, 23, 59, 59, 999999999, time.UTC)
	rate := mustDec(t, "20")

	rep, err := reportSvc.Calculate(ctx, ownerID, outletID, empID, start, end, "UTC", rate, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("calculate: %v", err)
	}
	if rep.OutletName != "HQ" || rep.UserName != "Employee User" || rep.DisplayName != "Employee User" || rep.Timezone != "UTC" {
		t.Errorf("report header = %+v", rep)
	}
	if rep.TotalHours.Format(2) != "12.00" || rep.TotalSalary.Format(2) != "240.00" {
		t.Errorf("report totals = %s / %s, want 12.00 / 240.00", rep.TotalHours.Format(2), rep.TotalSalary.Format(2))
	}
	if len(rep.Days) != 3 {
		t.Fatalf("report days = %d, want 3 (including a zero-activity day)", len(rep.Days))
	}
	day0 := rep.Days[0]
	if day0.Date != "2026-08-10" || len(day0.Pairs) != 1 || day0.TotalHours.Format(2) != "8.00" || day0.Salary.Format(2) != "160.00" {
		t.Errorf("day 1 = %+v", day0)
	}
	day1 := rep.Days[1]
	if day1.Date != "2026-08-11" || len(day1.Pairs) != 1 || day1.TotalHours.Format(2) != "4.00" || day1.Salary.Format(2) != "80.00" {
		t.Errorf("day 2 = %+v", day1)
	}
	day2 := rep.Days[2]
	if day2.Date != "2026-08-12" || len(day2.Pairs) != 0 || day2.TotalHours.Format(2) != "0.00" || day2.Salary.Format(2) != "0.00" {
		t.Errorf("zero-activity day = %+v", day2)
	}

	data, err := reportSvc.ExportExcel(ctx, ownerID, outletID, empID, start, end, "UTC", rate, "127.0.0.1", "test")
	if err != nil {
		t.Fatalf("export excel: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("excel export is empty")
	}
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		t.Errorf("excel export does not look like a ZIP archive (first bytes % x)", data[:4])
	}
}
