package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTGenerateParseRoundTrip(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", time.Minute)
	id := uuid.New()
	tok, err := s.GenerateAccessToken(id)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := s.ParseAccessToken(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != id {
		t.Errorf("subject = %v, want %v", got, id)
	}
}

func TestJWTParseWrongSecret(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", time.Minute)
	tok, _ := s.GenerateAccessToken(uuid.New())
	other := NewJWTService("wrong-secret-wrong-secret-wrong-secret-wrong", time.Minute)
	if _, err := other.ParseAccessToken(tok); err == nil {
		t.Fatal("expected signature error")
	}
}

func TestJWTParseExpired(t *testing.T) {
	s := NewJWTService("test-secret-test-secret-test-secret-test-secret", -time.Minute)
	tok, _ := s.GenerateAccessToken(uuid.New())
	if _, err := s.ParseAccessToken(tok); err == nil {
		t.Fatal("expected expiry error")
	}
}
