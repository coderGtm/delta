// Package user exposes endpoints for a signed-in user's own account, such as
// deleting the account.
package user

import (
	"context"

	"github.com/coderGtm/delta/go/db"
)

// AccountDeleter deletes an authenticated user's account, including its
// authentication provider account, refresh tokens, and local row.
type AccountDeleter interface {
	// DeleteAccount soft-deletes user and cleans up the associated
	// authentication provider account and refresh tokens.
	DeleteAccount(ctx context.Context, user *db.User, ip, userAgent string) error
}
