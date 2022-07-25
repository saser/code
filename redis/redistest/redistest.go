package redistest

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v9"
	"go.saser.se/docker/dockertest"
)

func Open(ctx context.Context, tb testing.TB) *redis.Client {
	tb.Helper()
	dockerOpts := dockertest.RunOptions{
		Image: dockertest.Load(ctx, tb, "redis/image.tar"),
	}
	id := dockertest.Run(ctx, tb, dockerOpts)
	addr := dockertest.Address(ctx, tb, id, "6379/tcp")

	redisOpts := &redis.Options{
		Addr: addr,
	}
	client := redis.NewClient(redisOpts)
	if err := client.Ping(ctx).Err(); err != nil {
		tb.Fatalf("pinging Redis server at %q: %v", addr, err)
	}
	return client
}
