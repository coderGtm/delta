// Package outlet implements the outlet CRUD, geofence, and membership use
// cases.
package outlet

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/decimal"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service coordinates the outlet CRUD, geofence, and membership use cases.
type Service struct {
	Store   *db.Store
	Audit   *audit.Recorder
	Metrics *metrics.Registry
}

// NewService returns a Service wired to the given dependencies.
func NewService(store *db.Store, a *audit.Recorder, m *metrics.Registry) *Service {
	return &Service{Store: store, Audit: a, Metrics: m}
}

// CreateOutletRequest is the payload used to create a new outlet. Pointer
// fields distinguish a missing value from an explicit zero.
type CreateOutletRequest struct {
	Name         string           `json:"name"`
	Latitude     *decimal.Decimal `json:"latitude"`
	Longitude    *decimal.Decimal `json:"longitude"`
	RadiusMeters *int             `json:"radiusMeters"`
}

// UpdateOutletRequest is the payload used to replace the editable details of
// an outlet. Pointer fields distinguish a missing value from an explicit zero.
type UpdateOutletRequest struct {
	Name         string           `json:"name"`
	Latitude     *decimal.Decimal `json:"latitude"`
	Longitude    *decimal.Decimal `json:"longitude"`
	RadiusMeters *int             `json:"radiusMeters"`
}

