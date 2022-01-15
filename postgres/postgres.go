// Package postgres provides interactions with Postgres databases. It does so by
// wrapping the pgxpool package.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.saser.se/postgres/log/glogadapter"
)

const retryInterval = 1 * time.Second

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
	cfg.ConnConfig.Logger = glogadapter.NewLogger()
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
			pool, err := pgxpool.ConnectConfig(ctx, cfg)
			if err != nil {
				continue
			}
			return &Pool{Pool: pool}, nil
		}
	}
}
