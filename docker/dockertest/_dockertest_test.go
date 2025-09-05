package dockertest

import (
	"context"
	"net/http"
	"testing"

	"github.com/cenkalti/backoff/v4"
)

const (
	helloWorld = "docker/dockertest/hello_world_image.tar"
	nginx      = "external/nginx_image/image/image.tar"
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
		Image: Load(ctx, t, nginx),
	}
	id := Run(ctx, t, opts)
	addr := Address(ctx, t, id, "80/tcp")

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