// OutletResponse is the API representation of an outlet.
type OutletResponse struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Latitude        decimal.Decimal `json:"latitude"`
	Longitude       decimal.Decimal `json:"longitude"`
	RadiusMeters    int             `json:"radiusMeters"`
	GeofenceEnabled bool            `json:"geofenceEnabled"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

// validateOutlet checks the request fields and returns a VALIDATION_ERROR API
// error listing every failing constraint in field order, or nil when the
// request is valid.
func validateOutlet(name string, lat, lon *decimal.Decimal, radius *int) *httpapi.APIError {
	var msgs []string
	if strings.TrimSpace(name) == "" {
		msgs = append(msgs, "Outlet name is required")
	} else if utf8.RuneCountInString(name) > 150 {
		msgs = append(msgs, "Outlet name must be at most 150 characters")
	}
	if lat == nil {
		msgs = append(msgs, "Latitude is required")
	} else {
		if lat.CmpInt(-90) < 0 {
			msgs = append(msgs, "Latitude must be greater than or equal to -90")
		}
		if lat.CmpInt(90) > 0 {
			msgs = append(msgs, "Latitude must be less than or equal to 90")
		}
	}
	if lon == nil {
		msgs = append(msgs, "Longitude is required")
	} else {
		if lon.CmpInt(-180) < 0 {
			msgs = append(msgs, "Longitude must be greater than or equal to -180")
		}
		if lon.CmpInt(180) > 0 {
			msgs = append(msgs, "Longitude must be less than or equal to 180")
		}
	}
	if radius == nil {
		msgs = append(msgs, "Radius in meters is required")
	} else if *radius <= 0 {
		msgs = append(msgs, "Radius in meters must be greater than zero")
	} else if *radius > math.MaxInt32 {
		msgs = append(msgs, "Radius in meters must be less than or equal to 2147483647")
	}
	if len(msgs) == 0 {
		return nil
	}
	return httpapi.Validation(strings.Join(msgs, ", "))
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
		return decimal.Decimal{}, fmt.Errorf("outlet: invalid numeric value")
	}
	shift := n.Exp + 7
	if shift < 0 {
		return decimal.Decimal{}, fmt.Errorf("outlet: numeric value has more than 7 fractional digits")
	}
	coeff := new(big.Int).Set(n.Int)
	if shift > 0 {
		coeff.Mul(coeff, tenTo(shift))
	}
	return decimal.FromBigInt(coeff, 7), nil
}

// toOutletResponse maps an outlet row to its API representation.
func toOutletResponse(o *db.Outlet) (*OutletResponse, error) {
	lat, err := decimalFromPgNumeric(o.Latitude)
	if err != nil {
		return nil, err
	}
	lon, err := decimalFromPgNumeric(o.Longitude)
	if err != nil {
		return nil, err
	}
	return &OutletResponse{
		ID:              toUUID(o.ID),
		Name:            o.Name,
		Latitude:        lat,
		Longitude:       lon,
		RadiusMeters:    int(o.RadiusMeters),
		GeofenceEnabled: o.GeofenceEnabled,
		CreatedAt:       o.CreatedAt.Time,
		UpdatedAt:       o.UpdatedAt.Time,
	}, nil
}

// getActiveUser loads the non-deleted user with the given ID.
func (s *Service) getActiveUser(ctx context.Context, userID uuid.UUID) (*db.User, error) {
	user, err := s.Store.Querier().GetUserByID(ctx, pgUUID(userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Authenticated user was not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// getActiveOutlet loads the non-removed outlet with the given ID.
func (s *Service) getActiveOutlet(ctx context.Context, outletID uuid.UUID) (*db.Outlet, error) {
	out, err := s.Store.Querier().GetOutletByID(ctx, pgUUID(outletID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet not found: " + outletID.String())
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// getActiveMembership loads the current user's active membership in the
// outlet, requiring it to exist and to be accepted.
func (s *Service) getActiveMembership(ctx context.Context, outletID, userID uuid.UUID) (*db.OutletMembership, error) {
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return nil, err
	}
	if m.Status != "ACCEPTED" {
		return nil, httpapi.Forbidden("You must accept the outlet invitation before accessing this outlet")
	}
	return &m, nil
}

// assertOwnerRole returns a FORBIDDEN API error unless role is the owner role.
func assertOwnerRole(role string) error {
	if role != "OWNER" {
		return httpapi.Forbidden("Only outlet owners can perform this action")
	}
	return nil
}

// assertOwner verifies that the user is an accepted owner of the outlet and
// returns the active user.
func (s *Service) assertOwner(ctx context.Context, outletID, userID uuid.UUID) (*db.User, error) {
	m, err := s.getActiveMembership(ctx, outletID, userID)
	if err != nil {
		return nil, err
	}
	if err := assertOwnerRole(m.Role); err != nil {
		return nil, err
	}
	return s.getActiveUser(ctx, userID)
}

// CreateOutlet creates a new outlet and an accepted owner membership for the
// creator.
func (s *Service) CreateOutlet(ctx context.Context, userID uuid.UUID, req CreateOutletRequest, ip, userAgent string) (*OutletResponse, error) {
	if err := validateOutlet(req.Name, req.Latitude, req.Longitude, req.RadiusMeters); err != nil {
		return nil, err
	}
	user, err := s.getActiveUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	lat := pgNumericFromDecimal(*req.Latitude)
	lon := pgNumericFromDecimal(*req.Longitude)
	var out db.Outlet
	err = s.Store.Tx(ctx, func(q db.Querier) error {
		o, err := q.CreateOutlet(ctx, db.CreateOutletParams{
			Name:         strings.TrimSpace(req.Name),
			Latitude:     lat,
			Longitude:    lon,
			RadiusMeters: int32(*req.RadiusMeters),
		})
		if err != nil {
			return err
		}
		out = o
		_, err = q.CreateMembership(ctx, db.CreateMembershipParams{
			OutletID:        o.ID,
			UserID:          pgUUID(userID),
			Role:            "OWNER",
			Status:          "ACCEPTED",
			DisplayName:     user.Name,
			InvitedByUserID: pgtype.UUID{},
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.created")
	s.Audit.Record(ctx, userID.String(), "OUTLET_CREATED", "OUTLET", toUUID(out.ID), map[string]any{"name": out.Name}, ip, userAgent)
	return toOutletResponse(&out)
}

// GetOutlet returns the outlet details for a user with an accepted membership.
func (s *Service) GetOutlet(ctx context.Context, userID, outletID uuid.UUID) (*OutletResponse, error) {
	out, err := s.getActiveOutlet(ctx, outletID)
	if err != nil {
		return nil, err
	}
	if _, err := s.getActiveMembership(ctx, outletID, userID); err != nil {
		return nil, err
	}
	return toOutletResponse(out)
}

// UpdateOutlet replaces the core editable details of an outlet.
func (s *Service) UpdateOutlet(ctx context.Context, userID, outletID uuid.UUID, req UpdateOutletRequest, ip, userAgent string) (*OutletResponse, error) {
	if err := validateOutlet(req.Name, req.Latitude, req.Longitude, req.RadiusMeters); err != nil {
		return nil, err
	}
	if _, err := s.assertOwner(ctx, outletID, userID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	lat := pgNumericFromDecimal(*req.Latitude)
	lon := pgNumericFromDecimal(*req.Longitude)
	updated, err := s.Store.Querier().UpdateOutlet(ctx, db.UpdateOutletParams{
		ID:           pgUUID(outletID),
		Name:         strings.TrimSpace(req.Name),
		Latitude:     lat,
		Longitude:    lon,
		RadiusMeters: int32(*req.RadiusMeters),
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.updated")
	s.Audit.Record(ctx, userID.String(), "OUTLET_UPDATED", "OUTLET", outletID, map[string]any{"name": updated.Name}, ip, userAgent)
	return toOutletResponse(&updated)
}

// UpdateGeofence toggles attendance geofence enforcement for an outlet. Only
// accepted outlet owners may perform this action.
func (s *Service) UpdateGeofence(ctx context.Context, userID, outletID uuid.UUID, enabled bool, ip, userAgent string) (*OutletResponse, error) {
	if _, err := s.assertOwner(ctx, outletID, userID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	updated, err := s.Store.Querier().UpdateOutletGeofence(ctx, db.UpdateOutletGeofenceParams{
		ID:              pgUUID(outletID),
		GeofenceEnabled: enabled,
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.geofence.updated", "enabled", strconv.FormatBool(updated.GeofenceEnabled))
	s.Audit.Record(ctx, userID.String(), "OUTLET_GEOFENCE_UPDATED", "OUTLET", outletID, map[string]any{"geofenceEnabled": updated.GeofenceEnabled}, ip, userAgent)
	return toOutletResponse(&updated)
}

// DeleteOutlet soft-deletes an outlet on behalf of an accepted owner,
// preserving historical membership and attendance records.
func (s *Service) DeleteOutlet(ctx context.Context, userID, outletID uuid.UUID, ip, userAgent string) error {
	user, err := s.assertOwner(ctx, outletID, userID)
	if err != nil {
		return err
	}
	out, err := s.getActiveOutlet(ctx, outletID)
	if err != nil {
		return err
	}
	if _, err := s.Store.Querier().DeleteOutlet(ctx, db.DeleteOutletParams{
		ID:              pgUUID(outletID),
		RemovedByUserID: user.ID,
	}); err != nil {
		return err
	}
	s.Metrics.Increment("outlet.deleted")
	s.Audit.Record(ctx, userID.String(), "OUTLET_DELETED", "OUTLET", outletID, map[string]any{"name": out.Name}, ip, userAgent)
	return nil
}

// pgUUID wraps id in a pgtype.UUID marked valid.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// toUUID unwraps the bytes of a pgtype.UUID back into a uuid.UUID.
func toUUID(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

// tenTo returns 10 raised to the n-th power.
func tenTo(n int32) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
