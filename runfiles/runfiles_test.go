package runfiles

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

const testfile = "runfiles/test.txt"

func TestPath(t *testing.T) {
	path, err := Path(testfile)
	if err != nil {
		t.Fatalf("Path(%q) err = %v; want nil", testfile, err)
	}
	// We can't assert much about the path -- it's too "workstation-dependent".
	// It should _probably_ be an absolute path, though.
	if got, want := filepath.IsAbs(path), true; got != want {
		t.Errorf("filepath.IsAbs(%q) = %v; want %v", path, got, want)
	}
	if testing.Verbose() {
		t.Logf("full path for %q: %q", testfile, path)
	}
}

func TestPathT(t *testing.T) {
	path := PathT(t, testfile)
	// We can't assert much about the path -- it's too "workstation-dependent".
	// It should _probably_ be an absolute path, though.
	if got, want := filepath.IsAbs(path), true; got != want {
		t.Errorf("filepath.IsAbs(%q) = %v; want %v", path, got, want)
	}
	if testing.Verbose() {
		t.Logf("full path for %q: %q", testfile, path)
	}
}

func TestOpen(t *testing.T) {
	f, err := Open(testfile)
	if err != nil {
		t.Fatalf("Open(%q) err = %v; want nil", testfile, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing file failed: %v", err)
		}
	}()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("reading file failed: %v", err)
	}
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestOpenT(t *testing.T) {
	f := OpenT(t, testfile)
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("reading file failed: %v", err)
	}
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestRead(t *testing.T) {
	got, err := Read(testfile)
	if err != nil {
		t.Fatalf("Read(%q) err = %v; want nil", testfile, err)
	}
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestReadT(t *testing.T) {
	got := ReadT(t, testfile)
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}
