// Package postgrestest contains functions for conveniently running Docker
// containers with Postgres databases. It is intended to be used in integration
// tests.
package postgrestest

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"go.saser.se/docker/dockertest"
	"go.saser.se/postgres"
	"go.saser.se/runfiles"
)

// Open takes a runfiles path to a file containing a schema definition (i.e.,
// "CREATE TABLE" statements and similar), and starts a new Docker container
// running Postgres with the given schema. Open returns a connection pool to the
// database in the container.
func Open(ctx context.Context, tb testing.TB, schemaPath string) *postgres.Pool {
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

	// Connect to the container.
	connString := (&url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   addr,
		Path:   dbName,
	}).String()
	pool, err := postgres.Open(ctx, connString)
	if err != nil {
		tb.Fatalf("Failed to open connection pool: %v", err)
	}
	tb.Cleanup(pool.Close)

	// Now that we have a connection we can run the schema script.
	schemaSQL := string(runfiles.ReadT(tb, schemaPath))
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		tb.Fatalf("Failed to create schema: %v", err)
	}

	// The container is up and has the correct schema, and we are ready to
	// return the connection pool to the caller.
	return pool
}
