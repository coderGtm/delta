// Package auth implements authentication primitives: the JWT access-token
// signing and verification service, the refresh-token lifecycle of creation,
// validation, rotation, revocation, and cleanup, a Firebase ID-token wrapper,
// the auth business service, and the HTTP middleware and handlers that expose
// them.
package auth

import (
	"context"
	"errors"
	"os"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

// UserInfo is the subset of a Firebase user profile that is relevant to
// authentication.
type UserInfo struct {
	UID         string
	Name        string
	Email       string
	PhoneNumber string
}

// Firebase verifies Firebase ID tokens and deletes Firebase users.
type Firebase interface {
	// VerifyIDToken returns the profile carried by token.
	VerifyIDToken(ctx context.Context, token string) (*UserInfo, error)
	// DeleteUser removes the Firebase user identified by uid.
	DeleteUser(ctx context.Context, uid string) error
}

// firebaseClient is a Firebase implementation backed by function fields so
// that both the real client and test stubs share one type.
type firebaseClient struct {
	verify func(ctx context.Context, token string) (*UserInfo, error)
	del    func(ctx context.Context, uid string) error
}

// VerifyIDToken delegates to the configured verify function.
func (c *firebaseClient) VerifyIDToken(ctx context.Context, token string) (*UserInfo, error) {
	return c.verify(ctx, token)
}

// DeleteUser delegates to the configured delete function.
func (c *firebaseClient) DeleteUser(ctx context.Context, uid string) error {
	return c.del(ctx, uid)
}

// errFirebaseNotConfigured is returned by the stub client when no Firebase
// service account is available, for example during local development.
var errFirebaseNotConfigured = errors.New("firebase not configured")

// notConfigured returns a Firebase stub whose operations all fail with
// errFirebaseNotConfigured.
func notConfigured() Firebase {
	return &firebaseClient{
		verify: func(ctx context.Context, token string) (*UserInfo, error) {
			return nil, errFirebaseNotConfigured
		},
		del: func(ctx context.Context, uid string) error {
			return errFirebaseNotConfigured
		},
	}
}

// NewFirebaseClient returns a Firebase backed by the service account file at
// serviceAccountPath. When the path is empty, the file does not exist, or
// initialization fails, it returns a non-nil stub whose operations fail with
// errFirebaseNotConfigured, so callers never need to handle a nil client.
func NewFirebaseClient(ctx context.Context, serviceAccountPath string) (Firebase, error) {
	if serviceAccountPath == "" {
		return notConfigured(), errFirebaseNotConfigured
	}
	if _, err := os.Stat(serviceAccountPath); err != nil {
		return notConfigured(), errFirebaseNotConfigured
	}
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(serviceAccountPath))
	if err != nil {
		return notConfigured(), err
	}
	client, err := app.Auth(ctx)
	if err != nil {
		return notConfigured(), err
	}
	return &firebaseClient{
		verify: func(ctx context.Context, token string) (*UserInfo, error) {
			tok, err := client.VerifyIDToken(ctx, token)
			if err != nil {
				return nil, err
			}
			name, _ := tok.Claims["name"].(string)
			email, _ := tok.Claims["email"].(string)
			phone, _ := tok.Claims["phone_number"].(string)
			return &UserInfo{UID: tok.UID, Name: name, Email: email, PhoneNumber: phone}, nil
		},
		del: func(ctx context.Context, uid string) error {
			return client.DeleteUser(ctx, uid)
		},
	}, nil
}

// NewStubFirebase returns a fake Firebase that always returns the given
// profile from VerifyIDToken and succeeds on DeleteUser. It is intended for
// tests.
func NewStubFirebase(info *UserInfo) Firebase {
	return &firebaseClient{
		verify: func(ctx context.Context, token string) (*UserInfo, error) {
			return info, nil
		},
		del: func(ctx context.Context, uid string) error {
			return nil
		},
	}
}
