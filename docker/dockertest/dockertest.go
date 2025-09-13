// Package dockertest contains test helpers for running Docker containers in
// tests. For better ergonomics when running specific types of containers, such
// as databases, it's probably a better idea to create a new package that wraps
// the functions in this package.
package dockertest

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.saser.se/runfiles"
)

// Pool wraps a [*dockertest.Pool] and provides some convenient test helpers on
// top of it.
type Pool struct {
	*dockertest.Pool
}

// NewPool initializes a new Docker client connection pool. Pass an empty string
// as the endpoint to get reasonable defaults; see the documentation for
// [dockertest.NewPool] for more details.
func NewPool(tb testing.TB, endpoint string) *Pool {
	tb.Helper()
	p, err := dockertest.NewPool(endpoint)
	if err != nil {
		tb.Fatalf("dockertest: create pool with endpoint %q: %v", endpoint, err)
	}
	return &Pool{p}
}

// Load takes a path to a tarball containing a Docker image and loads it into
// the Docker daemon, returning the image name. See the tests for an example of
// how to use this.
func (p *Pool) Load(ctx context.Context, tb testing.TB, path string) string {
	tb.Helper()

	f := runfiles.OpenT(tb, path)
	var out strings.Builder
	err := p.Client.LoadImage(docker.LoadImageOptions{
		Context:      ctx,
		InputStream:  f,
		OutputStream: &out,
	})
	if err != nil {
		tb.Fatalf("dockertest: failed to load image with path %q: %v.\nOutput: %q", path, err, out.String())
	}

	// This is a bit hacky and relies on specific output probably intended
	// for humans ... but it works well enough for now.
	// The Docker daemon responds with a message like:
	//
	//     Loaded image: example.com/my/image:tag
	//
	// (with a trailing newline) and we want to extract
	// "example.com/my/image:tag". To do that, we simply look for a string
	// following the exact format like the above, and return the image name.
	output := strings.TrimSpace(out.String())
	name, ok := strings.CutPrefix(output, "Loaded image: ")
	if !ok {
		tb.Fatalf("dockertest: load: could not extract image name from output from Docker daemon. Full output:\n%s", output)
	}
	return name
}

// RunOptions contains the options for [*Pool.Run].
type RunOptions struct {
	// The image to run a container with. Required.
	Image string
	// Environment variables the container should be started with. Optional.
	Environment map[string]string
	// The name of the container to be created. Optional. If not specified, the
	// Docker daemon will assign a name.
	Name string
	// Whether to keep (i.e., not remove) the container after the test ends.
	// Optional.
	KeepContainer bool
	// Whether to keep the container running after the test ends. Optional. If
	// this is set to true then KeepContainer must also be set to true.
	KeepRunning bool
}

// Run starts a container and keeps it alive for the duration of the test. Opts
// must at least contain the image to run.
func (p *Pool) Run(ctx context.Context, tb testing.TB, opts RunOptions) string {
	tb.Helper()

	// cleanupCtx is used for the common cleanup tasks (stopping/removing
	// containers) but has been separated from the cancel signal carried in
	// ctx (if any). This is intended to support the common use case of
	// using [testing.T.Context] as the top-level context in tests, which is
	// documented to be cancelled just before Cleanup functions are called.
	cleanupCtx := context.WithoutCancel(ctx)

	if opts.Image == "" {
		tb.Fatalf("dockertest: run: image is required.")
	}

	if opts.KeepRunning && !opts.KeepContainer {
		tb.Fatal("dockertest: run: if KeepRunning is true then KeepContainer must also be true.")
	}

	// First, create the container.
	contCfg := &docker.Config{
		Image: opts.Image,
	}
	for k, v := range opts.Environment {
		contCfg.Env = append(contCfg.Env, k+"="+v)
	}
	hostCfg := &docker.HostConfig{
		// As a sane default, always publish all ports. This can be revisited
		// later if needed.
		PublishAllPorts: true,
	}
	cont, err := p.Client.CreateContainer(docker.CreateContainerOptions{
		Context:    ctx,
		Name:       opts.Name,
		Config:     contCfg,
		HostConfig: hostCfg,
	})
	if err != nil {
		tb.Fatalf("dockertest: run: create container: %v", err)
	}
	// Unless opts.KeepContainer is true, the container should be removed when
	// the test ends.
	if !opts.KeepContainer {
		tb.Cleanup(func() {
			if err := p.Client.RemoveContainer(docker.RemoveContainerOptions{
				ID:      cont.ID,
				Context: cleanupCtx,
			}); err != nil {
				tb.Errorf("dockertest: run: remove container after test: %v", err)
			}
		})
	}

	// Second, after the container has been created, start the container.
	if err := p.Client.StartContainerWithContext(cont.ID, nil, ctx); err != nil {
		tb.Fatalf("dockertest: run: start container: %v", err)
	}
	// Unless opts.KeepRunning is true, stop the container when the test ends.
	if !opts.KeepRunning {
		tb.Cleanup(func() {
			err := p.Client.StopContainerWithContext(cont.ID, uint(time.Minute.Seconds()), cleanupCtx)
			if e := new(docker.ContainerNotRunning); errors.As(err, &e) {
				tb.Logf("NOTE: dockertest: run: attempted to stop container %v (running image %v) but it was not running.", cont.ID, opts.Image)
				return
			}
			if err != nil {
				tb.Errorf("dockertest: run: stop container after test: %v", err)
			}
		})
	}
	return cont.ID
}

// Address returns the address (hostname/IP and port) on the host that the given
// container port is bound to. The port should be given in Docker's
// "number/protocol" format. For example, if the container image exposes port
// 5432 over TCP, the host IP is "0.0.0.0", and the port on the host is 1337,
// port should be given as "5432/tcp" and Address will return "0.0.0.0:1337".
func (p *Pool) Address(ctx context.Context, tb testing.TB, id string, port string) string {
	tb.Helper()

	// It seems that in testing this operation sometimes fails if it is
	// executed too soon after the container has been created. Therefore, we
	// execute the binding lookup with exponential backoff on errors, to
	// increase its reliability. It should rarely matter in practice.

	var bindings []docker.PortBinding
	noBindings := errors.New("less than one binding")
	op := backoff.Operation(func() error {
		info, err := p.Client.InspectContainerWithContext(id, ctx)
		if err != nil {
			return err
		}
		if info.NetworkSettings == nil {
			return errors.New("NetworkSettings is nil")
		}
		bindings = info.NetworkSettings.Ports[docker.Port(port)]
		if len(bindings) == 0 {
			return &backoff.PermanentError{Err: noBindings}
		}
		return nil
	})
	if err := backoff.Retry(op, backoff.WithContext(backoff.NewExponentialBackOff(), ctx)); err != nil {
		if errors.Is(err, noBindings) {
			tb.Fatalf("dockertest: address: container %v does not have any port bindings for %q", id, port)
		}
		tb.Fatalf("dockertest: address: port %q is not exposed by container %v", port, id)
	}

	b := bindings[0]
	return net.JoinHostPort(b.HostIP, b.HostPort)
}
