package report

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coderGtm/delta/decimal"
	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
)

// Handlers exposes the report endpoints.
type Handlers struct {
	Svc        *Service
	TrustProxy bool
}

// currentUserID returns the authenticated user's ID from the request context.
func (h *Handlers) currentUserID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(httpapi.SubjectID(r))
}

// pathOutletID parses the {outletId} path value, returning a NOT_FOUND API
// error when it is not a valid UUID.
func pathOutletID(r *http.Request) (uuid.UUID, error) {
	val := r.PathValue("outletId")
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, httpapi.NotFound("Outlet not found: " + val)
	}
	return id, nil
}

// parseReportInstant parses an RFC3339 instant, falling back to an offsetless
// local date-time that is interpreted as UTC, and normalizes the result to
// UTC.
func parseReportInstant(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", raw)
	}
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

// reportQuery extracts and validates the salary-report query parameters,
// mirroring the reference controller's parameter validation. Parse failures
// short-circuit with a BAD_REQUEST error; every missing or invalid-value
// constraint is otherwise collected and joined with ", " in the field order
// userId, startTime, endTime, timezone, hourlyRate.
func reportQuery(q url.Values) (employeeID uuid.UUID, start, end time.Time, timezone string, rate *decimal.Decimal, err error) {
	var msgs []string

	raw := q.Get("userId")
	if raw == "" {
		msgs = append(msgs, "User ID is required")
	} else if id, perr := uuid.Parse(raw); perr != nil {
		return uuid.Nil, time.Time{}, time.Time{}, "", nil, httpapi.BadRequest("Invalid userId: " + raw)
	} else {
		employeeID = id
	}

	raw = q.Get("startTime")
	if raw == "" {
		msgs = append(msgs, "Start time is required")
	} else if t, perr := parseReportInstant(raw); perr != nil {
		return uuid.Nil, time.Time{}, time.Time{}, "", nil, httpapi.BadRequest("Invalid startTime: " + raw)
	} else {
		start = t
	}

	raw = q.Get("endTime")
	if raw == "" {
		msgs = append(msgs, "End time is required")
	} else if t, perr := parseReportInstant(raw); perr != nil {
		return uuid.Nil, time.Time{}, time.Time{}, "", nil, httpapi.BadRequest("Invalid endTime: " + raw)
	} else {
		end = t
	}

	timezone = strings.TrimSpace(q.Get("timezone"))
	if timezone == "" {
		msgs = append(msgs, "Timezone is required")
	}

	raw = q.Get("hourlyRate")
	if raw == "" {
		msgs = append(msgs, "Hourly rate is required")
	} else {
		d, perr := decimal.Parse([]byte(raw))
		if perr != nil {
			return uuid.Nil, time.Time{}, time.Time{}, "", nil, httpapi.BadRequest("Invalid hourlyRate: " + raw)
		}
		if d.CmpInt(0) <= 0 {
			msgs = append(msgs, "Hourly rate must be greater than zero")
		} else {
			rate = d
		}
	}

	if len(msgs) > 0 {
		return uuid.Nil, time.Time{}, time.Time{}, "", nil, httpapi.Validation(strings.Join(msgs, ", "))
	}
	return employeeID, start, end, timezone, rate, nil
}

// sanitizeForFilename renders an instant for a filename, replacing the colons
// of the RFC3339 representation so it is safe to use on common filesystems.
func sanitizeForFilename(t time.Time) string {
	return strings.ReplaceAll(t.UTC().Format(time.RFC3339Nano), ":", "-")
}

// Salary handles GET /api/v1/outlets/{outletId}/reports/salary, calculating
// the salary report for one employee in the outlet over an exact timestamp
// range.
func (h *Handlers) Salary(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, err := pathOutletID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	employeeID, start, end, timezone, rate, err := reportQuery(r.URL.Query())
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	rep, err := h.Svc.Calculate(r.Context(), userID, outletID, employeeID, start, end, timezone, rate, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, rep)
}

// SalaryXLSX handles GET /api/v1/outlets/{outletId}/reports/salary.xlsx,
// returning the salary report as an Excel workbook.
func (h *Handlers) SalaryXLSX(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	outletID, err := pathOutletID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	employeeID, start, end, timezone, rate, err := reportQuery(r.URL.Query())
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	data, err := h.Svc.ExportExcel(r.Context(), userID, outletID, employeeID, start, end, timezone, rate, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	filename := fmt.Sprintf("salary-report-%s-%s-to-%s.xlsx", employeeID, sanitizeForFilename(start), sanitizeForFilename(end))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
