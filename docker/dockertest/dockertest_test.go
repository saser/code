package dockertest

import (
	"net/http"
	"testing"

	"github.com/cenkalti/backoff/v4"
	"go.saser.se/runfiles"
)

var (
	helloWorld = runfiles.MustPath("docker/dockertest/hello_world/tarball.tar")
	nginx      = runfiles.MustPath("docker/dockertest/nginx/tarball.tar")
)

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

func TestPool_Address(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	p := NewPool(t, "")
	opts := RunOptions{
		Image: p.Load(ctx, t, nginx),
	}
	id := p.Run(ctx, t, opts)
	addr := p.Address(ctx, t, id, "80/tcp")

	// It's not uncommon that Address returns before the container has actually
	// opened up a listener on the given port. Therefore, if we don't do this
	// with a retry, this test likely fails.
	url := "http://" + addr + "/"
	var (
		res *http.Response
		err error
	)
	op := func() error {
		res, err = http.Get(url)
		return err
	}
	if err := backoff.Retry(op, backoff.WithContext(backoff.NewExponentialBackOff(), ctx)); err != nil {
		t.Fatalf("http.Get(%q) err = %v; want nil", url, err)
	}
	defer res.Body.Close()
	// The HTTP server promises to serve something on the "/" path. We only care
	// that we can make a request to it, and it returns 200 OK. We don't care
	// about the actual response.
	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("http.Get(%q) code = %v; want %v", url, got, want)
	}
}
