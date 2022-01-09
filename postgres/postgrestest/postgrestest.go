// Package postgrestest contains functions for conveniently running Docker
// containers with Postgres databases. It is intended to be used in integration
// tests.
package postgrestest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.saser.se/docker/dockertest"
	"go.saser.se/runfiles"
)

// Open takes a runfiles path to a file containing a schema definition (i.e.,
// "CREATE TABLE" statements and similar), and starts a new Docker container
// running Postgres with the given schema. Open returns a connection pool to the
// database in the container.
func Open(ctx context.Context, tb testing.TB, schemaPath string) *pgxpool.Pool {
	tb.Helper()
	const (
		user     = "postgrestest"
		password = "some-random-password"
	)
	// Construct the database name by trimming the file extension (if any) from
	// the schema path, and replacing all slashes with underscores. The name
	// doesn't have to be unique or actually mean anything, but constructing it
	// this way may help debuggability at some point in the future.
	dbName := strings.TrimSuffix(schemaPath, filepath.Ext(schemaPath))
	dbName = strings.ReplaceAll(dbName, "/", "_")

	// Start a Postgres container and get the address it's listening on.
	opts := dockertest.RunOptions{
		Image: dockertest.Load(ctx, tb, "postgres/image.tar"),
		Environment: map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       dbName,
		},
	}
	id := dockertest.Run(ctx, tb, opts)
	addr := dockertest.Address(ctx, tb, id, "5432/tcp")

	// Connect, using exponential backoff, to the container.
	connString := fmt.Sprintf("postgres://%s:%s@%s/%s", user, password, addr, dbName)
	pool := connectWithRetry(ctx, tb, connString)

	// Now that we have a connection we can run the schema script.
	schemaSQL := string(runfiles.ReadT(tb, schemaPath))
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		tb.Fatal(err)
	}

	// The container is up and has the correct schema, and we are ready to
	// return the connection pool to the caller.
	return pool
}

// connectWithRetry uses linear backoff to connect using the given connString,
// possibly trying multiple times. Retries are needed because the container
// might not be ready to accept connections very soon after coming up.
func connectWithRetry(ctx context.Context, tb testing.TB, connString string) *pgxpool.Pool {
	tb.Helper()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			tb.Fatal(ctx.Err())
		case <-ticker.C:
			pool, err := pgxpool.Connect(ctx, connString)
			if err != nil {
				continue
			}
			tb.Cleanup(pool.Close)
			return pool
		}
	}
}
