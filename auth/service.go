package auth

import (
	"context"
	"errors"
	"time"

	"github.com/coderGtm/delta/audit"
	"github.com/coderGtm/delta/db"
	"github.com/coderGtm/delta/httpapi"
	"github.com/coderGtm/delta/metrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service coordinates the authentication flows: login through a Firebase ID
// token, refresh-token rotation, logout, logout-all, and account deletion.
type Service struct {
	Store      *db.Store
	Firebase   Firebase
	JWT        *JWTService
	RefreshSvc *RefreshTokenService
	Audit      *audit.Recorder
	Metrics    *metrics.Registry
}

// NewService returns a Service wired to the given dependencies.
func NewService(store *db.Store, fb Firebase, jwt *JWTService, refresh *RefreshTokenService, a *audit.Recorder, m *metrics.Registry) *Service {
	m.RegisterCounter("auth_login_success_total", nil)
	m.RegisterCounter("auth_refresh_success_total", nil)
	m.RegisterCounter("auth_logout_success_total", nil)
	m.RegisterCounter("auth_logout_all_success_total", nil)
	m.RegisterCounter("user_deleted_total", nil)
	return &Service{Store: store, Firebase: fb, JWT: jwt, RefreshSvc: refresh, Audit: a, Metrics: m}
}

// LoginResponse is the body returned by a successful login.
type LoginResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	AccessToken  string    `json:"accessToken"`
	RefreshToken string    `json:"refreshToken"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// RefreshTokenResponse is the body returned by a successful refresh.
type RefreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// pgUUID wraps id in a pgtype.UUID marked valid.
func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

// toUUID unwraps the bytes of a pgtype.UUID back into a uuid.UUID.
func toUUID(id pgtype.UUID) uuid.UUID { return uuid.UUID(id.Bytes) }

// textOrEmpty returns the value of t, or "" when t is not valid.
func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// CreateUser persists a new user derived from the Firebase profile fields.
func (s *Service) CreateUser(ctx context.Context, uid, name, email, phone string) (*db.User, error) {
	user, err := s.Store.Querier().CreateUser(ctx, db.CreateUserParams{
		AuthUid: pgtype.Text{String: uid, Valid: true},
		Name:    name,
		Email:   pgtype.Text{String: email, Valid: email != ""},
		Phone:   pgtype.Text{String: phone, Valid: phone != ""},
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Login verifies a Firebase ID token and returns an access token and a fresh
// refresh token for the matching user, creating the user row on first sign-in.
func (s *Service) Login(ctx context.Context, firebaseIDToken, ip, userAgent string) (*LoginResponse, error) {
	info, err := s.Firebase.VerifyIDToken(ctx, firebaseIDToken)
	if err != nil {
		return nil, httpapi.InvalidToken("Invalid Firebase ID Token")
	}
	user, err := s.Store.Querier().GetUserByAuthUID(ctx, pgtype.Text{String: info.UID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		var created *db.User
		created, err = s.CreateUser(ctx, info.UID, info.Name, info.Email, info.PhoneNumber)
		if err == nil {
			user = *created
		}
	}
	if err != nil {
		return nil, err
	}
	userID := toUUID(user.ID)
	accessToken, err := s.JWT.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	refreshToken, err := s.RefreshSvc.Create(ctx, userID)
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("auth_login_success_total")
	s.Audit.Record(ctx, user.ID.String(), "AUTH_LOGIN", "USER", userID, map[string]any{"email": textOrEmpty(user.Email)}, ip, userAgent)
	return &LoginResponse{
		ID:           userID,
		Name:         user.Name,
		Email:        textOrEmpty(user.Email),
		Phone:        textOrEmpty(user.Phone),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		CreatedAt:    user.CreatedAt.Time,
		UpdatedAt:    user.UpdatedAt.Time,
	}, nil
}

// Refresh validates refreshToken, revokes it, issues a fresh refresh token for
// the same user, and returns it together with a new access token.
func (s *Service) Refresh(ctx context.Context, refreshToken, ip, userAgent string) (*RefreshTokenResponse, error) {
	newRaw, userID, err := s.RefreshSvc.RotateWithUser(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	accessToken, err := s.JWT.GenerateAccessToken(userID)
	if err != nil {
		return nil, err
	}
	s.Metrics.Increment("auth_refresh_success_total")
	s.Audit.Record(ctx, userID.String(), "AUTH_REFRESH", "USER", userID, nil, ip, userAgent)
	return &RefreshTokenResponse{AccessToken: accessToken, RefreshToken: newRaw}, nil
}

// Logout revokes refreshToken so it can no longer be used.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if err := s.RefreshSvc.Revoke(ctx, refreshToken); err != nil {
		return err
	}
	s.Metrics.Increment("auth_logout_success_total")
	return nil
}

// LogoutAll revokes every refresh token belonging to userID.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, ip, userAgent string) error {
	if err := s.RefreshSvc.RevokeAllForUser(ctx, userID); err != nil {
		return err
	}
	s.Metrics.Increment("auth_logout_all_success_total")
	s.Audit.Record(ctx, userID.String(), "AUTH_LOGOUT_ALL", "USER", userID, nil, ip, userAgent)
	return nil
}

// DeleteAccount soft-deletes user: it removes the Firebase account, revokes
// all of the user's refresh tokens, and marks the local row deleted.
func (s *Service) DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error {
	if user.DeletedAt.Valid {
		return httpapi.Conflict("Account has already been deleted")
	}
	if err := s.Firebase.DeleteUser(ctx, user.AuthUid.String); err != nil {
		return httpapi.Conflict("Failed to delete the user from the authentication provider")
	}
	userID := toUUID(user.ID)
	if err := s.RefreshSvc.RevokeAllForUser(ctx, userID); err != nil {
		return err
	}
	deleted, err := s.Store.Querier().DeleteUserRow(ctx, user.ID)
	if err != nil {
		return err
	}
	s.Metrics.Increment("user_deleted_total")
	s.Audit.Record(ctx, user.ID.String(), "USER_DELETED", "USER", userID, map[string]any{"historicalEmail": textOrEmpty(deleted.HistoricalEmail)}, ip, userAgent)
	return nil
}
