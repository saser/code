package grpctest

import (
	"context"
	"fmt"
	"testing"

	"go.saser.se/testing/echo"
	echopb "go.saser.se/testing/echo_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const concurrency = 10_000

// We run these tests with a significant amount of concurrency to try and suss
// out races w.r.t. picking ports and establishing connections.

func TestNewServerAddress(t *testing.T) {
	t.Parallel()
	for i := 0; i < concurrency; i++ {
		i := i
		t.Run(fmt.Sprintf("%05d", i), func(t *testing.T) {
			t.Parallel()
			addr := NewServerAddress(t, &echopb.Echo_ServiceDesc, echo.Server{})
			// We must connect to the server because otherwise it will be
			// stopped before it has even had a chance to start up, failing the
			// test.
			_, err := grpc.Dial(addr, grpc.WithBlock(), grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				t.Fatalf("failed to connect to %q: %v", addr, err)
			}
		})
	}
}

func TestNewClientConn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for i := 0; i < concurrency; i++ {
		i := i
		t.Run(fmt.Sprintf("%05d", i), func(t *testing.T) {
			t.Parallel()
			cc := NewClientConn(t, &echopb.Echo_ServiceDesc, echo.Server{})
			client := echopb.NewEchoClient(cc)
			req := &echopb.EchoRequest{Message: "Hello, grpctest"}
			if _, err := client.Echo(ctx, req); err != nil {
				t.Fatalf("[% 5d] Echo(%v) err = %v; want nil", i, req, err)
			}
		})
	}
}
