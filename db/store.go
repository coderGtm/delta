package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a pgx connection pool and exposes the query interface used by
// the application.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by the given connection pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool returns the underlying pgx connection pool.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Querier returns a Querier bound to the pool for running queries outside a
// transaction.
func (s *Store) Querier() Querier { return New(s.pool) }

// Tx runs fn inside a database transaction, rolling back on error and
// committing on success.
func (s *Store) Tx(ctx context.Context, fn func(q Querier) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(New(tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}
