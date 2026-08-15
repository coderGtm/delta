package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func dec(t *testing.T, s string) *decimal.Decimal {
	t.Helper()
	d, err := decimal.Parse([]byte(s))
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return d
}

func entry(typ string, ts time.Time) db.AttendanceEntry {
	return db.AttendanceEntry{
		ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		OutletID:  pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Type:      typ,
		EntryTime: pgtype.Timestamptz{Time: ts, Valid: true},
	}
}

func TestCompletedPairs(t *testing.T) {
	t.Run("two pairs", func(t *testing.T) {
		base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		entries := []db.AttendanceEntry{
			entry("CLOCK_IN", base.Add(9*time.Hour)),
			entry("CLOCK_OUT", base.Add(11*time.Hour)),
			entry("CLOCK_IN", base.Add(13*time.Hour)),
			entry("CLOCK_OUT", base.Add(14*time.Hour)),
		}
		pairs := CompletedPairs(entries)
		if len(pairs) != 2 {
			t.Fatalf("got %d pairs, want 2", len(pairs))
		}
		if got := pairs[0].Hours.Format(2); got != "2.00" {
			t.Errorf("pair 0 hours = %s, want 2.00", got)
		}
		if got := pairs[1].Hours.Format(2); got != "1.00" {
			t.Errorf("pair 1 hours = %s, want 1.00", got)
		}
	})

	t.Run("orphan clock out ignored", func(t *testing.T) {
		base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		entries := []db.AttendanceEntry{
			entry("CLOCK_OUT", base.Add(9*time.Hour)),
			entry("CLOCK_IN", base.Add(10*time.Hour)),
			entry("CLOCK_OUT", base.Add(11*time.Hour)),
		}
		pairs := CompletedPairs(entries)
		if len(pairs) != 1 {
			t.Fatalf("got %d pairs, want 1", len(pairs))
		}
		if got := pairs[0].Hours.Format(2); got != "1.00" {
			t.Errorf("hours = %s, want 1.00", got)
		}
	})

	t.Run("clock out before clock in pairs nothing", func(t *testing.T) {
		base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		entries := []db.AttendanceEntry{
			entry("CLOCK_OUT", base.Add(10*time.Hour)),
			entry("CLOCK_IN", base.Add(11*time.Hour)),
		}
		if pairs := CompletedPairs(entries); len(pairs) != 0 {
			t.Errorf("got %d pairs, want 0", len(pairs))
		}
	})
}

func TestHoursBetween(t *testing.T) {
	base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   time.Time
		out  time.Time
		want string
	}{
		{"two hours", base, base.Add(2 * time.Hour), "2.00"},
		{"3618 seconds rounds up", base, base.Add(3618 * time.Second), "1.01"},
		{"zero", base, base, "0.00"},
		{"ninety minutes", base, base.Add(90 * time.Minute), "1.50"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hoursBetween(tc.in, tc.out).Format(2); got != tc.want {
				t.Errorf("hoursBetween = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestValidateReportRequest(t *testing.T) {
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)

	if got, err := ValidateReportRequest(start, end, "Asia/Kolkata", dec(t, "10.5")); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	} else if got.String() != kolkata.String() {
		t.Errorf("got location %s, want %s", got, kolkata)
	}

	cases := []struct {
		name string
		from time.Time
		to   time.Time
		tz   string
		rate *decimal.Decimal
		want string
	}{
		{"both zero", time.Time{}, time.Time{}, "Asia/Kolkata", dec(t, "10.5"), "Start time and end time are required"},
		{"reversed", time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), start, "Asia/Kolkata", dec(t, "10.5"), "End time must be after start time"},
		{"reversed before timezone", time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), start, "Not/AZone", dec(t, "10.5"), "End time must be after start time"},
		{"bad timezone", start, end, "Not/AZone", dec(t, "10.5"), "Timezone must be a valid IANA timezone"},
		{"over 366 local days", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 367), "Asia/Kolkata", dec(t, "10.5"), "Salary reports can cover at most 366 local days"},
		{"nil rate", start, end, "Asia/Kolkata", nil, "Hourly rate must be greater than zero"},
		{"zero rate", start, end, "Asia/Kolkata", dec(t, "0"), "Hourly rate must be greater than zero"},
		{"negative rate", start, end, "Asia/Kolkata", dec(t, "-1.5"), "Hourly rate must be greater than zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateReportRequest(tc.from, tc.to, tc.tz, tc.rate)
			ae, ok := err.(*httpapi.APIError)
			if !ok || ae.Code != "BAD_REQUEST" || ae.Message != tc.want {
				t.Errorf("got %v, want BAD_REQUEST %q", err, tc.want)
			}
		})
	}

	if _, err := ValidateReportRequest(start, start.AddDate(0, 0, 365), "Asia/Kolkata", dec(t, "10.5")); err != nil {
		t.Errorf("exactly 366 local days rejected: %v", err)
	}
}

