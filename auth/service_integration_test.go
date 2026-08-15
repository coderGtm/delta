// Package auth_test contains integration tests for the auth package that run
// against a real PostgreSQL container.
package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coderGtm/delta/auth"
	"github.com/coderGtm/delta/contract"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// isInvalidToken reports whether err is an INVALID_TOKEN API error.
func isInvalidToken(err error) bool {
	var apiErr *httpapi.APIError
	return errors.As(err, &apiErr) && apiErr.Code == "INVALID_TOKEN"
}

// TestRefreshTokenServiceLifecycle exercises the create, rotate, revoke,
// revoke-all, and cleanup paths of RefreshTokenService against the database.
func TestRefreshTokenServiceLifecycle(t *testing.T) {
	store := contract.Setup(t)
	ctx := context.Background()
	user, err := store.Querier().CreateUser(ctx, db.CreateUserParams{
		AuthUid: pgtype.Text{String: "lifecycle-uid", Valid: true},
		Name:    "Lifecycle User",
		Email:   pgtype.Text{String: "lifecycle@example.com", Valid: true},
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	userID := uuid.UUID(user.ID.Bytes)

	svc := auth.NewRefreshTokenService(store, time.Hour, time.Minute)

	raw, err := svc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if raw == "" {
		t.Fatal("create returned an empty token")
	}

	newRaw, err := svc.Rotate(ctx, raw)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if newRaw == raw || newRaw == "" {
		t.Fatalf("rotate returned %q, want a fresh token", newRaw)
	}
	if _, err := svc.Rotate(ctx, raw); !isInvalidToken(err) {
		t.Errorf("rotate of a rotated token err = %v, want INVALID_TOKEN", err)
	}

	raw2, err := svc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}
	if err := svc.Revoke(ctx, raw2); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := svc.Rotate(ctx, raw2); !isInvalidToken(err) {
		t.Errorf("rotate of a revoked token err = %v, want INVALID_TOKEN", err)
	}

	raw3, err := svc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("create third token: %v", err)
	}
	raw4, err := svc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("create fourth token: %v", err)
	}
	if err := svc.RevokeAllForUser(ctx, userID); err != nil {
		t.Fatalf("revoke all: %v", err)
	}
	for _, tok := range []string{raw3, raw4} {
		if _, err := svc.Rotate(ctx, tok); !isInvalidToken(err) {
			t.Errorf("rotate after RevokeAllForUser err = %v, want INVALID_TOKEN", err)
		}
	}

	short := auth.NewRefreshTokenService(store, time.Millisecond, time.Millisecond)
	raw5, err := short.Create(ctx, userID)
	if err != nil {
		t.Fatalf("create short-lived token: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if _, err := short.Rotate(ctx, raw5); !isInvalidToken(err) {
		t.Errorf("rotate of an expired token err = %v, want INVALID_TOKEN", err)
	}
	expired, _, err := short.Cleanup(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if expired < 1 {
		t.Errorf("cleanup removed %d expired tokens, want at least 1", expired)
	}
}
