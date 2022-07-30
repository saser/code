package basic

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"go.saser.se/auth"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Credentials contains the username and password supplied in a request using
// HTTP Basic authentication. Credentials is to be used for per-RPC credentials
// in gRPC.
type Credentials struct {
	Username string
	Password string
}

var _ credentials.PerRPCCredentials = Credentials{}

// Parse takes a string of the format "Basic base64(username:password)" and
// parses it into Credentials.
func Parse(s string) (Credentials, error) {
	formatErr := fmt.Errorf("basic: credentials do not match expected format %q", "Basic base64(<username>:<password>)")

	kind, encoded, found := strings.Cut(s, " ")
	if !found {
		return Credentials{}, formatErr
	}
	if kind != "Basic" {
		return Credentials{}, formatErr
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Credentials{}, formatErr
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return Credentials{}, formatErr
	}

	return Credentials{
		Username: username,
		Password: password,
	}, nil
}

func FromIncomingContext(ctx context.Context) (Credentials, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Credentials{}, errors.New("basic: no metadata in incoming context")
	}
	values := md.Get(auth.MetadataKey)
	if got, want := len(values), 1; got != want {
		return Credentials{}, fmt.Errorf("basic: metadata key %q has %d values; want exactly %d", auth.MetadataKey, got, want)
	}

	formatErr := fmt.Errorf("basic: metadata key %q does not have expected format %q", auth.MetadataKey, "Basic base64(<username>:<password>)")

	kind, encoded, found := strings.Cut(values[0], " ")
	if !found {
		return Credentials{}, formatErr
	}
	if kind != "Basic" {
		return Credentials{}, formatErr
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Credentials{}, formatErr
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return Credentials{}, formatErr
	}

	return Credentials{
		Username: username,
		Password: password,
	}, nil
}

// HeaderValue encodes the credentials into the form expected by HTTP headers
// and gRPC metadata, namely:
//
//	Basic base64(username:password)
func (c Credentials) HeaderValue() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(c.Username+":"+c.Password))
}

// GetRequestMetadata returns a map containing the authorization key (see
// auth.MetadataKey) with a value of c.HeaderValue().
func (c Credentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		auth.MetadataKey: c.HeaderValue(),
	}, nil
}

// RequireTransportSecurity always returns yes -- HTTP Basic authentication is
// completely insecure without transport security (like HTTPS).
func (c Credentials) RequireTransportSecurity() bool { return true }
