package dockertest

import (
	"context"
	"net"
	"testing"
)

const (
	helloWorld = "docker/dockertest/hello_world_image.tar"
	postgres   = "postgres/image.tar"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if got, want := Load(ctx, t, helloWorld), "bazel/docker/dockertest:hello_world_image"; got != want {
		t.Errorf("Load(%q) = %q; want %q", helloWorld, got, want)
	}
}

func TestRun(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	opts := RunOptions{
		Image: Load(ctx, t, helloWorld),
	}
	id := Run(ctx, t, opts)
	// We can't assert much about the ID as it's assigned by the Docker daemon.
	// It shouldn't be empty, however.
	if id == "" {
		t.Errorf("Run(%+v) returned an empty string", opts)
	}
}

func TestAddress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	opts := RunOptions{
		Image: Load(ctx, t, postgres),
	}
	id := Run(ctx, t, opts)
	addr := Address(ctx, t, id, "5432/tcp")
	// We can't really assert much about the address, but we should be able to
	// dial and connect to it.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("net.Dial(_, %q) err = %v; want nil", addr, err)
	}
	if err := conn.Close(); err != nil {
		t.Error(err)
	}
}
