// Package postgres provides interactions with Postgres databases. It does so by
// wrapping the pgxpool package.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.saser.se/postgres/log/klogadapter"
)

const retryInterval = 1 * time.Second

// StatementBuilder is ready to use for PostgreSQL queries.
var StatementBuilder = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

// Pool contains a connection pool to a Postgres database.
type Pool struct {
	*pgxpool.Pool
}

// Open connects using the given connection string, retrying until either the
// connection succeeds or the context is cancelled.
func Open(ctx context.Context, connString string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	cfg.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   klogadapter.NewLogger(),
		LogLevel: tracelog.LogLevelTrace,
	}
	pool, err := openConfigWithRetry(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	return pool, err
}

// openConfigWithRetry implements linear backoff to connect with the given
// config until either the connection succeeds or the context is cancelled.
func openConfigWithRetry(ctx context.Context, cfg *pgxpool.Config) (*Pool, error) {
	ticker := time.NewTicker(retryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			if err != nil {
				continue
			}
			// We do a ping here to be more certain that the pool will actually
			// be useful after this method returns. If we don't do this we get
			// flaky tests that start up Postgres containers because the Docker
			// version of Postgres seems to be doing something weird at startup,
			// such as starting up and then restarting. Not sure what happens,
			// but this seems to fix it.
			if err := pool.Ping(ctx); err != nil {
				continue
			}
			return &Pool{Pool: pool}, nil
		}
	}
}
