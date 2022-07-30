package basic

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.saser.se/auth"
)

func TestParse(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		s    string
		want Credentials
	}{
		{
			name: "UsernameAndPassword",
			s:    "Basic YWxpY2U6c29tZXBhc3N3b3Jk",
			want: Credentials{
				Username: "alice",
				Password: "somepassword",
			},
		},
		{
			name: "EmptyUsername",
			s:    "Basic OnNvbWVwYXNzd29yZA==",
			want: Credentials{
				Username: "",
				Password: "somepassword",
			},
		},
		{
			name: "EmptyPassword",
			s:    "Basic YWxpY2U6",
			want: Credentials{
				Username: "alice",
				Password: "",
			},
		},
		{
			name: "EmptyUsernameAndPassword",
			s:    "Basic Og==",
			want: Credentials{
				Username: "",
				Password: "",
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.s)
			if err != nil {
				t.Fatalf("Parse(%q) err = %v; want nil", tt.s, err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Unexpected result of Parse (-want +got)\n%s", diff)
			}
		})
	}
}

func TestParse_Error(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		s    string
	}{
		{
			name: "WrongKind",
			s:    "Bearer YWxpY2U6c29tZXBhc3N3b3Jk",
		},
		{
			name: "NoKind",
			s:    "OnNvbWVwYXNzd29yZA==",
		},
		{
			name: "NotEncoded",
			s:    "Basic alice:password",
		},
		{
			name: "Empty",
			s:    "",
		},
		{
			name: "NoColon",
			s:    "Basic YWxpY2VwYXNzd29yZA==", // base64("alicepassword")
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Parse(tt.s); err == nil {
				t.Fatalf("Parse(%q) err = nil; want non-nil", tt.s)
			}
		})
	}
}

func TestParse_Roundtrip(t *testing.T) {
	t.Parallel()
	for _, c := range []Credentials{
		{Username: "alice", Password: "super secret password"},
		{Username: "", Password: "super secret password"},
		{Username: "alice", Password: ""},
		{Username: "", Password: ""},
	} {
		s := c.HeaderValue()
		got, err := Parse(s)
		if err != nil {
			t.Errorf("Parse(%q) err = %v; want nil", s, err)
		}
		if diff := cmp.Diff(c, got); diff != "" {
			t.Errorf("Unexpected diff from roundtripping (-want +got)\n%s", diff)
		}
	}
}

func TestCredentials_HeaderValue(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		c    Credentials
		want string
	}{
		{
			name: "UsernameAndPassword",
			c: Credentials{
				Username: "alice",
				Password: "somepassword",
			},
			want: "Basic YWxpY2U6c29tZXBhc3N3b3Jk",
		},
		{
			name: "EmptyUsername",
			c: Credentials{
				Username: "",
				Password: "somepassword",
			},
			want: "Basic OnNvbWVwYXNzd29yZA==",
		},
		{
			name: "EmptyPassword",
			c: Credentials{
				Username: "alice",
				Password: "",
			},
			want: "Basic YWxpY2U6",
		},
		{
			name: "EmptyUsernameAndPassword",
			c: Credentials{
				Username: "",
				Password: "",
			},
			want: "Basic Og==",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, want := tt.c.HeaderValue(), tt.want; got != want {
				t.Errorf("(%#v).String() = %q; want %q", tt.c, got, want)
			}
		})
	}
}

func TestCredentials_GetRequestMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	const (
		username = "alice"
		password = "super secret"
	)

	want := map[string]string{
		auth.MetadataKey: "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password)),
	}

	creds := Credentials{
		Username: username,
		Password: password,
	}
	got, err := creds.GetRequestMetadata(ctx)
	if err != nil {
		t.Fatalf("GetRequestMetadata() err = %v; want nil", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected diff in request metadata (-want +got)\n%s", diff)
	}
}
