package outlet

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func membership(role, status string, removed bool) *db.OutletMembership {
	m := &db.OutletMembership{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Role:        role,
		Status:      status,
		DisplayName: "Alex",
	}
	if removed {
		m.RemovedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	return m
}

func assertAPIError(t *testing.T, err error, wantStatus, wantMsg string) {
	t.Helper()
	ae, ok := err.(*httpapi.APIError)
	if !ok {
		t.Fatalf("err = %v, want *httpapi.APIError", err)
	}
	if ae.Message != wantMsg {
		t.Errorf("message = %q, want %q", ae.Message, wantMsg)
	}
	if wantStatus != "" && ae.Code != wantStatus {
		t.Errorf("code = %q, want %q", ae.Code, wantStatus)
	}
}

func TestInviteConflict(t *testing.T) {
	if err := inviteConflict(nil); err != nil {
		t.Fatalf("nil membership rejected: %v", err)
	}
	if err := inviteConflict(membership("EMPLOYEE", "ACCEPTED", false)); err == nil {
		t.Fatal("active member accepted")
	} else {
		assertAPIError(t, err, "CONFLICT", "User is already an active member of this outlet")
	}
	if err := inviteConflict(membership("EMPLOYEE", "INVITED", false)); err == nil {
		t.Fatal("pending invitee accepted")
	} else {
		assertAPIError(t, err, "CONFLICT", "User already has a pending invitation for this outlet")
	}
	if err := inviteConflict(membership("EMPLOYEE", "REJECTED", false)); err != nil {
		t.Fatalf("rejected membership blocked: %v", err)
	}
	if err := inviteConflict(membership("EMPLOYEE", "ACCEPTED", true)); err != nil {
		t.Fatalf("removed active membership blocked: %v", err)
	}
	if err := inviteConflict(membership("EMPLOYEE", "INVITED", true)); err != nil {
		t.Fatalf("removed invited membership blocked: %v", err)
	}
}

func TestInviteTargetGuard(t *testing.T) {
	id := uuid.New()
	if err := inviteTargetGuard(id, id); err != nil {
		t.Fatalf("same user rejected: %v", err)
	}
	err := inviteTargetGuard(id, uuid.New())
	assertAPIError(t, err, "FORBIDDEN", "You can only manage your own outlet invitations")
}

func TestInviteStatusGuard(t *testing.T) {
	if err := inviteStatusGuard("INVITED", "accepted"); err != nil {
		t.Fatalf("INVITED rejected: %v", err)
	}
	err := inviteStatusGuard("ACCEPTED", "accepted")
	assertAPIError(t, err, "BAD_REQUEST", "Only pending invitations can be accepted")
	err = inviteStatusGuard("ACCEPTED", "rejected")
	assertAPIError(t, err, "BAD_REQUEST", "Only pending invitations can be rejected")
}

func TestLeaveOutletGuard(t *testing.T) {
	if err := leaveOutletGuard("EMPLOYEE"); err != nil {
		t.Fatalf("EMPLOYEE rejected: %v", err)
	}
	err := leaveOutletGuard("OWNER")
	assertAPIError(t, err, "BAD_REQUEST", "Owners cannot leave an outlet through this endpoint")
}

func TestRemoveOwnerGuard(t *testing.T) {
	err := removeOwnerGuard("OWNER")
	assertAPIError(t, err, "BAD_REQUEST", "Owner memberships cannot be removed through this endpoint")
	if err := removeOwnerGuard("EMPLOYEE"); err != nil {
		t.Fatalf("EMPLOYEE rejected: %v", err)
	}
}

func TestMembershipOutletGuard(t *testing.T) {
	outletID := uuid.New()
	if err := membershipOutletGuard(outletID, outletID); err != nil {
		t.Fatalf("matching outlet rejected: %v", err)
	}
	err := membershipOutletGuard(uuid.New(), outletID)
	assertAPIError(t, err, "BAD_REQUEST", "The provided membership does not belong to the requested outlet")
}

func TestMapMembership(t *testing.T) {
	now := time.Now()
	m := db.OutletMembership{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		OutletID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:      pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Role:        "EMPLOYEE",
		Status:      "ACCEPTED",
		DisplayName: "Owner Label",
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}
	user := db.User{
		ID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Name:  "Jane Doe",
		Email: pgtype.Text{String: "a@b.c", Valid: true},
	}
	outlet := db.Outlet{
		ID:              pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Name:            "HQ",
		Latitude:        pgtype.Numeric{Int: big.NewInt(407128000), Exp: -7, Valid: true},
		Longitude:       pgtype.Numeric{Int: big.NewInt(-740060000), Exp: -7, Valid: true},
		RadiusMeters:    50,
		GeofenceEnabled: true,
		CreatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	}
	inviter := &db.User{ID: pgtype.UUID{Bytes: uuid.New(), Valid: true}, Name: "Owner"}

	resp, err := mapMembership(m, user, outlet, inviter)
	if err != nil {
		t.Fatalf("mapMembership: %v", err)
	}
	if resp.UserEmail == nil || *resp.UserEmail != "a@b.c" {
		t.Errorf("UserEmail = %v, want pointer to a@b.c", resp.UserEmail)
	}
	if resp.DisplayName != "Owner Label" {
		t.Errorf("DisplayName = %q, want Owner Label", resp.DisplayName)
	}
	if resp.InvitedByUserID == nil || *resp.InvitedByUserID != toUUID(inviter.ID) {
		t.Errorf("InvitedByUserID = %v, want inviter id", resp.InvitedByUserID)
	}
	if resp.InvitedByUserName == nil || *resp.InvitedByUserName != "Owner" {
		t.Errorf("InvitedByUserName = %v, want Owner", resp.InvitedByUserName)
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("first token: %v", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		t.Fatalf("first token = %v, want {", tok)
	}
	wantOrder := []string{"membershipId", "outlet", "userId", "userName", "userEmail", "displayName", "role", "status", "invitedByUserId", "invitedByUserName", "createdAt", "updatedAt"}
	for i, want := range wantOrder {
		key, err := dec.Token()
		if err != nil {
			t.Fatalf("key %d: %v", i, err)
		}
		if key != want {
			t.Errorf("key %d = %v, want %s", i, key, want)
		}
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("value %d: %v", i, err)
		}
		if i == 5 && v != resp.DisplayName {
			t.Errorf("displayName value = %v, want %s", v, resp.DisplayName)
		}
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var outletJSON map[string]json.RawMessage
	if err := json.Unmarshal(probe["outlet"], &outletJSON); err != nil {
		t.Fatalf("unmarshal outlet: %v", err)
	}
	if got := string(outletJSON["latitude"]); got != "40.7128000" {
		t.Errorf("latitude = %s, want 40.7128000", got)
	}
	if got := string(probe["userEmail"]); got != `"a@b.c"` {
		t.Errorf("userEmail = %s, want \"a@b.c\"", got)
	}

	noEmail := db.User{ID: user.ID, Name: user.Name}
	nullResp, err := mapMembership(m, noEmail, outlet, nil)
	if err != nil {
		t.Fatalf("mapMembership null: %v", err)
	}
	rawNull, err := json.Marshal(nullResp)
	if err != nil {
		t.Fatalf("marshal null: %v", err)
	}
	var probeNull map[string]json.RawMessage
	if err := json.Unmarshal(rawNull, &probeNull); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if got := string(probeNull["userEmail"]); got != "null" {
		t.Errorf("userEmail = %s, want null", got)
	}
	if got := string(probeNull["invitedByUserId"]); got != "null" {
		t.Errorf("invitedByUserId = %s, want null", got)
	}
	if got := string(probeNull["invitedByUserName"]); got != "null" {
		t.Errorf("invitedByUserName = %s, want null", got)
	}
}

func TestUserMembershipSortClause(t *testing.T) {
	p := httpapi.PageParams{Sort: []httpapi.SortOrder{{Field: "displayName", Desc: true}}, Sorted: true}
	clause, _ := p.OrderClause(userMembershipSortable)
	if clause != " ORDER BY display_name DESC" {
		t.Errorf("clause = %q, want \" ORDER BY display_name DESC\"", clause)
	}

	p = httpapi.PageParams{Sort: []httpapi.SortOrder{{Field: "nope", Desc: true}}, Sorted: true}
	clause, _ = p.OrderClause(userMembershipSortable)
	if clause != "" {
		t.Errorf("clause = %q, want empty", clause)
	}
}
