package attendance

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/decimal"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// EntryType is the kind of an attendance entry.
type EntryType string

const (
	// EntryTypeClockIn marks an entry as the start of a work period.
	EntryTypeClockIn EntryType = "CLOCK_IN"
	// EntryTypeClockOut marks an entry as the end of a work period.
	EntryTypeClockOut EntryType = "CLOCK_OUT"
)

// CreateOwnRequest is the payload for employee self-service attendance.
// Pointer coordinate fields distinguish a missing value from an explicit zero.
type CreateOwnRequest struct {
	Type      EntryType        `json:"type"`
	Latitude  *decimal.Decimal `json:"latitude"`
	Longitude *decimal.Decimal `json:"longitude"`
}

// ManageRequest is the payload for owner-created employee attendance. The
// user pointer and the pointer coordinates distinguish missing values.
type ManageRequest struct {
	UserID    *uuid.UUID       `json:"userId"`
	Type      EntryType        `json:"type"`
	EntryTime time.Time        `json:"entryTime"`
	Latitude  *decimal.Decimal `json:"latitude"`
	Longitude *decimal.Decimal `json:"longitude"`
}

// UpdateRequest is the payload for owner-updated employee attendance. The
// pointer coordinates distinguish missing values.
type UpdateRequest struct {
	Type      EntryType        `json:"type"`
	EntryTime time.Time        `json:"entryTime"`
	Latitude  *decimal.Decimal `json:"latitude"`
	Longitude *decimal.Decimal `json:"longitude"`
}

// Service coordinates attendance entry creation and access rules.
type Service struct {
	Store   *db.Store
	Clock   func() time.Time
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

// NewService returns a Service wired to the given dependencies. A nil clock
// falls back to the system clock.
func NewService(store *db.Store, clock func() time.Time, a *audit.Recorder, m *metrics.Registry) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{Store: store, Clock: clock, Audit: a, Metrics: m}
}

// validateCreateOwn checks the self-service attendance payload and returns a
// VALIDATION_ERROR API error listing every failing constraint in field order,
// or nil when the request is valid.
func validateCreateOwn(req CreateOwnRequest) *httpapi.APIError {
	var msgs []string
	if req.Type == "" {
		msgs = append(msgs, "Attendance type is required")
	} else if req.Type != EntryTypeClockIn && req.Type != EntryTypeClockOut {
		return httpapi.BadRequest("Unsupported attendance type: " + string(req.Type))
	}
	msgs = append(msgs, coordinateMsgs(true, req.Latitude)...)
	msgs = append(msgs, coordinateMsgs(false, req.Longitude)...)
	if len(msgs) == 0 {
		return nil
	}
	return httpapi.Validation(strings.Join(msgs, ", "))
}

// validateManage checks the owner-managed attendance payload and returns a
// VALIDATION_ERROR API error listing every failing constraint in field order,
// or nil when the request is valid.
func validateManage(req ManageRequest) *httpapi.APIError {
	var msgs []string
	if req.UserID == nil {
		msgs = append(msgs, "User ID is required")
	}
	if req.Type == "" {
		msgs = append(msgs, "Attendance type is required")
	} else if req.Type != EntryTypeClockIn && req.Type != EntryTypeClockOut {
		return httpapi.BadRequest("Unsupported attendance type: " + string(req.Type))
	}
	if req.EntryTime.IsZero() {
		msgs = append(msgs, "Entry time is required")
	}
	msgs = append(msgs, coordinateMsgs(true, req.Latitude)...)
	msgs = append(msgs, coordinateMsgs(false, req.Longitude)...)
	if len(msgs) == 0 {
		return nil
	}
	return httpapi.Validation(strings.Join(msgs, ", "))
}

// validateUpdate checks the owner-managed attendance update payload and
// returns a VALIDATION_ERROR API error listing every failing constraint in
// field order, or nil when the request is valid.
func validateUpdate(req UpdateRequest) *httpapi.APIError {
	var msgs []string
	if req.Type == "" {
		msgs = append(msgs, "Attendance type is required")
	} else if req.Type != EntryTypeClockIn && req.Type != EntryTypeClockOut {
		return httpapi.BadRequest("Unsupported attendance type: " + string(req.Type))
	}
	if req.EntryTime.IsZero() {
		msgs = append(msgs, "Entry time is required")
	}
	msgs = append(msgs, coordinateMsgs(true, req.Latitude)...)
	msgs = append(msgs, coordinateMsgs(false, req.Longitude)...)
	if len(msgs) == 0 {
		return nil
	}
	return httpapi.Validation(strings.Join(msgs, ", "))
}

