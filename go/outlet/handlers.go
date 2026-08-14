package outlet

import (
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
)

// Handlers exposes the outlet and membership endpoints.
type Handlers struct {
	Svc        *Service
	TrustProxy bool
}

// updateGeofenceRequest is the body of PUT /api/v1/outlets/{outletId}/geofence.
// The pointer distinguishes a missing flag from an explicit false.
type updateGeofenceRequest struct {
	GeofenceEnabled *bool `json:"geofenceEnabled"`
}

// inviteMemberRequest is the body of POST /api/v1/outlets/{outletId}/memberships/invite.
type inviteMemberRequest struct {
	Email string `json:"email"`
}

// updateDisplayNameRequest is the body of PUT /api/v1/outlets/{outletId}/memberships/{membershipId}/display-name.
type updateDisplayNameRequest struct {
	DisplayName string `json:"displayName"`
}

// emailRe is a permissive email format check mirroring Hibernate's @Email
// constraint (empty strings are rejected by the blank check before this runs;
// single-label domains like a@b are accepted).
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)

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

// pathMembershipID parses the {membershipId} path value, returning a NOT_FOUND
// API error when it is not a valid UUID.
func pathMembershipID(r *http.Request) (uuid.UUID, error) {
	val := r.PathValue("membershipId")
	id, err := uuid.Parse(val)
	if err != nil {
		return uuid.Nil, httpapi.NotFound("Outlet membership not found: " + val)
	}
	return id, nil
}

// CreateOutlet handles POST /api/v1/outlets, creating a new outlet owned by
// the authenticated user.
func (h *Handlers) CreateOutlet(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	var req CreateOutletRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.CreateOutlet(r.Context(), userID, req, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

// GetOutlet handles GET /api/v1/outlets/{outletId}, returning the outlet when
// the authenticated user has an accepted membership.
func (h *Handlers) GetOutlet(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.Svc.GetOutlet(r.Context(), userID, outletID)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// UpdateOutlet handles PUT /api/v1/outlets/{outletId}, replacing the editable
// details of an outlet. Only accepted outlet owners may perform this action.
func (h *Handlers) UpdateOutlet(w http.ResponseWriter, r *http.Request) {
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
	var req UpdateOutletRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.UpdateOutlet(r.Context(), userID, outletID, req, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// UpdateGeofence handles PUT /api/v1/outlets/{outletId}/geofence, toggling
// attendance geofence enforcement for an outlet. Only accepted outlet owners
// may perform this action.
func (h *Handlers) UpdateGeofence(w http.ResponseWriter, r *http.Request) {
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
	var req updateGeofenceRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if req.GeofenceEnabled == nil {
		httpapi.WriteError(w, httpapi.Validation("Geofence enabled flag is required"))
		return
	}
	out, err := h.Svc.UpdateGeofence(r.Context(), userID, outletID, *req.GeofenceEnabled, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// GetMyOutlets handles GET /api/v1/outlets/mine, listing the outlets the
// authenticated user has already joined.
func (h *Handlers) GetMyOutlets(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	page, err := h.Svc.GetMyOutlets(r.Context(), userID, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

// GetMyInvites handles GET /api/v1/outlets/invites, listing the pending outlet
// invitations for the authenticated user.
func (h *Handlers) GetMyInvites(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	page, err := h.Svc.GetMyInvites(r.Context(), userID, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

// GetOutletMemberships handles GET /api/v1/outlets/{outletId}/memberships,
// listing the memberships of an outlet on behalf of its accepted owner.
func (h *Handlers) GetOutletMemberships(w http.ResponseWriter, r *http.Request) {
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
	page, err := h.Svc.GetOutletMemberships(r.Context(), userID, outletID, httpapi.ParsePageParams(r))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

// DeleteOutlet handles DELETE /api/v1/outlets/{outletId}, soft-deleting an
// outlet on behalf of its accepted owner.
func (h *Handlers) DeleteOutlet(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Svc.DeleteOutlet(r.Context(), userID, outletID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LeaveOutlet handles POST /api/v1/outlets/{outletId}/leave, soft-removing
// the authenticated user's own membership in the outlet. Owners cannot leave
// through this endpoint.
func (h *Handlers) LeaveOutlet(w http.ResponseWriter, r *http.Request) {
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
	if err := h.Svc.LeaveOutlet(r.Context(), userID, outletID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// InviteMember handles POST /api/v1/outlets/{outletId}/memberships/invite,
// sending an invitation to an existing user to join the outlet as an employee.
func (h *Handlers) InviteMember(w http.ResponseWriter, r *http.Request) {
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
	var req inviteMemberRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		httpapi.WriteError(w, httpapi.Validation("Employee email is required"))
		return
	}
	if !emailRe.MatchString(strings.TrimSpace(req.Email)) {
		httpapi.WriteError(w, httpapi.Validation("Employee email must be a valid email address"))
		return
	}
	out, err := h.Svc.InviteMember(r.Context(), userID, outletID, req.Email, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, out)
}

// RemoveMembership handles DELETE /api/v1/outlets/{outletId}/memberships/{membershipId},
// soft-removing an employee's membership in an outlet on behalf of its owner.
func (h *Handlers) RemoveMembership(w http.ResponseWriter, r *http.Request) {
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
	membershipID, err := pathMembershipID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if err := h.Svc.RemoveMembership(r.Context(), userID, outletID, membershipID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent")); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateDisplayName handles PUT /api/v1/outlets/{outletId}/memberships/{membershipId}/display-name,
// setting the owner-controlled display name of a member of the outlet.
func (h *Handlers) UpdateDisplayName(w http.ResponseWriter, r *http.Request) {
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
	membershipID, err := pathMembershipID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	var req updateDisplayNameRequest
	if err := httpapi.DecodeJSON(r, &req); err != nil {
		httpapi.WriteError(w, err)
		return
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		httpapi.WriteError(w, httpapi.Validation("Display name is required"))
		return
	}
	if utf8.RuneCountInString(strings.TrimSpace(req.DisplayName)) > 255 {
		httpapi.WriteError(w, httpapi.Validation("Display name must be at most 255 characters"))
		return
	}
	out, err := h.Svc.UpdateDisplayName(r.Context(), userID, outletID, membershipID, req.DisplayName, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// AcceptInvite handles POST /api/v1/outlets/memberships/{membershipId}/accept,
// accepting a pending outlet invitation for the authenticated user.
func (h *Handlers) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	membershipID, err := pathMembershipID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.AcceptInvite(r.Context(), userID, membershipID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}

// RejectInvite handles POST /api/v1/outlets/memberships/{membershipId}/reject,
// rejecting a pending outlet invitation for the authenticated user.
func (h *Handlers) RejectInvite(w http.ResponseWriter, r *http.Request) {
	userID, err := h.currentUserID(r)
	if err != nil {
		httpapi.WriteError(w, httpapi.Internal("authenticated user missing"))
		return
	}
	membershipID, err := pathMembershipID(r)
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	out, err := h.Svc.RejectInvite(r.Context(), userID, membershipID, httpapi.ClientIP(r, h.TrustProxy), r.Header.Get("User-Agent"))
	if err != nil {
		httpapi.WriteError(w, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, out)
}
