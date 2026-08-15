package report

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coderGtm/delta/go/decimal"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/xuri/excelize/v2"
)

func mustDec(s string) decimal.Decimal {
	d, err := decimal.Parse([]byte(s))
	if err != nil {
		panic(err)
	}
	return *d
}

func assertCell(t *testing.T, f *excelize.File, ref, want string) {
	t.Helper()
	got, err := f.GetCellValue("Salary Report", ref)
	if err != nil {
		t.Fatalf("GetCellValue(%s): %v", ref, err)
	}
	if got != want {
		t.Errorf("cell %s = %q, want %q", ref, got, want)
	}
}

func TestSanitizeExcelValue(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"=cmd()", "'=cmd()"},
		{"+sum(1,1)", "'+sum(1,1)"},
		{"-1+2", "'-1+2"},
		{"@cmd()", "'@cmd()"},
		{"\tleading tab", "'\tleading tab"},
		{"\x7f", "'\x7f"},
		{"", ""},
		{"plain text", "plain text"},
	}
	for _, c := range cases {
		if got := sanitizeExcelValue(c.in); got != c.want {
			t.Errorf("sanitizeExcelValue(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBuildExcel(t *testing.T) {
	email := "emp@example.com"
	rep := &SalaryReport{
		OutletName:  "Downtown",
		DisplayName: "Alex",
		UserEmail:   &email,
		StartTime:   time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		Timezone:    "UTC",
		TotalHours:  mustDec("2.00"),
		HourlyRate:  mustDec("10.5"),
		TotalSalary: mustDec("21.00"),
		Days: []Day{
			{
				Date: "2026-08-14",
				Pairs: []Pair{
					{ClockIn: time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC), ClockOut: time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)},
				},
				TotalHours: mustDec("2.00"),
				HourlyRate: mustDec("10.5"),
				Salary:     mustDec("21.00"),
			},
		},
	}
	data, err := (&Service{}).BuildExcel(rep)
	if err != nil {
		t.Fatalf("BuildExcel: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("BuildExcel returned an empty workbook")
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	if got := f.GetSheetName(0); got != "Salary Report" {
		t.Fatalf("sheet 0 name = %q, want %q", got, "Salary Report")
	}
	for _, c := range []struct{ ref, want string }{
		{"A1", "Salary Report"},
		{"A4", "Date"},
		{"B4", "Clock In 1"},
		{"C4", "Clock Out 1"},
		{"D4", "Total Hours"},
		{"E4", "Hourly Rate"},
		{"F4", "Salary"},
		{"A5", "2026-08-14"},
		{"B5", "09:00:00"},
		{"C5", "11:00:00"},
		{"D5", "2"},
		{"E5", "10.5"},
		{"F5", "21"},
		{"A6", "TOTAL"},
		{"D6", "2"},
		{"E6", "10.5"},
		{"F6", "21"},
	} {
		assertCell(t, f, c.ref, c.want)
	}
}

func TestBuildExcelFormatsTimesInReportTimezone(t *testing.T) {
	rep := &SalaryReport{
		OutletName:  "Downtown",
		DisplayName: "Alex",
		StartTime:   time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC),
		EndTime:     time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC),
		Timezone:    "Asia/Kolkata",
		TotalHours:  mustDec("2.00"),
		HourlyRate:  mustDec("10.5"),
		TotalSalary: mustDec("21.00"),
		Days: []Day{
			{
				Date: "2026-08-14",
				Pairs: []Pair{
					{ClockIn: time.Date(2026, 8, 13, 18, 30, 0, 0, time.UTC), ClockOut: time.Date(2026, 8, 13, 20, 30, 0, 0, time.UTC)},
				},
				TotalHours: mustDec("2.00"),
				HourlyRate: mustDec("10.5"),
				Salary:     mustDec("21.00"),
			},
		},
	}
	data, err := (&Service{}).BuildExcel(rep)
	if err != nil {
		t.Fatalf("BuildExcel: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer f.Close()
	assertCell(t, f, "B5", "00:00:00")
	assertCell(t, f, "C5", "02:00:00")
}

func TestBuildExcelRejectsInvalidTimezone(t *testing.T) {
	rep := &SalaryReport{Timezone: "Not/AZone"}
	_, err := (&Service{}).BuildExcel(rep)
	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("BuildExcel error = %v, want BAD_REQUEST API error", err)
	}
}