func TestBuildReport(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	outletID := uuid.New()
	employeeID := uuid.New()
	start := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 15, 18, 30, 0, 0, time.UTC)
	clockIn := time.Date(2026, 8, 14, 23, 0, 0, 0, time.UTC) // 04:30 local on 08-15
	clockOut := time.Date(2026, 8, 15, 1, 0, 0, 0, time.UTC) // 06:30 local on 08-15
	entries := []db.AttendanceEntry{
		entry("CLOCK_IN", clockIn),
		entry("CLOCK_OUT", clockOut),
	}
	employee := &db.User{
		ID:    pgtype.UUID{Bytes: employeeID, Valid: true},
		Name:  "Alice",
		Email: pgtype.Text{String: "alice@example.com", Valid: true},
	}

	report := buildReport(outletID, "Acme", employee, "Alice (custom)", start, end, loc, *dec(t, "10.5"), entries)
	if len(report.Days) != 2 {
		t.Fatalf("got %d days, want 2", len(report.Days))
	}
	first := report.Days[0]
	if first.Date != "2026-08-14" {
		t.Errorf("day 0 date = %s, want 2026-08-14", first.Date)
	}
	if len(first.Pairs) != 0 {
		t.Errorf("day 0 got %d pairs, want 0", len(first.Pairs))
	}
	if got := first.TotalHours.Format(2); got != "0.00" {
		t.Errorf("day 0 totalHours = %s, want 0.00", got)
	}
	second := report.Days[1]
	if second.Date != "2026-08-15" {
		t.Errorf("day 1 date = %s, want 2026-08-15", second.Date)
	}
	if len(second.Pairs) != 1 {
		t.Fatalf("day 1 got %d pairs, want 1", len(second.Pairs))
	}
	if got := second.Pairs[0].Hours.Format(2); got != "2.00" {
		t.Errorf("pair hours = %s, want 2.00", got)
	}
	if got := second.TotalHours.Format(2); got != "2.00" {
		t.Errorf("day 1 totalHours = %s, want 2.00", got)
	}
	if got := second.Salary.Format(2); got != "21.00" {
		t.Errorf("day 1 salary = %s, want 21.00", got)
	}
	if got := report.TotalHours.Format(2); got != "2.00" {
		t.Errorf("totalHours = %s, want 2.00", got)
	}
	if got := report.TotalSalary.Format(2); got != "21.00" {
		t.Errorf("totalSalary = %s, want 21.00", got)
	}
	if report.Timezone != "Asia/Kolkata" {
		t.Errorf("timezone = %s, want Asia/Kolkata", report.Timezone)
	}
	if !report.StartTime.Equal(start) {
		t.Errorf("startTime = %s, want %s", report.StartTime, start)
	}

	b, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	s := string(b)
	for _, want := range []string{`"date":"2026-08-14"`, `"totalHours":2.00`, `"hourlyRate":10.5`, `"userEmail":"alice@example.com"`} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %s in %s", want, s)
		}
	}

	deleted := &db.User{
		ID:    pgtype.UUID{Bytes: employeeID, Valid: true},
		Name:  "Alice",
		Email: pgtype.Text{},
	}
	report = buildReport(outletID, "Acme", deleted, "Alice (custom)", start, end, loc, *dec(t, "10.5"), nil)
	b, err = json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal deleted user: %v", err)
	}
	if !strings.Contains(string(b), `"userEmail":null`) {
		t.Errorf("deleted user JSON missing null email: %s", b)
	}
}

func TestBuildReportDays(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
	entries := []db.AttendanceEntry{
		entry("CLOCK_IN", base),
		entry("CLOCK_OUT", base.Add(3618*time.Second)),
	}
	employee := &db.User{
		ID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Name:  "Bob",
		Email: pgtype.Text{String: "bob@example.com", Valid: true},
	}
	report := buildReport(uuid.New(), "Acme", employee, "Bob", base, base.Add(2*time.Hour), loc, *dec(t, "10.5"), entries)
	if got := report.Days[0].TotalHours.Format(2); got != "1.01" {
		t.Errorf("totalHours = %s, want 1.01", got)
	}
	if got := report.Days[0].Salary.Format(2); got != "10.61" {
		t.Errorf("day salary = %s, want 10.61", got)
	}
	if got := report.TotalSalary.Format(2); got != "10.61" {
		t.Errorf("totalSalary = %s, want 10.61", got)
	}
}
