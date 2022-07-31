// Package grpctest contains utilities for quickly setting up a gRPC server that
// serves an implementation of a service. The package is intended to make
// writing unit tests using a real gRPC transport easier.
package grpctest

import (
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

// NewServerAddress starts up a gRPC server, registers the given implementation
// for the given service description, and returns the address the gRPC server is
// listening on.
func NewServerAddress(tb testing.TB, sd *grpc.ServiceDesc, impl any) string {
	tb.Helper()

	// We use a port number of 0 to let the operating system assign us a port.
	const listenAddr = "localhost:0"
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		tb.Fatalf("failed to create TCP listener for %q: %v", listenAddr, err)
	}
	tb.Cleanup(func() {
		if err := lis.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			tb.Errorf("failed to close TCP listener for %q: %v", listenAddr, err)
		}
	})

	errc := make(chan error, 1)
	tb.Cleanup(func() {
		if err := <-errc; err != nil {
			tb.Errorf("gRPC server failed to serve: %v", err)
		}
	})

	creds := serverCredentials(tb)
	srv := grpc.NewServer(grpc.Creds(creds))
	srv.RegisterService(sd, impl)
	go func() {
		errc <- srv.Serve(lis)
	}()
	tb.Cleanup(srv.GracefulStop)

	return lis.Addr().String()
}

// NewClientConn starts up a gRPC server, with the given implementation
// registered for the given service description, and returns a gRPC client
// pointed at that server. NewClientConn does _not_ wait until the client has
// fully connected before returning.
func NewClientConn(tb testing.TB, sd *grpc.ServiceDesc, impl any) *grpc.ClientConn {
	tb.Helper()

	addr := NewServerAddress(tb, sd, impl)
	creds := clientCredentials(tb)
	cc, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		tb.Fatalf("failed to open gRPC connection to %q: %v", addr, err)
	}
	tb.Cleanup(func() {
		if err := cc.Close(); err != nil {
			tb.Errorf("failed to close gRPC connection to %q: %v", addr, err)
		}
	})
	return cc
}