// coordinateMsgs returns the validation messages for one latitude or longitude
// value. isLat selects the field label and bounds. A value can violate at most
// one of its two bounds.
func coordinateMsgs(isLat bool, v *decimal.Decimal) []string {
	if v == nil {
		if isLat {
			return []string{"Latitude is required"}
		}
		return []string{"Longitude is required"}
	}
	if isLat {
		switch {
		case v.CmpInt(-90) < 0:
			return []string{"Latitude must be greater than or equal to -90"}
		case v.CmpInt(90) > 0:
			return []string{"Latitude must be less than or equal to 90"}
		}
		return nil
	}
	switch {
	case v.CmpInt(-180) < 0:
		return []string{"Longitude must be greater than or equal to -180"}
	case v.CmpInt(180) > 0:
		return []string{"Longitude must be less than or equal to 180"}
	}
	return nil
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

// uuidPtr returns a pointer to the UUID value of u, or nil when u is not
// valid.
func uuidPtr(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := toUUID(u)
	return &id
}

// pgNumericFromDecimal converts d to a scale-7 PostgreSQL numeric value,
// rounding half away from zero when needed.
func pgNumericFromDecimal(d decimal.Decimal) pgtype.Numeric {
	d.ScaleTo(7)
	return pgtype.Numeric{Int: d.Unscaled(), Exp: -7, Valid: true}
}

// decimalFromPgNumeric converts a PostgreSQL numeric value into a scale-7
// decimal, rejecting null, NaN, infinite, and over-long values.
func decimalFromPgNumeric(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid || n.NaN || n.InfinityModifier != 0 || n.Int == nil {
		return decimal.Decimal{}, fmt.Errorf("attendance: invalid numeric value")
	}
	shift := n.Exp + 7
	if shift < 0 {
		return decimal.Decimal{}, fmt.Errorf("attendance: numeric value has more than 7 fractional digits")
	}
	coeff := new(big.Int).Set(n.Int)
	if shift > 0 {
		coeff.Mul(coeff, tenTo(shift))
	}
	return decimal.FromBigInt(coeff, 7), nil
}

// tenTo returns 10 raised to the n-th power.
func tenTo(n int32) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}

// getActiveMembership loads the current user's accepted membership in the
// outlet. notFoundMsg identifies the membership subject in the NOT_FOUND
// error.
func (s *Service) getActiveMembership(ctx context.Context, outletID, userID uuid.UUID, notFoundMsg string) (*db.OutletMembership, error) {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound(notFoundMsg)
	}
	if err != nil {
		return nil, err
	}
	if m.Status != "ACCEPTED" {
		return nil, httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	return &m, nil
}

