package attendance

import (
	"net/http"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

// Handlers exposes the attendance endpoints.
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

// pathAttendanceEntryID parses the {attendanceEntryId} path value, returning a
// NOT_FOUND API error when it is not a valid UUID.
func pathAttendanceEntryID(r *http.Request) (uuid.UUID, error) {
	val := r.PathValue("attendanceEntryId")
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, httpapi.NotFound("Attendance entry not found: " + val)
	}
	return id, nil
}

// CreateOwn handles POST /api/v1/outlets/{outletId}/attendance, recording the
// authenticated employee's own attendance entry with the current server time.
func (h *Handlers) CreateOwn(w http.ResponseWriter, r *http.Request) {
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
	var req CreateOwnRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.CreateOwn(r.Context(), userID, outletID, req, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

// CreateManaged handles POST /api/v1/outlets/{outletId}/attendance/manage,
// recording an attendance entry for an employee on behalf of the outlet's
// accepted owner.
func (h *Handlers) CreateManaged(w http.ResponseWriter, r *http.Request) {
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
	var req ManageRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.CreateManaged(r.Context(), userID, outletID, req, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

// List handles GET /api/v1/outlets/{outletId}/attendance, returning a page of
// attendance entries. Owners may filter by user; employees only receive their
// own records.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
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
	var targetUserID *uuid.UUID
	if raw := r.URL.Query().Get("userId"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			httpapi.WriteError(w, httpapi.BadRequest("Invalid userId: "+raw))
			return
		}
		targetUserID = &id
	}
	resp, err := h.Svc.List(r.Context(), userID, outletID, targetUserID, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /api/v1/outlets/{outletId}/attendance/{attendanceEntryId},
// returning a single attendance entry when the caller is allowed to view it.
func (h *Handlers) Get(w http.ResponseWriter, r *http.Request) {
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
	entryID, err := pathAttendanceEntryID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.Get(r.Context(), userID, outletID, entryID)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// Update handles PUT /api/v1/outlets/{outletId}/attendance/{attendanceEntryId},
// replacing the type, time, and location of an attendance entry on behalf of
// the outlet's accepted owner.
func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
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
	entryID, err := pathAttendanceEntryID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	var req UpdateRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.Update(r.Context(), userID, outletID, entryID, req, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// Delete handles DELETE /api/v1/outlets/{outletId}/attendance/{attendanceEntryId},
// removing an attendance entry on behalf of the outlet's accepted owner.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
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
	entryID, err := pathAttendanceEntryID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if err := h.Svc.Delete(r.Context(), userID, outletID, entryID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
