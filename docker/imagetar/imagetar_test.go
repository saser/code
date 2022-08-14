package imagetar

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.saser.se/runfiles"
)

var (
	testimage  = runfiles.MustRead("docker/imagetar/testimage_hello_world.tar")
	testbundle = runfiles.MustRead("docker/imagetar/testbundle.tar")
)

// replaceFile reads the given tar archive, replaces the named file with the
// given contents, and returns the resulting archive as a byte slice. As a
// special case, if contents is nil the named file is not written to the new
// archive, effectively deleting it.
func replaceFile(t *testing.T, archive []byte, name string, contents []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		var body []byte
		if hdr.Name == name {
			if contents == nil {
				continue
			}
			hdr.Size = int64(len(contents))
			body = contents
		} else {
			var err error
			body, err = io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}

func TestRepositories(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		r    io.Reader
		want map[string]map[string]string
	}{
		{
			name: "SingleImage",
			r:    bytes.NewReader(testimage),
			want: map[string]map[string]string{
				"bazel/docker/imagetar": {
					"testimage_hello_world": "a5f34025714d147c8ad37b8e237b52af7b58a5f44be46a5e550f0873705d1f24",
				},
			},
		},
		{
			name: "Bundle",
			r:    bytes.NewReader(testbundle),
			want: map[string]map[string]string{
				"bazel/docker/imagetar": {
					"testimage_hello_world": "a5f34025714d147c8ad37b8e237b52af7b58a5f44be46a5e550f0873705d1f24",
					"testimage_hola_mundo":  "a5f34025714d147c8ad37b8e237b52af7b58a5f44be46a5e550f0873705d1f24",
				},
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Repositories(tt.r)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Repositories: unexpected return value (-want +got)\n%s", diff)
			}
		})
	}
}

func TestRepositories_Error(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		r    io.Reader
		want error
	}{
		{
			name: "NotFound",
			r:    bytes.NewReader(replaceFile(t, testimage, "repositories", nil)),
			want: ErrRepositoriesNotFound,
		},
		{
			name: "Invalid",
			r:    bytes.NewReader(replaceFile(t, testimage, "repositories", []byte("this is not JSON"))),
			want: ErrRepositoriesInvalid,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, got := Repositories(tt.r)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("unexpected error from Repositories (-want +got)\n%s", diff)
			}
		})
	}
}

func TestImages(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		r    io.Reader
		want []string
	}{
		{
			name: "SingleImage",
			r:    bytes.NewReader(testimage),
			want: []string{
				"bazel/docker/imagetar:testimage_hello_world",
			},
		},
		{
			name: "Bundle",
			r:    bytes.NewReader(testbundle),
			want: []string{
				"bazel/docker/imagetar:testimage_hello_world",
				"bazel/docker/imagetar:testimage_hola_mundo",
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := Images(tt.r)
			if err != nil {
				t.Fatal(err)
			}
			lessFunc := func(s1, s2 string) bool { return s1 < s2 }
			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(lessFunc)); diff != "" {
				t.Errorf("unexpected result from Images (-want +got)\n%s", diff)
			}
		})
	}
}

func TestImages_Error(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		r    io.Reader
		want error
	}{
		{
			name: "NotFound",
			r:    bytes.NewReader(replaceFile(t, testimage, "repositories", nil)),
			want: ErrRepositoriesNotFound,
		},
		{
			name: "Invalid",
			r:    bytes.NewReader(replaceFile(t, testimage, "repositories", []byte("this is not JSON"))),
			want: ErrRepositoriesInvalid,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, got := Images(tt.r)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("unexpected error from Images (-want +got)\n%s", diff)
			}
		})
	}
}
