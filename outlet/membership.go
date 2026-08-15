package outlet

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// MembershipResponse is the API representation of a user's membership in an
// outlet, including the outlet and invited-by context. The user email and the
// invited-by fields are pointers so that absent values are serialized as JSON
// null rather than empty strings.
type MembershipResponse struct {
	MembershipID      uuid.UUID       `json:"membershipId"`
	Outlet            *OutletResponse `json:"outlet"`
	UserID            uuid.UUID       `json:"userId"`
	UserName          string          `json:"userName"`
	UserEmail         *string         `json:"userEmail"`
	DisplayName       string          `json:"displayName"`
	Role              string          `json:"role"`
	Status            string          `json:"status"`
	InvitedByUserID   *uuid.UUID      `json:"invitedByUserId"`
	InvitedByUserName *string         `json:"invitedByUserName"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

// mapMembership maps a membership row together with its user, outlet, and
// optional inviter to the API representation.
func mapMembership(m db.OutletMembership, user db.User, outlet db.Outlet, inviter *db.User) (MembershipResponse, error) {
	out, err := toOutletResponse(&outlet)
	if err != nil {
		return MembershipResponse{}, err
	}
	var invitedByUserID *uuid.UUID
	var invitedByUserName *string
	if inviter != nil {
		invitedByUserID = uuidPtr(inviter.ID)
		invitedByUserName = strPtr(inviter.Name)
	}
	return MembershipResponse{
		MembershipID:      toUUID(m.ID),
		Outlet:            out,
		UserID:            toUUID(m.UserID),
		UserName:          user.Name,
		UserEmail:         textPtr(user.Email),
		DisplayName:       m.DisplayName,
		Role:              m.Role,
		Status:            m.Status,
		InvitedByUserID:   invitedByUserID,
		InvitedByUserName: invitedByUserName,
		CreatedAt:         m.CreatedAt.Time,
		UpdatedAt:         m.UpdatedAt.Time,
	}, nil
}

// loadMembershipDetails loads a non-removed membership together with its
// user, outlet, and optional inviter.
func (s *Service) loadMembershipDetails(ctx context.Context, membershipID uuid.UUID) (db.GetMembershipDetailsByIDRow, error) {
	row, err := s.Store.Querier().GetMembershipDetailsByID(ctx, pgUUID(membershipID))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.GetMembershipDetailsByIDRow{}, httpapi.NotFound("Outlet membership not found: " + membershipID.String())
	}
	if err != nil {
		return db.GetMembershipDetailsByIDRow{}, err
	}
	return row, nil
}

// detailsParts splits a loaded membership detail row into the membership,
// user, outlet, and inviter values used by mapMembership.
func detailsParts(row db.GetMembershipDetailsByIDRow) (db.OutletMembership, db.User, db.Outlet, *db.User) {
	m := db.OutletMembership{
		ID:              row.ID,
		OutletID:        row.OutletID,
		UserID:          row.UserID,
		Role:            row.Role,
		Status:          row.Status,
		DisplayName:     row.DisplayName,
		InvitedByUserID: row.InvitedByUserID,
		RemovedAt:       row.RemovedAt,
		RemovedByUserID: row.RemovedByUserID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
	user := db.User{ID: row.UserID_2, Name: row.UserName, Email: row.UserEmail}
	outlet := db.Outlet{
		ID:              row.OutletID_2,
		Name:            row.OutletName,
		Latitude:        row.Latitude,
		Longitude:       row.Longitude,
		RadiusMeters:    row.RadiusMeters,
		GeofenceEnabled: row.GeofenceEnabled,
		CreatedAt:       row.OutletCreatedAt,
		UpdatedAt:       row.OutletUpdatedAt,
	}
	var inviter *db.User
	if row.InvitedByUserID_2.Valid {
		inviter = &db.User{ID: row.InvitedByUserID_2, Name: textOrEmpty(row.InvitedByUserName)}
	}
	return m, user, outlet, inviter
}

// inviteConflict reports whether an existing membership blocks re-inviting
// its user. Active members and pending invitees conflict; rejected and removed
// memberships may be reopened.
func inviteConflict(existing *db.OutletMembership) error {
	if existing == nil {
		return nil
	}
	if !existing.RemovedAt.Valid && existing.Status == "ACCEPTED" {
		return httpapi.Conflict("User is already an active member of this outlet")
	}
	if !existing.RemovedAt.Valid && existing.Status == "INVITED" {
		return httpapi.Conflict("User already has a pending invitation for this outlet")
	}
	return nil
}

// inviteTargetGuard rejects attempts to accept or reject an invitation that
// belongs to another user.
func inviteTargetGuard(memberUserID, currentUserID uuid.UUID) error {
	if memberUserID != currentUserID {
		return httpapi.Forbidden("You can only manage your own outlet invitations")
	}
	return nil
}

// inviteStatusGuard rejects accept and reject operations on memberships whose
// status is not pending. verb describes the attempted operation ("accepted" or
// "rejected").
func inviteStatusGuard(status, verb string) error {
	if status != "INVITED" {
		return httpapi.BadRequest("Only pending invitations can be " + verb)
	}
	return nil
}

// leaveOutletGuard rejects owner self-removal through the leave endpoint.
func leaveOutletGuard(role string) error {
	if role == "OWNER" {
		return httpapi.BadRequest("Owners cannot leave an outlet through this endpoint")
	}
	return nil
}

// removeOwnerGuard rejects owner membership removal by the owner of an outlet.
func removeOwnerGuard(role string) error {
	if role == "OWNER" {
		return httpapi.BadRequest("Owner memberships cannot be removed through this endpoint")
	}
	return nil
}

// membershipOutletGuard rejects operations on a membership that belongs to a
// different outlet than the one in the request path.
func membershipOutletGuard(memberOutletID, outletID uuid.UUID) error {
	if memberOutletID != outletID {
		return httpapi.BadRequest("The provided membership does not belong to the requested outlet")
	}
	return nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique constraint
// violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// InviteMember invites an existing active user to join the outlet as an
// employee. A user whose previous membership was rejected or removed is
// re-invited by reopening that membership.
func (s *Service) InviteMember(ctx context.Context, ownerID, outletID uuid.UUID, email, ip, userAgent string) (*MembershipResponse, error) {
	inviter, err := s.assertOwner(ctx, outletID, ownerID)
	if err != nil {
		return nil, err
	}
	invitee, err := s.Store.Querier().GetUserByEmailCaseInsensitive(ctx, strings.TrimSpace(email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.NotFound("No active user found for email: " + strings.TrimSpace(email))
	}
	if err != nil {
		return nil, err
	}
	existing, err := s.Store.Querier().GetMembershipByOutletAndUserIncludingRemoved(ctx, db.GetMembershipByOutletAndUserIncludingRemovedParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(toUUID(invitee.ID)),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		existing = db.OutletMembership{}
		err = nil
	}
	if err != nil {
		return nil, err
	}
	var m db.OutletMembership
	if existing.ID.Valid {
		if err := inviteConflict(&existing); err != nil {
			return nil, err
		}
		m, err = s.Store.Querier().UpdateMembershipInvite(ctx, db.UpdateMembershipInviteParams{
			ID:              existing.ID,
			Role:            "EMPLOYEE",
			Status:          "INVITED",
			InvitedByUserID: pgUUID(toUUID(inviter.ID)),
		})
	} else {
		if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
			return nil, err
		}
		m, err = s.Store.Querier().CreateMembership(ctx, db.CreateMembershipParams{
			OutletID:        pgUUID(outletID),
			UserID:          pgUUID(toUUID(invitee.ID)),
			Role:            "EMPLOYEE",
			Status:          "INVITED",
			DisplayName:     invitee.Name,
			InvitedByUserID: pgUUID(toUUID(inviter.ID)),
		})
	}
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpapi.Conflict("User already has a membership record for this outlet")
		}
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.invited")
	s.Audit.Record(ctx, inviter.ID.String(), "OUTLET_MEMBER_INVITED", "OUTLET_MEMBERSHIP", toUUID(m.ID),
		map[string]any{"outletId": toUUID(m.OutletID), "inviteeUserId": toUUID(invitee.ID)}, ip, userAgent)
	row, err := s.loadMembershipDetails(ctx, toUUID(m.ID))
	if err != nil {
		return nil, err
	}
	mem, user, out, inviterUser := detailsParts(row)
	resp, err := mapMembership(mem, user, out, inviterUser)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// AcceptInvite accepts a pending outlet invitation on behalf of its target
// user.
func (s *Service) AcceptInvite(ctx context.Context, userID, membershipID uuid.UUID, ip, userAgent string) (*MembershipResponse, error) {
	row, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	m, user, outlet, inviter := detailsParts(row)
	if err := inviteTargetGuard(toUUID(m.UserID), userID); err != nil {
		return nil, err
	}
	if err := inviteStatusGuard(m.Status, "accepted"); err != nil {
		return nil, err
	}
	updated, err := s.Store.Querier().UpdateMembershipStatus(ctx, db.UpdateMembershipStatusParams{ID: m.ID, Status: "ACCEPTED"})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.accepted")
	s.Audit.Record(ctx, userID.String(), "OUTLET_INVITE_ACCEPTED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": toUUID(m.OutletID)}, ip, userAgent)
	resp, err := mapMembership(updated, user, outlet, inviter)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// RejectInvite rejects a pending outlet invitation on behalf of its target
// user.
func (s *Service) RejectInvite(ctx context.Context, userID, membershipID uuid.UUID, ip, userAgent string) (*MembershipResponse, error) {
	row, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	m, user, outlet, inviter := detailsParts(row)
	if err := inviteTargetGuard(toUUID(m.UserID), userID); err != nil {
		return nil, err
	}
	if err := inviteStatusGuard(m.Status, "rejected"); err != nil {
		return nil, err
	}
	updated, err := s.Store.Querier().UpdateMembershipStatus(ctx, db.UpdateMembershipStatusParams{ID: m.ID, Status: "REJECTED"})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.rejected")
	s.Audit.Record(ctx, userID.String(), "OUTLET_INVITE_REJECTED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": toUUID(m.OutletID)}, ip, userAgent)
	resp, err := mapMembership(updated, user, outlet, inviter)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// LeaveOutlet soft-removes the current user's own membership in the outlet so
// that access is revoked while historical attendance stays valid.
func (s *Service) LeaveOutlet(ctx context.Context, userID, outletID uuid.UUID, ip, userAgent string) error {
	currentUser, err := s.getActiveUser(ctx, userID)
	if err != nil {
		return err
	}
	m, err := s.Store.Querier().GetMembershipByOutletAndUser(ctx, db.GetMembershipByOutletAndUserParams{
		OutletID: pgUUID(outletID),
		UserID:   pgUUID(userID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.NotFound("Outlet membership was not found for the current user")
	}
	if err != nil {
		return err
	}
	if err := leaveOutletGuard(m.Role); err != nil {
		return err
	}
	if _, err := s.Store.Querier().RemoveMembership(ctx, db.RemoveMembershipParams{
		ID:              m.ID,
		RemovedByUserID: pgUUID(toUUID(currentUser.ID)),
	}); err != nil {
		return err
	}
	s.Metrics.Increment("outlet.membership.left")
	s.Audit.Record(ctx, userID.String(), "OUTLET_MEMBERSHIP_LEFT", "OUTLET_MEMBERSHIP", toUUID(m.ID),
		map[string]any{"outletId": outletID}, ip, userAgent)
	return nil
}

// RemoveMembership soft-removes an employee's membership in an outlet on
// behalf of its owner.
func (s *Service) RemoveMembership(ctx context.Context, ownerID, outletID, membershipID uuid.UUID, ip, userAgent string) error {
	owner, err := s.assertOwner(ctx, outletID, ownerID)
	if err != nil {
		return err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return err
	}
	row, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return err
	}
	m, _, _, _ := detailsParts(row)
	if err := membershipOutletGuard(toUUID(m.OutletID), outletID); err != nil {
		return err
	}
	if err := removeOwnerGuard(m.Role); err != nil {
		return err
	}
	if _, err := s.Store.Querier().RemoveMembership(ctx, db.RemoveMembershipParams{
		ID:              m.ID,
		RemovedByUserID: pgUUID(toUUID(owner.ID)),
	}); err != nil {
		return err
	}
	s.Metrics.Increment("outlet.membership.removed")
	s.Audit.Record(ctx, ownerID.String(), "OUTLET_MEMBERSHIP_REMOVED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": outletID, "removedUserId": toUUID(m.UserID)}, ip, userAgent)
	return nil
}

// UpdateDisplayName sets the owner-controlled display name of a member of the
// outlet.
func (s *Service) UpdateDisplayName(ctx context.Context, ownerID, outletID, membershipID uuid.UUID, name, ip, userAgent string) (*MembershipResponse, error) {
	if _, err := s.assertOwner(ctx, outletID, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.getActiveOutlet(ctx, outletID); err != nil {
		return nil, err
	}
	row, err := s.loadMembershipDetails(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	m, user, outlet, inviter := detailsParts(row)
	if err := membershipOutletGuard(toUUID(m.OutletID), outletID); err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(name)
	updated, err := s.Store.Querier().UpdateMembershipDisplayName(ctx, db.UpdateMembershipDisplayNameParams{
		ID:          m.ID,
		DisplayName: displayName,
	})
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("outlet.membership.display_name.updated")
	s.Audit.Record(ctx, ownerID.String(), "OUTLET_MEMBERSHIP_DISPLAY_NAME_UPDATED", "OUTLET_MEMBERSHIP", membershipID,
		map[string]any{"outletId": outletID, "userId": toUUID(m.UserID), "displayName": displayName}, ip, userAgent)
	resp, err := mapMembership(updated, user, outlet, inviter)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// textOrEmpty returns the string value of t, or "" when t is not valid.
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

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

// strPtr returns a pointer to s.
func strPtr(s string) *string {
	return &s
}
