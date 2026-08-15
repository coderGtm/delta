// Package contract provides the shared test infrastructure for the contract
// and integration test suites: a testcontainers-backed PostgreSQL store and a
// stub Firebase identity provider.
package contract

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coderGtm/delta/go/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Setup starts a disposable PostgreSQL 17 container, applies the schema
// migrations, and returns a store connected to it. The container and the
// connection pool are torn down when the test completes. When no container
// runtime is reachable the calling test is skipped rather than failed so the
// suite degrades gracefully on machines without Docker or Podman.
func Setup(t *testing.T) *db.Store {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image: "postgres:17",
		Env: map[string]string{
			"POSTGRES_DB":       "delta",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "postgres",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithStartupTimeout(60 * time.Second),
	}
	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
		return nil
	}
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	port, err := pg.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("mapped port: %v", err)
	}
	url := fmt.Sprintf("postgres://postgres:postgres@localhost:%s/delta?sslmode=disable", port.Port())

	// The readiness log can precede the postgres listener being fully
	// accepting connections under some container runtimes, so the
	// connection-bound steps retry until they succeed.
	deadline := time.Now().Add(60 * time.Second)
	for {
		err = db.Migrate(ctx, url)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("migrate: %v", err)
		}
		time.Sleep(250 * time.Millisecond)
	}

	pool, err := db.Open(ctx, url)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return db.NewStore(pool)
}
