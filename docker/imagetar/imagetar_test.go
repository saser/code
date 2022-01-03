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

var testimage = runfiles.MustRead("docker/imagetar/testimage.tar")

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
	want := map[string]map[string]string{
		"bazel/docker/imagetar": {
			"testimage": "a5f34025714d147c8ad37b8e237b52af7b58a5f44be46a5e550f0873705d1f24",
		},
	}
	got, err := Repositories(bytes.NewReader(testimage))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Repositories: unexpected return value (-want +got)\n%s", diff)
	}
}

func TestRepositories_Error(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			_, got := Repositories(tt.r)
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateErrors()); diff != "" {
				t.Errorf("unexpected error from Repositories (-want +got)\n%s", diff)
			}
		})
	}
}
