package basic

import (
	"context"
	"errors"

	"go.saser.se/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Interceptor(username string, password string) (grpc.UnaryServerInterceptor, error) {
	if username == "" {
		return nil, errors.New("basic: interceptor: empty username")
	}
	if password == "" {
		return nil, errors.New("basic: interceptor: empty password")
	}

	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "basic: no metadata in incoming context")
		}
		values := md.Get(auth.MetadataKey)
		if got, want := len(values), 1; got != want {
			return nil, status.Errorf(codes.Unauthenticated, "basic: metadata key %q has %d values; want exactly %d", auth.MetadataKey, got, want)
		}

		creds, err := Parse(values[0])
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "basic: metadata key %q does not have expected format %q", auth.MetadataKey, "Basic base64(username:password)")
		}

		if creds.Username == "" {
			return nil, status.Error(codes.Unauthenticated, "basic: credentials contains empty username")
		}
		if creds.Password == "" {
			return nil, status.Error(codes.Unauthenticated, "basic: credentials contains empty password")
		}
		if creds.Username != username || creds.Password != password {
			return nil, status.Error(codes.Unauthenticated, "basic: credentials contain mismatched username and password")
		}
		return handler(ctx, req)
	}
	return interceptor, nil
}
