// Package db provides the PostgreSQL connection pool, embedded migrations, and
// the query store used by the service.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open creates a pgx connection pool for the given PostgreSQL URL. The pool
// connects lazily; use Ping to verify connectivity.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, url)
}
