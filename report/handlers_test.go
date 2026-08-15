package report

import (
	"net/url"
	"testing"
	"time"

	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
)

func TestReportQuery(t *testing.T) {
	uid := uuid.New()

	t.Run("valid RFC3339 values", func(t *testing.T) {
		q := url.Values{
			"userId":     {uid.String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T18:30:00+05:30"},
			"timezone":   {"  Asia/Kolkata  "},
			"hourlyRate": {"10.50"},
		}
		employeeID, start, end, timezone, rate, err := reportQuery(q)
		if err != nil {
			t.Fatalf("reportQuery: %v", err)
		}
		if employeeID != uid {
			t.Errorf("employeeID = %s, want %s", employeeID, uid)
		}
		wantStart := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		if !start.Equal(wantStart) || start.Location() != time.UTC {
			t.Errorf("start = %v, want UTC %v", start, wantStart)
		}
		wantEnd := time.Date(2026, 8, 15, 13, 0, 0, 0, time.UTC)
		if !end.Equal(wantEnd) || end.Location() != time.UTC {
			t.Errorf("end = %v, want UTC %v", end, wantEnd)
		}
		if timezone != "Asia/Kolkata" {
			t.Errorf("timezone = %q, want Asia/Kolkata", timezone)
		}
		if rate == nil {
			t.Fatal("rate is nil")
		}
		if got := rate.Scale(); got != 2 {
			t.Errorf("rate scale = %d, want 2", got)
		}
	})

	t.Run("offsetless times are UTC", func(t *testing.T) {
		q := url.Values{
			"userId":     {uid.String()},
			"startTime":  {"2026-08-14T00:00:00"},
			"endTime":    {"2026-08-15T18:30:00"},
			"timezone":   {"UTC"},
			"hourlyRate": {"10"},
		}
		_, start, end, _, rate, err := reportQuery(q)
		if err != nil {
			t.Fatalf("reportQuery: %v", err)
		}
		wantStart := time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)
		if !start.Equal(wantStart) || start.Location() != time.UTC {
			t.Errorf("start = %v, want UTC %v", start, wantStart)
		}
		wantEnd := time.Date(2026, 8, 15, 18, 30, 0, 0, time.UTC)
		if !end.Equal(wantEnd) || end.Location() != time.UTC {
			t.Errorf("end = %v, want UTC %v", end, wantEnd)
		}
		if rate == nil {
			t.Fatal("rate is nil")
		}
	})

	t.Run("missing all", func(t *testing.T) {
		_, _, _, _, _, err := reportQuery(url.Values{})
		want := "User ID is required, Start time is required, End time is required, Timezone is required, Hourly rate is required"
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "VALIDATION_ERROR" || ae.Message != want {
			t.Errorf("got %v, want VALIDATION_ERROR %q", err, want)
		}
	})

	t.Run("invalid userId", func(t *testing.T) {
		q := url.Values{
			"userId":     {"not-a-uuid"},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"10"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "BAD_REQUEST" || ae.Message != "Invalid userId: not-a-uuid" {
			t.Errorf("got %v, want BAD_REQUEST Invalid userId: not-a-uuid", err)
		}
	})

	t.Run("invalid startTime", func(t *testing.T) {
		q := url.Values{
			"userId":     {uuid.New().String()},
			"startTime":  {"not-a-time"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"10"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "BAD_REQUEST" || ae.Message != "Invalid startTime: not-a-time" {
			t.Errorf("got %v, want BAD_REQUEST Invalid startTime: not-a-time", err)
		}
	})

	t.Run("invalid endTime", func(t *testing.T) {
		q := url.Values{
			"userId":     {uuid.New().String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"not-a-time"},
			"timezone":   {"UTC"},
			"hourlyRate": {"10"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "BAD_REQUEST" || ae.Message != "Invalid endTime: not-a-time" {
			t.Errorf("got %v, want BAD_REQUEST Invalid endTime: not-a-time", err)
		}
	})

	t.Run("invalid hourlyRate", func(t *testing.T) {
		q := url.Values{
			"userId":     {uuid.New().String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"abc"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "BAD_REQUEST" || ae.Message != "Invalid hourlyRate: abc" {
			t.Errorf("got %v, want BAD_REQUEST Invalid hourlyRate: abc", err)
		}
	})

	t.Run("zero hourly rate", func(t *testing.T) {
		q := url.Values{
			"userId":     {uuid.New().String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"0"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "VALIDATION_ERROR" || ae.Message != "Hourly rate must be greater than zero" {
			t.Errorf("got %v, want VALIDATION_ERROR Hourly rate must be greater than zero", err)
		}
	})

	t.Run("negative hourly rate", func(t *testing.T) {
		q := url.Values{
			"userId":     {uuid.New().String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"-5"},
		}
		_, _, _, _, _, err := reportQuery(q)
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "VALIDATION_ERROR" || ae.Message != "Hourly rate must be greater than zero" {
			t.Errorf("got %v, want VALIDATION_ERROR Hourly rate must be greater than zero", err)
		}
	})

	t.Run("zero hourly rate joins missing fields", func(t *testing.T) {
		q := url.Values{"hourlyRate": {"0"}}
		_, _, _, _, _, err := reportQuery(q)
		want := "User ID is required, Start time is required, End time is required, Timezone is required, Hourly rate must be greater than zero"
		ae, ok := err.(*httpapi.APIError)
		if !ok || ae.Code != "VALIDATION_ERROR" || ae.Message != want {
			t.Errorf("got %v, want VALIDATION_ERROR %q", err, want)
		}
	})

	t.Run("scale preserved", func(t *testing.T) {
		q := url.Values{
			"userId":     {uid.String()},
			"startTime":  {"2026-08-14T00:00:00Z"},
			"endTime":    {"2026-08-15T00:00:00Z"},
			"timezone":   {"UTC"},
			"hourlyRate": {"10.50"},
		}
		_, _, _, _, rate, err := reportQuery(q)
		if err != nil {
			t.Fatalf("reportQuery: %v", err)
		}
		if rate == nil {
			t.Fatal("rate is nil")
		}
		if got := rate.Scale(); got != 2 {
			t.Errorf("rate scale = %d, want 2", got)
		}
		if got := rate.Format(2); got != "10.50" {
			t.Errorf("rate = %s, want 10.50", got)
		}
	})
}
