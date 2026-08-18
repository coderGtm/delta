// Package report provides salary report calculation from completed attendance
// pairs.
package report

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/coderGtm/delta/audit"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/coderGtm/delta/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Pair is a completed clock-in/clock-out pair.
type Pair struct {
	ClockIn  time.Time       `json:"clockIn"`
	ClockOut time.Time       `json:"clockOut"`
	Hours    decimal.Decimal `json:"hours"`
}

// Day is one calendar day of a salary report.
type Day struct {
	Date       string          `json:"date"`
	Pairs      []Pair          `json:"attendancePairs"`
	TotalHours decimal.Decimal `json:"totalHours"`
	HourlyRate decimal.Decimal `json:"hourlyRate"`
	Salary     decimal.Decimal `json:"salary"`
}

// SalaryReport is the owner-facing salary report for one employee in one
// outlet over an instant range, grouped daily by a timezone.
type SalaryReport struct {
	OutletID    uuid.UUID       `json:"outletId"`
	OutletName  string          `json:"outletName"`
	UserID      uuid.UUID       `json:"userId"`
	UserName    string          `json:"userName"`
	UserEmail   *string         `json:"userEmail"`
	DisplayName string          `json:"displayName"`
	StartTime   time.Time       `json:"startTime"`
	EndTime     time.Time       `json:"endTime"`
	Timezone    string          `json:"timezone"`
	HourlyRate  decimal.Decimal `json:"hourlyRate"`
	TotalHours  decimal.Decimal `json:"totalHours"`
	TotalSalary decimal.Decimal `json:"totalSalary"`
	Days        []Day           `json:"days"`
}

// hoursBetween returns the scale-2 hours between clockIn and clockOut,
// rounding half up. Whole seconds only, matching the contract.
func hoursBetween(clockIn, clockOut time.Time) decimal.Decimal {
	seconds := new(big.Int).SetInt64(int64(clockOut.Sub(clockIn) / time.Second))
	n := new(big.Int).Mul(seconds, big.NewInt(100))
	q, r := new(big.Int).QuoRem(n, big.NewInt(3600), new(big.Int))
	if new(big.Int).Mul(r, big.NewInt(2)).Cmp(big.NewInt(3600)) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return decimal.FromBigInt(q, 2)
}

// mul2 returns a*b rounded half up to scale 2.
func mul2(a, b decimal.Decimal) decimal.Decimal {
	coeff := new(big.Int).Mul(a.Unscaled(), b.Unscaled())
	d := decimal.FromBigInt(coeff, a.Scale()+b.Scale())
	d.ScaleTo(2)
	return d
}

// sum2 returns the exact sum of scale-2 decimals as a scale-2 decimal.
func sum2(vals []decimal.Decimal) decimal.Decimal {
	total := new(big.Int)
	for _, v := range vals {
		total.Add(total, v.Unscaled())
	}
	return decimal.FromBigInt(total, 2)
}

// CompletedPairs pairs each CLOCK_IN with the next strictly-later CLOCK_OUT.
// entries must be ordered by entry time ascending.
func CompletedPairs(entries []db.AttendanceEntry) []Pair {
	pairs := make([]Pair, 0)
	var pending *db.AttendanceEntry
	for i := range entries {
		entry := &entries[i]
		if entry.Type == "CLOCK_IN" {
			pending = entry
			continue
		}
		if entry.Type == "CLOCK_OUT" && pending != nil && entry.EntryTime.Time.After(pending.EntryTime.Time) {
			hours := hoursBetween(pending.EntryTime.Time, entry.EntryTime.Time)
			pairs = append(pairs, Pair{
				ClockIn:  pending.EntryTime.Time,
				ClockOut: entry.EntryTime.Time,
				Hours:    hours,
			})
			pending = nil
		}
	}
	return pairs
}

// groupByLocalDate groups entries by their local calendar date, preserving
// the order of entries within each date.
func groupByLocalDate(entries []db.AttendanceEntry, loc *time.Location) map[string][]db.AttendanceEntry {
	byDate := make(map[string][]db.AttendanceEntry)
	for _, e := range entries {
		key := e.EntryTime.Time.In(loc).Format("2006-01-02")
		byDate[key] = append(byDate[key], e)
	}
	return byDate
}

// ValidateReportRequest validates a salary report range and hourly rate,
// returning the parsed timezone or a BAD_REQUEST API error.
func ValidateReportRequest(start, end time.Time, timezone string, hourlyRate *decimal.Decimal) (*time.Location, error) {
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
	s := start.In(loc)
	e := end.Add(-time.Nanosecond).In(loc)
	startDate := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, loc)
	endDate := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, loc)
	if startDate.AddDate(0, 0, 365).Before(endDate) {
		return nil, httpapi.BadRequest("Salary reports can cover at most 366 local days")
	}
	if hourlyRate == nil || hourlyRate.CmpInt(0) <= 0 {
		return nil, httpapi.BadRequest("Hourly rate must be greater than zero")
	}
	return loc, nil
}

