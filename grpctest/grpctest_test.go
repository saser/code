package grpctest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.saser.se/testing/echo"
	echopb "go.saser.se/testing/echo_go_proto"
	"google.golang.org/grpc"
)

// We run these tests with a significant amount of concurrency to try and suss
// out races w.r.t. picking ports and establishing connections.
const concurrency = 10_000

func TestNewServerAddress(t *testing.T) {
	t.Parallel()

	// Bugs in the tested code (which includes the code generating the
	// TLS certificate) generally show up as tests that hang. We set a
	// short timeout to allow tests to fail quickly if that is the case.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	for i := 0; i < concurrency; i++ {
		t.Run(fmt.Sprintf("%05d", i), func(t *testing.T) {
			t.Parallel()
			addr := NewServerAddress(t, &echopb.Echo_ServiceDesc, echo.Server{})

			// The goroutine that runs (*grpc.Server).Serve from
			// NewServerAddress might not even have started before
			// NewServerAddress returns. If the test finished as soon as
			// NewServerAddress returned, then the call to Serve will fail,
			// failing the test. To work around this, we make sure to connect to
			// the server before finishing the test.
			creds := clientCredentials(t)
			_, err := grpc.DialContext(ctx, addr, grpc.WithBlock(), grpc.WithTransportCredentials(creds))
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
		t.Run(fmt.Sprintf("%05d", i), func(t *testing.T) {
			t.Parallel()
			cc := NewClientConn(t, &echopb.Echo_ServiceDesc, echo.Server{})
			client := echopb.NewEchoClient(cc)
			req := &echopb.EchoRequest{Message: "Hello, grpctest"}
			if _, err := client.Echo(ctx, req); err != nil {
				t.Fatalf("Echo(%v) err = %v; want nil", req, err)
			}
		})
	}
}
