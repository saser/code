package grpctest

import (
	"context"
	"fmt"
	"testing"

	"go.saser.se/testing/echo"
	echopb "go.saser.se/testing/echo_go_proto"
	"google.golang.org/grpc"
)

// We run these tests with a significant amount of concurrency to try and suss
// out races w.r.t. picking ports and establishing connections.
const concurrency = 100_000

func TestNew(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for i := 0; i < concurrency; i++ {
		t.Run(fmt.Sprintf("%06d", i), func(t *testing.T) {
			t.Parallel()

			srv := New(ctx, t, Options{
				ServiceDesc:    &echopb.Echo_ServiceDesc,
				Implementation: echo.Server{},
			})

			if srv.Address == "" {
				t.Errorf("srv.Addr = %q; want non-empty", srv.Address)
			}
			if srv.ClientConn == nil {
				t.Errorf("srv.Conn = %v; want non-nil", srv.ClientConn)
			}

			// It should be possible to open a new connection to the address.
			_, err := grpc.DialContext(
				ctx,
				srv.Address,
				grpc.WithBlock(), // We need to block to know that it really works.
				grpc.WithTransportCredentials(clientCredentials(t)),
			)
			if err != nil {
				t.Errorf("failed to create a new gRPC connection to %q: %v", srv.Address, err)
			}

			client := echopb.NewEchoClient(srv.ClientConn)
			const msg = "I'm using servertest"
			req := &echopb.EchoRequest{Message: msg}
			res, err := client.Echo(ctx, req)
			if err != nil {
				t.Errorf("Echo(%v) err = %v; want nil", req, err)
			}
			if got, want := res.GetMessage(), msg; got != want {
				t.Errorf("Echo(%v) message = %q; want %q", req, got, want)
			}
		})
	}
}
