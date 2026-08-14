// Package auth implements authentication primitives: the JWT access-token
// signing and verification service and the refresh-token lifecycle of
// creation, validation, rotation, revocation, and cleanup.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTService signs and verifies HMAC-SHA256 access tokens carrying the
// authenticated user's UUID as the token subject.
type JWTService struct {
	secret []byte
	ttl    time.Duration
}

// NewJWTService returns a JWTService that signs tokens with the given secret
// and assigns them the given time-to-live.
func NewJWTService(secret string, ttl time.Duration) *JWTService {
	return &JWTService{secret: []byte(secret), ttl: ttl}
}

// GenerateAccessToken signs and returns an HS256 access token whose subject
// is the string form of userID and whose issued-at and expiry claims are set
// relative to the current time.
func (s *JWTService) GenerateAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(s.secret)
}

// ParseAccessToken verifies the signature and expiry of token and returns the
// subject UUID it carries. Any parse, signature, or expiry error is returned.
func (s *JWTService) ParseAccessToken(token string) (uuid.UUID, error) {
	parsed, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || !parsed.Valid {
		return uuid.Nil, errors.New("invalid token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}
