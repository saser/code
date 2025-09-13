// Package dockertest contains test helpers for running Docker containers in
// tests. For better ergonomics when running specific types of containers, such
// as databases, it's probably a better idea to create a new package that wraps
// the functions in this package.
package dockertest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.saser.se/runfiles"
)

type Pool struct {
	*dockertest.Pool
}

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
				Context: ctx,
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
			err := p.Client.StopContainerWithContext(cont.ID, uint(time.Minute.Seconds()), ctx)
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

// // newClient creates a new Docker SDK client based on environment variables. The
// // client is closed when the test ends.
// func newClient(tb testing.TB) *client.Client {
// 	tb.Helper()
// 	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	tb.Cleanup(func() {
// 		if err := c.Close(); err != nil {
// 			tb.Error(err)
// 		}
// 	})
// 	return c
// }

// // Load takes a runfile path (i.e., one relative to the workspace root) of a
// // .tar archive containing a single Docker image and loads that image into the
// // Docker daemon.
// func Load(ctx context.Context, tb testing.TB, path string) string {
// 	tb.Helper()
// 	f := runfiles.OpenT(tb, path)

// 	// We want to load exactly one image.
// 	imgs, err := imagetar.Images(f)
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	if len(imgs) != 1 {
// 		tb.Fatalf("image archive did not contain exactly one image; got %q", imgs)
// 	}

// 	// Seek back to the beginning of the archive and load it into the Docker
// 	// daemon.
// 	f.Seek(0, 0) // First 0 means offset 0, second 0 means "relative to origin of file".
// 	c := newClient(tb)
// 	res, err := c.ImageLoad(ctx, f, true /*quiet*/)
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	defer res.Body.Close()
// 	return imgs[0]
// }

// // Run creates a new container, starts it, and then returns the container ID.
// // The opts.Image field set to an image that exists in the Docker daemon.
// func Run(ctx context.Context, tb testing.TB, opts RunOptions) string {
// 	tb.Helper()

// 	if opts.Image == "" {
// 		tb.Fatalf("run options does not have Image set")
// 	}

// 	if opts.KeepRunning && !opts.KeepContainer {
// 		tb.Fatal("KeepContainer must be true if KeepRunning is true")
// 	}

// 	c := newClient(tb)

// 	// First, create the container.
// 	contCfg := &container.Config{
// 		Image: opts.Image,
// 	}
// 	for k, v := range opts.Environment {
// 		contCfg.Env = append(contCfg.Env, k+"="+v)
// 	}
// 	hostCfg := &container.HostConfig{
// 		// As a sane default, always publish all ports. This can be revisited
// 		// later if needed.
// 		PublishAllPorts: true,
// 	}
// 	cont, err := c.ContainerCreate(ctx, contCfg, hostCfg, nil, nil, opts.Name)
// 	if err != nil {
// 		tb.Fatal(err)
// 	}
// 	if len(cont.Warnings) > 0 {
// 		for _, w := range cont.Warnings {
// 			tb.Error(w)
// 		}
// 		tb.FailNow()
// 	}
// 	// Unless opts.KeepContainer is true, the container should be removed when
// 	// the test ends.
// 	if !opts.KeepContainer {
// 		tb.Cleanup(func() {
// 			if err := c.ContainerRemove(ctx, cont.ID, types.ContainerRemoveOptions{}); err != nil {
// 				tb.Error(err)
// 			}
// 		})
// 	}

// 	// Second, after the container has been created, start the container.
// 	if err := c.ContainerStart(ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
// 		tb.Fatal(err)
// 	}
// 	// Unless opts.KeepRunning is true, stop the container when the test ends.
// 	if !opts.KeepRunning {
// 		tb.Cleanup(func() {
// 			if err := c.ContainerStop(ctx, cont.ID, nil); err != nil {
// 				tb.Error(err)
// 			}
// 		})
// 	}
// 	return cont.ID
// }

// // Address returns the address (hostname/IP and port) on the host that the given
// // container port is bound to. The port should be given in Docker's
// // "number/protocol" format. For example, if the container image exposes port
// // 5432 over TCP, the host IP is "0.0.0.0", and the port on the host is 1337,
// // port should be given as "5432/tcp" and Address will return "0.0.0.0:1337".
// func Address(ctx context.Context, tb testing.TB, id string, port string) string {
// 	tb.Helper()
// 	c := newClient(tb)

// 	// It seems that in testing this operation sometimes fails if it is executed
// 	// too soon after the container has been created. Therefore, we execute the
// 	// binding lookup with exponential backoff on errors, to increase its
// 	// reliability. It should rarely matter in practice.

// 	var bindings []nat.PortBinding
// 	errLessThanOneBinding := errors.New("less than one binding")
// 	op := backoff.Operation(func() error {
// 		info, err := c.ContainerInspect(ctx, id)
// 		if err != nil {
// 			return err
// 		}
// 		if info.NetworkSettings == nil {
// 			return errors.New("networksettings is nil")
// 		}
// 		var ok bool
// 		bindings, ok = info.NetworkSettings.Ports[nat.Port(port)]
// 		if !ok {
// 			return errors.New("no port bindings at all")
// 		}
// 		if len(bindings) < 1 {
// 			return &backoff.PermanentError{Err: errLessThanOneBinding}
// 		}
// 		return nil
// 	})
// 	if err := backoff.Retry(op, backoff.WithContext(backoff.NewExponentialBackOff(), ctx)); err != nil {
// 		if errors.Is(err, errLessThanOneBinding) {
// 			tb.Fatalf("container %v has less than one port binding for %q; got %v", id, port, bindings)
// 		}
// 		tb.Fatalf("port %q is not exposed by container %v", port, id)
// 	}

// 	b := bindings[0]
// 	return net.JoinHostPort(b.HostIP, b.HostPort)
// }
