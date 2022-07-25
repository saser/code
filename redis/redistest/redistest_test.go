package redistest

import (
	"context"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	ctx := context.Background()
	client := Open(ctx, t)
	const (
		key        = "session"
		value      = "abc123"
		expiration = 10 * time.Minute
	)
	if err := client.Set(ctx, key, value, expiration).Err(); err != nil {
		t.Fatalf("SET %q %q (expiration: %v) failed: %v", key, value, expiration, err)
	}
	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("GET %q failed: %v", key, err)
	}
	if got, want := got, value; got != want {
		t.Fatalf("GET %q returned %q; want %q", key, got, want)
	}
}
