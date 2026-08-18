package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestVerifyIDTokenMapsClaims(t *testing.T) {
	fb := NewStubFirebase(&UserInfo{UID: "u1", Name: "N", Email: "e@x", PhoneNumber: "123"})
	info, err := fb.VerifyIDToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
	if info.UID != "u1" || info.Name != "N" || info.Email != "e@x" || info.PhoneNumber != "123" {
		t.Fatalf("info = %+v", info)
	}
}

func TestNewFirebaseClientEmptyPathIsStub(t *testing.T) {
	fb, err := NewFirebaseClient(context.Background(), "")
	if fb == nil {
		t.Fatalf("NewFirebaseClient(\"\") returned a nil client, want a non-nil stub")
	}
	if err == nil || !errors.Is(err, errFirebaseNotConfigured) {
		t.Fatalf("NewFirebaseClient(\"\") err = %v, want errFirebaseNotConfigured", err)
	}
	if _, verr := fb.VerifyIDToken(context.Background(), "tok"); !errors.Is(verr, errFirebaseNotConfigured) {
		t.Fatalf("VerifyIDToken err = %v, want errFirebaseNotConfigured", verr)
	}
	if derr := fb.DeleteUser(context.Background(), "uid"); !errors.Is(derr, errFirebaseNotConfigured) {
		t.Fatalf("DeleteUser err = %v, want errFirebaseNotConfigured", derr)
	}
}

func TestTextOrEmpty(t *testing.T) {
	if got := textOrEmpty(pgtype.Text{String: "abc", Valid: true}); got != "abc" {
		t.Fatalf("valid text = %q, want %q", got, "abc")
	}
	if got := textOrEmpty(pgtype.Text{}); got != "" {
		t.Fatalf("invalid text = %q, want empty", got)
	}
}

func TestPGUUIDRoundTrip(t *testing.T) {
	id := uuid.New()
	if got := toUUID(pgUUID(id)); got != id {
		t.Fatalf("round trip = %v, want %v", got, id)
	}
}
