package report

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coderGtm/delta/go/decimal"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ExportExcel generates the Excel workbook for the salary report. Like the
// reference flow it records the report event (via Calculate) and then the
// export event and metric.
func (s *Service) ExportExcel(ctx context.Context, ownerID, outletID, employeeID uuid.UUID, start, end time.Time, timezone string, hourlyRate *decimal.Decimal, ip, userAgent string) ([]byte, error) {
	report, err := s.Calculate(ctx, ownerID, outletID, employeeID, start, end, timezone, hourlyRate, ip, userAgent)
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("report.salary.generated", "format", "xlsx")
	s.Audit.Record(ctx, ownerID.String(), "SALARY_REPORT_EXCEL_GENERATED", "OUTLET", outletID,
		map[string]any{"employeeUserId": employeeID, "startTime": start.UTC().Format(time.RFC3339Nano), "endTime": end.UTC().Format(time.RFC3339Nano), "timezone": report.Timezone, "format": "xlsx"}, ip, userAgent)
	return s.BuildExcel(report)
}

// BuildExcel renders report as an XLSX workbook matching the reference export
// layout: title, metadata, header, one row per day, and a total row. The
// receiver is unused; the method is attached to Service to match the plan
// interface.
func (s *Service) BuildExcel(report *SalaryReport) ([]byte, error) {
	loc, err := time.LoadLocation(report.Timezone)
	if err != nil {
		return nil, httpapi.BadRequest("Timezone must be a valid IANA timezone")
	}
	maxPairs := 0
	for _, d := range report.Days {
		if len(d.Pairs) > maxPairs {
			maxPairs = len(d.Pairs)
		}
	}

	f := excelize.NewFile()
	defer f.Close()
	f.SetSheetName("Sheet1", "Salary Report")

	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, err
	}
	sheet := "Salary Report"

	row := 1
	writeStringCell(f, sheet, cell(1, row), "Salary Report", bold)

	row++
	writeStringCell(f, sheet, cell(1, row), "Outlet", bold)
	writeStringCell(f, sheet, cell(2, row), report.OutletName, 0)
	writeStringCell(f, sheet, cell(3, row), "Employee", bold)
	writeStringCell(f, sheet, cell(4, row), fmt.Sprintf("%s <%s>", report.DisplayName, emailStr(report.UserEmail)), 0)
	writeStringCell(f, sheet, cell(5, row), "Period", bold)
	writeStringCell(f, sheet, cell(6, row), report.StartTime.In(loc).String()+" to "+report.EndTime.In(loc).String(), 0)
	writeStringCell(f, sheet, cell(7, row), "Timezone", bold)
	writeStringCell(f, sheet, cell(8, row), report.Timezone, 0)

	row++
	row++
	col := 1
	writeStringCell(f, sheet, cell(col, row), "Date", bold)
	col++
	for i := 1; i <= maxPairs; i++ {
		writeStringCell(f, sheet, cell(col, row), fmt.Sprintf("Clock In %d", i), bold)
		col++
		writeStringCell(f, sheet, cell(col, row), fmt.Sprintf("Clock Out %d", i), bold)
		col++
	}
	writeStringCell(f, sheet, cell(col, row), "Total Hours", bold)
	col++
	writeStringCell(f, sheet, cell(col, row), "Hourly Rate", bold)
	col++
	writeStringCell(f, sheet, cell(col, row), "Salary", bold)

	for _, d := range report.Days {
		row++
		col = 1
		writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue(d.Date), 0)
		col++
		for _, p := range d.Pairs {
			writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue(p.ClockIn.In(loc).Format("15:04:05")), 0)
			col++
			writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue(p.ClockOut.In(loc).Format("15:04:05")), 0)
			col++
		}
		for col < 2+maxPairs*2 {
			writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue(""), 0)
			col++
		}
		writeNumericCell(f, sheet, cell(col, row), d.TotalHours.Float64(), 0)
		col++
		writeNumericCell(f, sheet, cell(col, row), d.HourlyRate.Float64(), 0)
		col++
		writeNumericCell(f, sheet, cell(col, row), d.Salary.Float64(), 0)
	}

	row++
	col = 1
	writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue("TOTAL"), bold)
	col++
	for col < 2+maxPairs*2 {
		writeStringCell(f, sheet, cell(col, row), sanitizeExcelValue(""), bold)
		col++
	}
	writeNumericCell(f, sheet, cell(col, row), report.TotalHours.Float64(), bold)
	col++
	writeNumericCell(f, sheet, cell(col, row), report.HourlyRate.Float64(), bold)
	col++
	writeNumericCell(f, sheet, cell(col, row), report.TotalSalary.Float64(), bold)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// emailStr renders the email pointer for the metadata row, substituting
// "null" for a nil pointer as the reference export does.
func emailStr(email *string) string {
	if email == nil {
		return "null"
	}
	return *email
}

// writeStringCell writes a sanitized string value at ref, applying styleID
// when non-zero.
func writeStringCell(f *excelize.File, sheet, ref, value string, styleID int) {
	f.SetCellValue(sheet, ref, sanitizeExcelValue(value))
	if styleID != 0 {
		f.SetCellStyle(sheet, ref, ref, styleID)
	}
}

// writeNumericCell writes value as a number at ref, applying styleID when
// non-zero.
func writeNumericCell(f *excelize.File, sheet, ref string, value float64, styleID int) {
	f.SetCellValue(sheet, ref, value)
	if styleID != 0 {
		f.SetCellStyle(sheet, ref, ref, styleID)
	}
}

// sanitizeExcelValue defends against spreadsheet formula injection by
// prefixing a single quote when the value starts with a formula or control
// character.
func sanitizeExcelValue(v string) string {
	if v == "" {
		return v
	}
	first := v[0]
	if first == '=' || first == '+' || first == '-' || first == '@' || first <= 0x20 || first == 0x7f {
		return "'" + v
	}
	return v
}

// columnName returns the Excel column letter for a 1-based column number
// (1 -> A, 2 -> B, 27 -> AA).
func columnName(col int) string {
	var b strings.Builder
	for col > 0 {
		col--
		b.WriteByte(byte('A' + col%26))
		col /= 26
	}
	s := b.String()
	rev := []byte(s)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return string(rev)
}

// cell returns the 1-based (col, row) coordinate as an A1-style reference.
func cell(col, row int) string {
	return fmt.Sprintf("%s%d", columnName(col), row)
}
