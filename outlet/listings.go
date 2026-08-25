package outlet

import (
	"context"
	"fmt"

	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// userMembershipRow is a membership row joined with its user and outlet for
// the user-facing listing queries. The db tags match the column aliases of
// userMembershipsSQL exactly, as required by pgx.RowToStructByName.
type userMembershipRow struct {
	MembershipID                  pgtype.UUID        `db:"membership_id"`
	UserID                        pgtype.UUID        `db:"user_id"`
	Role                          string             `db:"role"`
	Status                        string             `db:"status"`
	DisplayName                   string             `db:"display_name"`
	InvitedByUserID               pgtype.UUID        `db:"invited_by_user_id"`
	CreatedAt                     pgtype.Timestamptz `db:"created_at"`
	UpdatedAt                     pgtype.Timestamptz `db:"updated_at"`
	MemberUserID                  pgtype.UUID        `db:"member_user_id"`
	UserName                      string             `db:"user_name"`
	UserEmail                     pgtype.Text        `db:"user_email"`
	OutletID                      pgtype.UUID        `db:"outlet_id"`
	OutletName                    string             `db:"outlet_name"`
	Latitude                      pgtype.Numeric     `db:"latitude"`
	Longitude                     pgtype.Numeric     `db:"longitude"`
	RadiusMeters                  int32              `db:"radius_meters"`
	GeofenceEnabled               bool               `db:"geofence_enabled"`
	ShowRecentEntriesToEmployees  bool               `db:"show_recent_entries_to_employees"`
	ShowTotalTimeTodayToEmployees bool               `db:"show_total_time_today_to_employees"`
	OutletCreatedAt               pgtype.Timestamptz `db:"outlet_created_at"`
	OutletUpdatedAt               pgtype.Timestamptz `db:"outlet_updated_at"`
}

// outletMembershipRow is a membership row joined with its user, outlet, and
// inviter for the outlet-facing listing query. The db tags match the column
// aliases of outletMembershipsSQL exactly, as required by
// pgx.RowToStructByName. OutletID is the membership's outlet id; MemberOutletID
// is the id of the outlet row itself.
type outletMembershipRow struct {
	MembershipID                  pgtype.UUID        `db:"membership_id"`
	OutletID                      pgtype.UUID        `db:"outlet_id"`
	UserID                        pgtype.UUID        `db:"user_id"`
	Role                          string             `db:"role"`
	Status                        string             `db:"status"`
	DisplayName                   string             `db:"display_name"`
	InvitedByUserID               pgtype.UUID        `db:"invited_by_user_id"`
	CreatedAt                     pgtype.Timestamptz `db:"created_at"`
	UpdatedAt                     pgtype.Timestamptz `db:"updated_at"`
	MemberUserID                  pgtype.UUID        `db:"member_user_id"`
	UserName                      string             `db:"user_name"`
	UserEmail                     pgtype.Text        `db:"user_email"`
	MemberOutletID                pgtype.UUID        `db:"outlet_id_2"`
	OutletName                    string             `db:"outlet_name"`
	Latitude                      pgtype.Numeric     `db:"latitude"`
	Longitude                     pgtype.Numeric     `db:"longitude"`
	RadiusMeters                  int32              `db:"radius_meters"`
	GeofenceEnabled               bool               `db:"geofence_enabled"`
	ShowRecentEntriesToEmployees  bool               `db:"show_recent_entries_to_employees"`
	ShowTotalTimeTodayToEmployees bool               `db:"show_total_time_today_to_employees"`
	OutletCreatedAt               pgtype.Timestamptz `db:"outlet_created_at"`
	OutletUpdatedAt               pgtype.Timestamptz `db:"outlet_updated_at"`
	InviterUserID                 pgtype.UUID        `db:"inviter_user_id"`
	InviterUserName               pgtype.Text        `db:"inviter_user_name"`
}

// userMembershipsSQL lists a user's memberships in active outlets by status.
// The %s placeholder is replaced with a whitelisted ORDER BY clause.
const userMembershipsSQL = `SELECT m.id AS membership_id, m.user_id AS user_id, m.role AS role, m.status AS status,
	m.display_name AS display_name, m.invited_by_user_id AS invited_by_user_id,
	m.created_at AS created_at, m.updated_at AS updated_at,
	u.id AS member_user_id, u.name AS user_name, u.email AS user_email,
	o.id AS outlet_id, o.name AS outlet_name, o.latitude AS latitude, o.longitude AS longitude,
	o.radius_meters AS radius_meters, o.geofence_enabled AS geofence_enabled,
	o.show_recent_entries_to_employees AS show_recent_entries_to_employees, o.show_total_time_today_to_employees AS show_total_time_today_to_employees,
	o.created_at AS outlet_created_at, o.updated_at AS outlet_updated_at
FROM outlet_memberships m
JOIN users u ON u.id = m.user_id
JOIN outlets o ON o.id = m.outlet_id
WHERE m.user_id = $1 AND m.status = $2 AND m.removed_at IS NULL AND o.removed_at IS NULL
%s
LIMIT $3 OFFSET $4`

// outletMembershipsSQL lists the non-removed memberships of an outlet together
// with each member's user and inviter. The %s placeholder is replaced with a
// whitelisted ORDER BY clause.
const outletMembershipsSQL = `SELECT m.id AS membership_id, m.outlet_id AS outlet_id, m.user_id AS user_id, m.role AS role, m.status AS status,
	m.display_name AS display_name, m.invited_by_user_id AS invited_by_user_id,
	m.created_at AS created_at, m.updated_at AS updated_at,
	u.id AS member_user_id, u.name AS user_name, u.email AS user_email,
	o.id AS outlet_id_2, o.name AS outlet_name, o.latitude AS latitude, o.longitude AS longitude,	o.radius_meters AS radius_meters, o.geofence_enabled AS geofence_enabled,
	o.show_recent_entries_to_employees AS show_recent_entries_to_employees, o.show_total_time_today_to_employees AS show_total_time_today_to_employees,
	o.created_at AS outlet_created_at, o.updated_at AS outlet_updated_at,
	iu.id AS inviter_user_id, iu.name AS inviter_user_name
FROM outlet_memberships m
JOIN users u ON u.id = m.user_id
JOIN outlets o ON o.id = m.outlet_id
LEFT JOIN users iu ON iu.id = m.invited_by_user_id
WHERE m.outlet_id = $1 AND m.removed_at IS NULL
%s
LIMIT $2 OFFSET $3`

// userMembershipSortable maps client sort field names to their SQL column
// aliases for the user-facing membership listings.
var userMembershipSortable = map[string]string{
	"id":          "membership_id",
	"updatedAt":   "updated_at",
	"createdAt":   "created_at",
	"displayName": "display_name",
	"status":      "status",
	"role":        "role",
	"outletName":  "outlet_name",
}

// outletMembershipSortable maps client sort field names to their SQL column
// aliases for the outlet-facing membership listing.
var outletMembershipSortable = map[string]string{
	"id":                "membership_id",
	"updatedAt":         "updated_at",
	"createdAt":         "created_at",
	"displayName":       "display_name",
	"status":            "status",
	"role":              "role",
	"userName":          "user_name",
	"userEmail":         "user_email",
	"invitedByUserName": "inviter_user_name",
}

// GetMyOutlets returns the pages of outlets the user has accepted membership
// in.
func (s *Service) GetMyOutlets(ctx context.Context, userID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	return s.listUserMemberships(ctx, userID, "ACCEPTED", p)
}

// GetMyInvites returns the pages of pending outlet invitations for the user.
func (s *Service) GetMyInvites(ctx context.Context, userID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	return s.listUserMemberships(ctx, userID, "INVITED", p)
}

// GetOutletMemberships returns the pages of memberships of an outlet on behalf
// of its accepted owner.
func (s *Service) GetOutletMemberships(ctx context.Context, ownerID, outletID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	if _, err := s.assertOwner(ctx, outletID, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	return s.listOutletMemberships(ctx, outletID, p)
}

// listUserMemberships queries the user's memberships with the given status,
// honoring a whitelisted client sort and defaulting to most recently updated
// first.
func (s *Service) listUserMemberships(ctx context.Context, userID uuid.UUID, status string, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	order, _ := p.OrderClause(userMembershipSortable)
	if order == "" {
		order = " ORDER BY updated_at DESC"
	}
	rows, err := s.Store.Pool().Query(ctx, fmt.Sprintf(userMembershipsSQL, order), pgUUID(userID), status, int32(p.Size), int32(p.Page*p.Size))
	if err != nil {
		return nil, err
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[userMembershipRow])
	if err != nil {
		return nil, err
	}
	total, err := s.Store.Querier().CountMembershipsForUserByStatus(ctx, db.CountMembershipsForUserByStatusParams{UserID: pgUUID(userID), Status: status})
	if err != nil {
		return nil, err
	}
	out := make([]MembershipResponse, 0, len(collected))
	for _, row := range collected {
		m := db.OutletMembership{
			ID:              row.MembershipID,
			OutletID:        row.OutletID,
			UserID:          row.UserID,
			Role:            row.Role,
			Status:          row.Status,
			DisplayName:     row.DisplayName,
			InvitedByUserID: row.InvitedByUserID,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
		user := db.User{ID: row.MemberUserID, Name: row.UserName, Email: row.UserEmail}
		outlet := db.Outlet{
			ID:                            row.OutletID,
			Name:                          row.OutletName,
			Latitude:                      row.Latitude,
			Longitude:                     row.Longitude,
			RadiusMeters:                  row.RadiusMeters,
			GeofenceEnabled:               row.GeofenceEnabled,
			ShowRecentEntriesToEmployees:  row.ShowRecentEntriesToEmployees,
			ShowTotalTimeTodayToEmployees: row.ShowTotalTimeTodayToEmployees,
			CreatedAt:                     row.OutletCreatedAt,
			UpdatedAt:                     row.OutletUpdatedAt,
		}
		item, err := mapMembership(m, user, outlet, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return httpapi.NewPageResponse(out, total, p), nil
}

// listOutletMemberships queries the memberships of an outlet, honoring a
// whitelisted client sort and defaulting to oldest first.
func (s *Service) listOutletMemberships(ctx context.Context, outletID uuid.UUID, p httpapi.PageParams) (*httpapi.PageResponse[MembershipResponse], error) {
	order, _ := p.OrderClause(outletMembershipSortable)
	if order == "" {
		order = " ORDER BY created_at ASC"
	}
	rows, err := s.Store.Pool().Query(ctx, fmt.Sprintf(outletMembershipsSQL, order), pgUUID(outletID), int32(p.Size), int32(p.Page*p.Size))
	if err != nil {
		return nil, err
	}
	collected, err := pgx.CollectRows(rows, pgx.RowToStructByName[outletMembershipRow])
	if err != nil {
		return nil, err
	}
	total, err := s.Store.Querier().CountMembershipsForOutlet(ctx, pgUUID(outletID))
	if err != nil {
		return nil, err
	}
	out := make([]MembershipResponse, 0, len(collected))
	for _, row := range collected {
		m := db.OutletMembership{
			ID:              row.MembershipID,
			OutletID:        row.OutletID,
			UserID:          row.UserID,
			Role:            row.Role,
			Status:          row.Status,
			DisplayName:     row.DisplayName,
			InvitedByUserID: row.InvitedByUserID,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
		user := db.User{ID: row.MemberUserID, Name: row.UserName, Email: row.UserEmail}
		outlet := db.Outlet{
			ID:                            row.MemberOutletID,
			Name:                          row.OutletName,
			Latitude:                      row.Latitude,
			Longitude:                     row.Longitude,
			RadiusMeters:                  row.RadiusMeters,
			GeofenceEnabled:               row.GeofenceEnabled,
			ShowRecentEntriesToEmployees:  row.ShowRecentEntriesToEmployees,
			ShowTotalTimeTodayToEmployees: row.ShowTotalTimeTodayToEmployees,
			CreatedAt:                     row.OutletCreatedAt,
			UpdatedAt:                     row.OutletUpdatedAt,
		}
		var inviter *db.User
		if row.InviterUserID.Valid {
			inviter = &db.User{ID: row.InviterUserID, Name: textOrEmpty(row.InviterUserName)}
		}
		item, err := mapMembership(m, user, outlet, inviter)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return httpapi.NewPageResponse(out, total, p), nil
}
