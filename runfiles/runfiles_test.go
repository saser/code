package runfiles

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

const (
	testfile     = "runfiles/test.txt"
	doesNotExist = "runfiles/does_not_exist.txt"
)

func TestPath(t *testing.T) {
	t.Parallel()
	path, err := Path(testfile)
	if err != nil {
		t.Fatalf("Path(%q) err = %v; want nil", testfile, err)
	}
	// We can't assert much about the path -- it's too "workstation-dependent".
	// It should _probably_ be an absolute path, though.
	if got, want := filepath.IsAbs(path), true; got != want {
		t.Errorf("filepath.IsAbs(%q) = %v; want %v", path, got, want)
	}
}

func TestPath_Error(t *testing.T) {
	t.Parallel()
	got, err := Path(doesNotExist)
	if err == nil {
		t.Errorf("Path(%q) err = nil; want non-nil", doesNotExist)
	}
	if want := ""; got != want {
		t.Errorf("Path(%q) = %q; want %q", doesNotExist, got, want)
	}
}

func TestPathT(t *testing.T) {
	t.Parallel()
	path := PathT(t, testfile)
	// We can't assert much about the path -- it's too "workstation-dependent".
	// It should _probably_ be an absolute path, though.
	if got, want := filepath.IsAbs(path), true; got != want {
		t.Errorf("filepath.IsAbs(%q) = %v; want %v", path, got, want)
	}
}

func TestMustPath(t *testing.T) {
	t.Parallel()
	path := MustPath(testfile)
	// We can't assert much about the path -- it's too "workstation-dependent".
	// It should _probably_ be an absolute path, though.
	if got, want := filepath.IsAbs(path), true; got != want {
		t.Errorf("filepath.IsAbs(%q) = %v; want %v", path, got, want)
	}
}

func TestMustPath_Panic(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("MustPath(%q) either did not panic or panicked with nil argument", doesNotExist)
		}
	}()
	MustPath(doesNotExist)
}

func TestOpen(t *testing.T) {
	t.Parallel()
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

func TestOpen_Error(t *testing.T) {
	t.Parallel()
	f, err := Open(doesNotExist)
	if err == nil {
		t.Errorf("Open(%q) err = nil; want non-nil", doesNotExist)
	}
	if f != nil {
		t.Errorf("Open(%q) f = %v; want nil", doesNotExist, f)
	}
}

func TestOpenT(t *testing.T) {
	t.Parallel()
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

func TestMustOpen(t *testing.T) {
	t.Parallel()
	f := MustOpen(testfile)
	defer func() {
		if err := f.Close(); err != nil {
			t.Errorf("closing file failed: %v", err)
		}
	}()
}

func TestMustOpen_Panic(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("MustOpen(%q) either did not panic or panicked with nil argument", doesNotExist)
		}
	}()
	MustOpen(doesNotExist)
}

func TestRead(t *testing.T) {
	t.Parallel()
	got, err := Read(testfile)
	if err != nil {
		t.Fatalf("Read(%q) err = %v; want nil", testfile, err)
	}
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestRead_Error(t *testing.T) {
	t.Parallel()
	got, err := Read(doesNotExist)
	if err == nil {
		t.Errorf("Read(%q) err = nil; want non-nil", doesNotExist)
	}
	if got != nil {
		t.Errorf("Read(%q) contents = %v; want nil", doesNotExist, got)
	}
}

func TestReadT(t *testing.T) {
	t.Parallel()
	got := ReadT(t, testfile)
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestMustRead(t *testing.T) {
	t.Parallel()
	got := MustRead(testfile)
	want := []byte("This is an example file to be used in tests.\n")
	if !bytes.Equal(got, want) {
		t.Errorf("unexpected file contents\ngot:  %v\nwant: %v", got, want)
	}
}

func TestMustRead_Panic(t *testing.T) {
	t.Parallel()
	defer func() {
		if err := recover(); err == nil {
			t.Errorf("MustRead(%q) either did not panic or panicked with nil argument", doesNotExist)
		}
	}()
	MustRead(doesNotExist)
}
