package dockertest

import "testing"

func TestPool_Load(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	p := NewPool(t, "")
	p.Load(ctx, t, "docker/dockertest/hello_world/tarball.tar")
}