// assertActiveOutlet loads the non-removed outlet with the given ID. Only
// write paths call it; read paths must not so that historical records stay
// readable for deleted outlets.
func (s *Service) assertActiveOutlet(ctx context.Context, outletID uuid.UUID) (*db.Outlet, error) {
	o, err := s.Store.Querier().GetOutletByID(ctx, pgUUID(outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet not found: " + outletID.String())
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// requireEmployeeRole returns a FORBIDDEN API error unless role is the
// employee role.
func requireEmployeeRole(role string) error {
	if role != "EMPLOYEE" {
		return httpapi.Forbidden("Only accepted employees can create their own attendance entries")
	}
	return nil
}

// requireOwnerRole returns a FORBIDDEN API error unless role is the owner
// role.
func requireOwnerRole(role string) error {
	if role != "OWNER" {
		return httpapi.Forbidden("Only outlet owners can perform this action")
	}
	return nil
}

// enforceGeofence rejects attendance writes whose coordinates fall outside
// the outlet's geofence when enforcement is enabled.
func (s *Service) enforceGeofence(o *db.Outlet, lat, lon *decimal.Decimal) error {
	if !o.GeofenceEnabled {
		return nil
	}
	centerLat, err := decimalFromPgNumeric(o.Latitude)
	if err != nil {
		return err
	}
	centerLon, err := decimalFromPgNumeric(o.Longitude)
	if err != nil {
		return err
	}
	within := IsWithinRadiusMeters(centerLat.Float64(), centerLon.Float64(), lat.Float64(), lon.Float64(), int(o.RadiusMeters))
	if !within {
		s.Metrics.Increment("attendance.geofence.rejected", "outletId", o.ID.String())
		return httpapi.Forbidden("Attendance location is outside the outlet geofence")
	}
	return nil
}

// memberDisplayName resolves the outlet-scoped display name for a user,
// falling back to the account name when no membership row exists. Removed
// memberships still resolve so historical entries keep their custom names.
func (s *Service) memberDisplayName(ctx context.Context, outletID, userID uuid.UUID, fallback string) (string, error) {
	rows, err := s.Store.Querier().ListMembershipsForOutletByUser(ctx, db.ListMembershipsForOutletByUserParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(userID),
	})
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return fallback, nil
	}
	return rows[0].DisplayName, nil
}

// EntryResponse is the API representation of an attendance entry.
type EntryResponse struct {
	ID              uuid.UUID       `json:"id"`
	OutletID        uuid.UUID       `json:"outletId"`
	UserID          uuid.UUID       `json:"userId"`
	UserName        string          `json:"userName"`
	UserEmail       *string         `json:"userEmail"`
	DisplayName     string          `json:"displayName"`
	Type            EntryType       `json:"type"`
	EntryTime       time.Time       `json:"entryTime"`
	Latitude        decimal.Decimal `json:"latitude"`
	Longitude       decimal.Decimal `json:"longitude"`
	CreatedByUserID *uuid.UUID      `json:"createdByUserId"`
	UpdatedByUserID *uuid.UUID      `json:"updatedByUserId"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// toEntryResponse maps an attendance row together with its user and resolved
// display name to the API representation.
func (s *Service) toEntryResponse(e *db.AttendanceEntry, user *db.User, displayName string) (*EntryResponse, error) {
	lat, err := decimalFromPgNumeric(e.Latitude)
	if err != nil {
		return nil, err
	}
	lon, err := decimalFromPgNumeric(e.Longitude)
	if err != nil {
		return nil, err
	}
	return &EntryResponse{
		ID:              toUUID(e.ID),
		OutletID:        toUUID(e.OutletID),
		UserID:          toUUID(e.UserID),
		UserName:        user.Name,
		UserEmail:       textPtr(user.Email),
		DisplayName:     displayName,
		Type:            EntryType(e.Type),
		EntryTime:       e.EntryTime.Time,
		Latitude:        lat,
		Longitude:       lon,
		CreatedByUserID: uuidPtr(e.CreatedBy),
		UpdatedByUserID: uuidPtr(e.UpdatedBy),
		CreatedAt:       e.CreatedAt.Time,
		UpdatedAt:       e.UpdatedAt.Time,
	}, nil
}

// CreateOwn records the employee's own attendance entry using the current
// server UTC time. Only accepted employees may create entries.
func (s *Service) CreateOwn(ctx context.Context, userID, outletID uuid.UUID, req CreateOwnRequest, ip, userAgent string) (*EntryResponse, error) {
	if err := validateCreateOwn(req); err != nil {
		return nil, err
	}
	m, err := s.getActiveMembership(ctx, outletID, userID, "Outlet membership was not found for the current user")
	if err != nil {
		return nil, err
	}
	o, err := s.assertActiveOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	if err := requireEmployeeRole(m.Role); err != nil {
		return nil, err
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	entry, err := s.Store.Querier().CreateAttendanceEntry(ctx, db.CreateAttendanceEntryParams{
		UserID:    pgUUID(userID),
		OutletID:  pgUUID(outletID),
		Type:      string(req.Type),
		EntryTime: pgtype.Timestamptz{Time: s.Clock().UTC(), Valid: true},
		Latitude:  pgNumericFromDecimal(*req.Latitude),
		Longitude: pgNumericFromDecimal(*req.Longitude),
		CreatedBy: pgUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.created", "mode", "self")
	s.Audit.Record(ctx, userID.String(), "ATTENDANCE_CREATED", "ATTENDANCE_ENTRY", toUUID(entry.ID),
		map[string]any{"outletId": outletID, "userId": userID, "type": entry.Type, "mode": "self"}, ip, userAgent)
	u, err := s.Store.Querier().GetUserByID(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(&entry, &u, m.DisplayName)
}

// CreateManaged records an attendance entry for an employee on behalf of an
// accepted outlet owner.
func (s *Service) CreateManaged(ctx context.Context, ownerID, outletID uuid.UUID, req ManageRequest, ip, userAgent string) (*EntryResponse, error) {
	if err := validateManage(req); err != nil {
		return nil, err
	}
	m, err := s.getActiveMembership(ctx, outletID, ownerID, "Outlet membership was not found for the current user")
	if err != nil {
		return nil, err
	}
	if err := requireOwnerRole(m.Role); err != nil {
		return nil, err
	}
	o, err := s.assertActiveOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	target, err := s.getActiveMembership(ctx, outletID, *req.UserID, "Outlet membership was not found for the requested user")
	if err != nil {
		return nil, err
	}
	if target.Status != "ACCEPTED" || target.Role != "EMPLOYEE" {
		return nil, httpapi.BadRequest("Attendance can only be created for accepted employee memberships")
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	entry, err := s.Store.Querier().CreateAttendanceEntry(ctx, db.CreateAttendanceEntryParams{
		UserID:    pgUUID(*req.UserID),
		OutletID:  pgUUID(outletID),
		Type:      string(req.Type),
		EntryTime: pgtype.Timestamptz{Time: req.EntryTime.UTC(), Valid: true},
		Latitude:  pgNumericFromDecimal(*req.Latitude),
		Longitude: pgNumericFromDecimal(*req.Longitude),
		CreatedBy: pgUUID(ownerID),
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.created", "mode", "managed")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_CREATED", "ATTENDANCE_ENTRY", toUUID(entry.ID),
		map[string]any{"outletId": outletID, "userId": *req.UserID, "type": entry.Type, "mode": "managed"}, ip, userAgent)
	u, err := s.Store.Querier().GetUserByID(ctx, pgUUID(*req.UserID))
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(&entry, &u, target.DisplayName)
}

// Get returns a single attendance entry when the caller is allowed to view it.
// Owners may view any entry; employees may only view their own.
func (s *Service) Get(ctx context.Context, callerID, outletID, entryID uuid.UUID) (*EntryResponse, error) {
	m, err := s.getActiveMembership(ctx, outletID, callerID, "Outlet membership was not found for the current user")
	if err != nil {
		return nil, err
	}
	e, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{
		ID:       pgUUID(entryID),
		OutletID: pgUUID(outletID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Attendance entry not found: " + entryID.String())
	}
	if err != nil {
		return nil, err
	}
	if m.Role != "OWNER" && toUUID(e.UserID) != callerID {
		return nil, httpapi.Forbidden("Employees can only view their own attendance entries")
	}
	u, err := s.Store.Querier().GetUserByID(ctx, pgUUID(toUUID(e.UserID)))
	if err != nil {
		return nil, err
	}
	dn, err := s.memberDisplayName(ctx, outletID, toUUID(e.UserID), u.Name)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(&e, &u, dn)
}

// Update replaces the type, time, and location of an attendance entry on
// behalf of an accepted outlet owner.
func (s *Service) Update(ctx context.Context, ownerID, outletID, entryID uuid.UUID, req UpdateRequest, ip, userAgent string) (*EntryResponse, error) {
	if err := validateUpdate(req); err != nil {
		return nil, err
	}
	m, err := s.getActiveMembership(ctx, outletID, ownerID, "Outlet membership was not found for the current user")
	if err != nil {
		return nil, err
	}
	if err := requireOwnerRole(m.Role); err != nil {
		return nil, err
	}
	o, err := s.assertActiveOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	if _, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{
		ID:       pgUUID(entryID),
		OutletID: pgUUID(outletID),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpapi.NotFound("Attendance entry not found: " + entryID.String())
		}
		return nil, err
	}
	if err := s.enforceGeofence(o, req.Latitude, req.Longitude); err != nil {
		return nil, err
	}
	updated, err := s.Store.Querier().UpdateAttendanceEntry(ctx, db.UpdateAttendanceEntryParams{
		ID:        pgUUID(entryID),
		Type:      string(req.Type),
		EntryTime: pgtype.Timestamptz{Time: req.EntryTime.UTC(), Valid: true},
		Latitude:  pgNumericFromDecimal(*req.Latitude),
		Longitude: pgNumericFromDecimal(*req.Longitude),
		UpdatedBy: pgUUID(ownerID),
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("attendance.updated")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_UPDATED", "ATTENDANCE_ENTRY", entryID,
		map[string]any{"outletId": outletID, "userId": toUUID(updated.UserID), "type": updated.Type}, ip, userAgent)
	u, err := s.Store.Querier().GetUserByID(ctx, pgUUID(toUUID(updated.UserID)))
	if err != nil {
		return nil, err
	}
	dn, err := s.memberDisplayName(ctx, outletID, toUUID(updated.UserID), u.Name)
	if err != nil {
		return nil, err
	}
	return s.toEntryResponse(&updated, &u, dn)
}

// Delete removes an attendance entry on behalf of an accepted outlet owner.
func (s *Service) Delete(ctx context.Context, ownerID, outletID, entryID uuid.UUID, ip, userAgent string) error {
	m, err := s.getActiveMembership(ctx, outletID, ownerID, "Outlet membership was not found for the current user")
	if err != nil {
		return err
	}
	if err := requireOwnerRole(m.Role); err != nil {
		return err
	}
	if _, err := s.assertActiveOutlet(ctx, outletID); err != nil {
		return err
	}
	e, err := s.Store.Querier().GetAttendanceEntryByIDAndOutlet(ctx, db.GetAttendanceEntryByIDAndOutletParams{
		ID:       pgUUID(entryID),
		OutletID: pgUUID(outletID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.NotFound("Attendance entry not found: " + entryID.String())
	}
	if err != nil {
		return err
	}
	if err := s.Store.Querier().DeleteAttendanceEntry(ctx, pgUUID(entryID)); err != nil {
		return err
	}
	s.Metrics.Increment("attendance.deleted")
	s.Audit.Record(ctx, ownerID.String(), "ATTENDANCE_DELETED", "ATTENDANCE_ENTRY", entryID,
		map[string]any{"outletId": outletID, "userId": toUUID(e.UserID), "type": e.Type}, ip, userAgent)
	return nil
}