// buildReport assembles a SalaryReport from attendance entries, grouping
// completed pairs by local calendar day.
func buildReport(outletID uuid.UUID, outletName string, employee *db.User, employeeDisplayName string,
	start, end time.Time, loc *time.Location, hourlyRate decimal.Decimal, entries []db.AttendanceEntry) *SalaryReport {
	byDate := groupByLocalDate(entries, loc)
	s := start.In(loc)
	e := end.Add(-time.Nanosecond).In(loc)
	startDate := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, loc)
	endDate := time.Date(e.Year(), e.Month(), e.Day(), 0, 0, 0, 0, loc)

	var days []Day
	var dayHours []decimal.Decimal
	var daySalaries []decimal.Decimal
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		pairs := CompletedPairs(byDate[d.Format("2006-01-02")])
		var hours []decimal.Decimal
		for _, p := range pairs {
			hours = append(hours, p.Hours)
		}
		totalHours := sum2(hours)
		salary := mul2(totalHours, hourlyRate)
		days = append(days, Day{
			Date:       d.Format("2006-01-02"),
			Pairs:      pairs,
			TotalHours: totalHours,
			HourlyRate: hourlyRate,
			Salary:     salary,
		})
		dayHours = append(dayHours, totalHours)
		daySalaries = append(daySalaries, salary)
	}

	return &SalaryReport{
		OutletID:    outletID,
		OutletName:  outletName,
		UserID:      toUUID(employee.ID),
		UserName:    employee.Name,
		UserEmail:   textPtr(employee.Email),
		DisplayName: employeeDisplayName,
		StartTime:   start.UTC(),
		EndTime:     end.UTC(),
		Timezone:    loc.String(),
		HourlyRate:  hourlyRate,
		TotalHours:  sum2(dayHours),
		TotalSalary: sum2(daySalaries),
		Days:        days,
	}
}

// Service computes salary reports from completed attendance pairs.
type Service struct {
	Store   *db.Store
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

// NewService returns a Service wired to the given dependencies.
func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service {
	m.RegisterCounter("report_salary_generated_total", []string{"format"}, []string{"json"}, []string{"xlsx"})
	return &Service{Store: store, Audit: a, Metrics: m}
}

// pgUUID wraps id in a pgtype.UUID marked valid.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// toUUID unwraps the bytes of a pgtype.UUID back into a uuid.UUID.
func toUUID(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

// textPtr returns a pointer to the string value of t, or nil when t is not
// valid.
func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}

// assertOwner rejects the caller unless they hold an accepted OWNER
// membership in the outlet.
func (s *Service) assertOwner(ctx context.Context, outletID, ownerID uuid.UUID) error {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(ownerID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return err
	}
	if m.Status != "ACCEPTED" {
		return httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	if m.Role != "OWNER" {
		return httpapi.Forbidden("Only outlet owners can perform this action")
	}
	return nil
}

// Calculate builds the salary report for one employee in one outlet over an
// instant range. Only accepted outlet owners may generate reports, and the
// path stays readable for soft-deleted outlets, employees, and memberships.
func (s *Service) Calculate(ctx context.Context, ownerID, outletID, employeeID uuid.UUID, start, end time.Time, timezone string, hourlyRate *decimal.Decimal, ip, userAgent string) (*SalaryReport, error) {
	loc, err := ValidateReportRequest(start, end, timezone, hourlyRate)
	if err != nil {
		return nil, err
	}
	if err := s.assertOwner(ctx, outletID, ownerID); err != nil {
		return nil, err
	}
	outlet, err := s.Store.Querier().GetOutletByIDIncludingDeleted(ctx, pgUUID(outletID))
	if err != nil {
		return nil, err
	}
	member, err := s.Store.Querier().GetMembershipByOutletAndUserIncludingRemoved(ctx, db.GetMembershipByOutletAndUserIncludingRemovedParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(employeeID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the requested employee")
	}
	if err != nil {
		return nil, err
	}
	employee, err := s.Store.Querier().GetUserByIDIncludingDeleted(ctx, pgUUID(employeeID))
	if err != nil {
		return nil, err
	}
	entries, err := s.Store.Querier().ListAttendanceByOutletUserRange(ctx, db.ListAttendanceByOutletUserRangeParams{
		OutletID:    pgUUID(outletID),
		UserID:      pgUUID(employeeID),
		EntryTime:   pgtype.Timestamptz{Time: start.UTC(), Valid: true},
		EntryTime_2: pgtype.Timestamptz{Time: end.UTC(), Valid: true},
	})
	if err != nil {
		return nil, err
	}
	report := buildReport(toUUID(outlet.ID), outlet.Name, &employee, member.DisplayName, start, end, loc, *hourlyRate, entries)
	s.Metrics.Increment("report_salary_generated_total", "format", "json")
	s.Audit.Record(ctx, ownerID.String(), "SALARY_REPORT_GENERATED", "OUTLET", outletID,
		map[string]any{
			"employeeUserId": employeeID,
			"startTime":      start.UTC().Format(time.RFC3339Nano),
			"endTime":        end.UTC().Format(time.RFC3339Nano),
			"timezone":       loc.String(),
			"format":         "json",
		}, ip, userAgent)
	return report, nil
}
