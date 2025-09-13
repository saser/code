package dockertest

import (
	"testing"
)

const helloWorld = "docker/dockertest/hello_world/tarball.tar"

func TestPool_Load(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := NewPool(t, "")
	got := p.Load(ctx, t, helloWorld)
	want := "saser.se/docker/dockertest/hello_world:latest"
	if got != want {
		t.Errorf("Load(%q) = %q; want %q", helloWorld, got, want)
	}
}

func TestPool_Run(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	p := NewPool(t, "")
	opts := RunOptions{
		Image: p.Load(ctx, t, helloWorld),
	}
	id := p.Run(ctx, t, opts)
	// We can't assert much about the ID as it's assigned by the Docker daemon.
	// It shouldn't be empty, however.
	if id == "" {
		t.Errorf("Run(%+v) returned an empty string", opts)
	}
}
