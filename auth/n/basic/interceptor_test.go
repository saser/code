package basic

import (
	"context"
	"testing"

	"go.saser.se/grpctest"
	"go.saser.se/testing/echo"
	echopb "go.saser.se/testing/echo_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInterceptor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const (
		username = "alice"
		password = "super secret"
	)

	i, err := Interceptor(username, password)
	if err != nil {
		t.Fatalf("Interceptor(%q, %q) err = %v; want nil", username, password, err)
	}
	srv := grpctest.New(ctx, t, grpctest.Options{
		ServiceDesc:    &echopb.Echo_ServiceDesc,
		Implementation: echo.Server{},

		ServerOptions: []grpc.ServerOption{
			grpc.UnaryInterceptor(i),
		},
	})

	for _, tt := range []struct {
		name  string
		creds Credentials
		want  codes.Code
	}{
		{
			name: "OK",
			creds: Credentials{
				Username: username,
				Password: password,
			},
			want: codes.OK,
		},
		{
			name: "EmptyUsername",
			creds: Credentials{
				Username: "",
				Password: password,
			},
			want: codes.Unauthenticated,
		},
		{
			name: "EmptyPassword",
			creds: Credentials{
				Username: username,
				Password: "",
			},
			want: codes.Unauthenticated,
		},
		{
			name: "WrongUsername",
			creds: Credentials{
				Username: "bob",
				Password: password,
			},
			want: codes.Unauthenticated,
		},
		{
			name: "WrongPassword",
			creds: Credentials{
				Username: username,
				Password: "this is wrong",
			},
			want: codes.Unauthenticated,
		},
		{
			name: "WrongUsernameAndPassword",
			creds: Credentials{
				Username: "bob",
				Password: "this is wrong",
			},
			want: codes.Unauthenticated,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := echopb.NewEchoClient(srv.ClientConn)
			req := &echopb.EchoRequest{Message: "This needs authentication"}
			_, err := client.Echo(ctx, req, grpc.PerRPCCredentials(tt.creds))
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("creds = %+v; Echo(%v) err = %v; want code %v", tt.creds, req, err, want)
			}
		})
	}
}
