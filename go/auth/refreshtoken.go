package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// RefreshTokenService manages the full lifecycle of refresh tokens: creating
// them from random raw values, validating them, rotating them on use, revoking
// them, and periodically cleaning up expired or stale revoked rows. Only the
// SHA-256 hash of a token is persisted; the raw token is returned to the
// caller exactly once.
type RefreshTokenService struct {
	store     *db.Store
	ttl       time.Duration
	retention time.Duration
}

// NewRefreshTokenService returns a service that persists refresh tokens
// through the given store, grants each token a lifetime of ttl, and keeps
// revoked tokens around for retention after their updated_at before cleanup
// removes them.
func NewRefreshTokenService(store *db.Store, ttl, retention time.Duration) *RefreshTokenService {
	return &RefreshTokenService{store: store, ttl: ttl, retention: retention}
}

// hashToken returns the lowercase SHA-256 hex digest of raw, the form in
// which refresh tokens are stored and looked up.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// generateRandomToken returns a new URL-safe base64 token built from 32
// cryptographically random bytes. RawURLEncoding omits '=' padding.
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Create issues a new refresh token for userID, persisting only its hash, and
// returns the raw token that the caller must hand to the client.
func (s *RefreshTokenService) Create(ctx context.Context, userID uuid.UUID) (string, error) {
	raw, err := generateRandomToken()
	if err != nil {
		return "", err
	}
	_, err = s.store.Querier().CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		TokenHash: hashToken(raw),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(s.ttl), Valid: true},
		Revoked:   false,
		UserID:    pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return "", err
	}
	return raw, nil
}

// validate resolves raw to its stored row, rejecting unknown, revoked, and
// expired tokens with the corresponding InvalidToken error.
func (s *RefreshTokenService) validate(ctx context.Context, raw string) (*db.RefreshToken, error) {
	row, err := s.store.Querier().GetRefreshTokenByHash(ctx, hashToken(raw))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpapi.InvalidToken("Invalid refresh token")
	}
	if err != nil {
		return nil, err
	}
	if row.Revoked {
		return nil, httpapi.InvalidToken("Refresh token has been revoked")
	}
	if row.ExpiresAt.Time.Before(time.Now().UTC()) {
		return nil, httpapi.InvalidToken("Refresh token has expired")
	}
	return &row, nil
}

// Revoke marks the token identified by raw as revoked so it can no longer be
// validated or rotated.
func (s *RefreshTokenService) Revoke(ctx context.Context, raw string) error {
	row, err := s.validate(ctx, raw)
	if err != nil {
		return err
	}
	_, err = s.store.Querier().UpdateRefreshTokenRevoked(ctx, db.UpdateRefreshTokenRevokedParams{ID: row.ID, Revoked: true})
	return err
}

// RotateWithUser validates raw, revokes it, and issues a fresh token for the
// same user, returning the new raw token and the user the old token belonged
// to.
func (s *RefreshTokenService) RotateWithUser(ctx context.Context, raw string) (string, uuid.UUID, error) {
	row, err := s.validate(ctx, raw)
	if err != nil {
		return "", uuid.Nil, err
	}
	if _, err := s.store.Querier().UpdateRefreshTokenRevoked(ctx, db.UpdateRefreshTokenRevokedParams{ID: row.ID, Revoked: true}); err != nil {
		return "", uuid.Nil, err
	}
	newRaw, err := s.Create(ctx, uuid.UUID(row.UserID.Bytes))
	if err != nil {
		return "", uuid.Nil, err
	}
	return newRaw, uuid.UUID(row.UserID.Bytes), nil
}

// Rotate validates raw, revokes it, and issues a fresh token for the same
// user, returning the new raw token.
func (s *RefreshTokenService) Rotate(ctx context.Context, raw string) (string, error) {
	newRaw, _, err := s.RotateWithUser(ctx, raw)
	return newRaw, err
}

// RevokeAllForUser revokes every non-revoked refresh token belonging to
// userID, for example when the account is deleted.
func (s *RefreshTokenService) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.store.Querier().RevokeAllRefreshTokensForUser(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	return err
}

// Cleanup deletes refresh tokens whose expiry has passed and tokens that were
// revoked longer than the retention window ago, returning how many rows each
// delete removed.
func (s *RefreshTokenService) Cleanup(ctx context.Context) (expired, revoked int64, err error) {
	now := time.Now().UTC()
	expired, err = s.store.Querier().DeleteExpiredRefreshTokens(ctx, pgtype.Timestamptz{Time: now, Valid: true})
	if err != nil {
		return 0, 0, err
	}
	revoked, err = s.store.Querier().DeleteOldRevokedRefreshTokens(ctx, pgtype.Timestamptz{Time: now.Add(-s.retention), Valid: true})
	if err != nil {
		return 0, 0, err
	}
	return expired, revoked, nil
}

// RunCleanupTicker starts a background goroutine that runs Cleanup every
// interval until ctx is canceled. Each run uses a context detached from the
// caller's cancellation so an in-flight cleanup completes. Errors are logged
// at error level; runs that removed tokens are logged at info level.
func (s *RefreshTokenService) RunCleanupTicker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expired, revoked, err := s.Cleanup(context.WithoutCancel(ctx))
				if err != nil {
					slog.Error("refresh token cleanup", "err", err)
					continue
				}
				if expired > 0 || revoked > 0 {
					slog.Info("refresh token cleanup removed tokens", "expired", expired, "revoked", revoked)
				}
			}
		}
	}()
}
