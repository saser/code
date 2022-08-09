// Package grpctest contains utilities for quickly setting up a gRPC server that
// serves an implementation of a service. The package is intended to make
// writing unit tests using a real gRPC transport easier.
package grpctest

import (
	"context"
	"errors"
	"net"
	"testing"

	"go.saser.se/runfiles"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	certFile = runfiles.MustPath("grpctest/test_cert.pem")
	keyFile  = runfiles.MustPath("grpctest/test_key.pem")
)

// serverCredentials creates server credentials from the pre-initialized
// certFile and keyFile paths above.
func serverCredentials(tb testing.TB) credentials.TransportCredentials {
	tb.Helper()
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		tb.Fatalf("failed to create server credentials from testonly TLS certificate and key: %v", err)
	}
	return creds
}

// clientCredentials creates client credentials from the pre-initialized
// certFile path above.
func clientCredentials(tb testing.TB) credentials.TransportCredentials {
	tb.Helper()
	creds, err := credentials.NewClientTLSFromFile(certFile, "")
	if err != nil {
		tb.Fatalf("failed to create client credentials from testonly TLS certificate: %v", err)
	}
	return creds
}

// Server represents a gRPC server that has been set up by this package.
type Server struct {
	// Address is what the server is listening on.
	Address string
	// ClientConn is pre-dialed and ready for use.
	ClientConn *grpc.ClientConn
}

// Options configures how the server and client connection should be set up.
//
// ServiceDesc and Implementation are required fields. All other fields are
// optional.
type Options struct {
	// ServiceDesc is the gRPC service that should be served by the server.
	ServiceDesc *grpc.ServiceDesc
	// Implementation must implement ServiceDesc.
	Implementation any
}

// New sets up a Server and arranges for all associated resources to be cleaned up when the test ends.
//
// The passed in context is only used until New returns. This is enforced by
// deriving a new context that is cancelled when New returns.
func New(ctx context.Context, tb testing.TB, opts Options) *Server {
	tb.Helper()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if opts.ServiceDesc == nil {
		tb.Error("servertest: the service description must not be nil")
	}
	if opts.Implementation == nil {
		tb.Error("servertest: the implementation must not be nil")
	}
	if tb.Failed() {
		tb.FailNow()
	}

	// Only listen on localhost. Using 0 as the port number will make the
	// operating system allocate a port for us.
	const listenAddr = "localhost:0"
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		tb.Fatalf("failed to create listener on %q: %v", listenAddr, err)
	}
	tb.Cleanup(func() {
		// The listener will be used for the gRPC server we will start up later.
		// When that server is stopped it will also close the listener, which
		// results in a net.ErrClosed error. Therefore, we only fail the test if
		// we get some other error.
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			tb.Errorf("failed to close listener on %q: %v", listenAddr, err)
		}
	})

	srvOpts := []grpc.ServerOption{
		grpc.Creds(serverCredentials(tb)),
	}
	srv := grpc.NewServer(srvOpts...)
	srv.RegisterService(opts.ServiceDesc, opts.Implementation)

	errc := make(chan error, 1)
	tb.Cleanup(func() {
		if err := <-errc; err != nil {
			tb.Errorf("gRPC server returned an error: %v", err)
		}
	})

	go func() {
		errc <- srv.Serve(lis)
	}()
	tb.Cleanup(srv.GracefulStop)

	addr := lis.Addr().String()
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(clientCredentials(tb)),
	}
	cc, err := grpc.DialContext(ctx, addr, dialOpts...)
	if err != nil {
		tb.Fatalf("failed to open gRPC connection to %q: %v", addr, err)
	}
	tb.Cleanup(func() {
		if err := cc.Close(); err != nil {
			tb.Errorf("failed to close gRPC connection to %q: %v", addr, err)
		}
	})

	return &Server{
		Address:    addr,
		ClientConn: cc,
	}
}
