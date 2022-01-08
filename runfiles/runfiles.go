// Package runfiles contains functions for working with Bazel runfiles.
package runfiles

import (
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
)

// Path returns the full path to the given runfile.
func Path(name string) (string, error) {
	return bazel.Runfile(name)
}

// PathT is like Path but fails the test if the runfile cannot be found.
func PathT(tb testing.TB, name string) string {
	tb.Helper()
	path, err := Path(name)
	if err != nil {
		tb.Fatal(err)
	}
	return path
}

// MustPath is like Path but panics on error.
func MustPath(name string) string {
	path, err := Path(name)
	if err != nil {
		panic(err)
	}
	return path
}

// Open opens the given runfile.
func Open(name string) (*os.File, error) {
	path, err := Path(name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

// OpenT is like Open but fails the test if opening the file fails. OpenT
// arranges for the file to be closed at the end of the test.
func OpenT(tb testing.TB, name string) *os.File {
	tb.Helper()
	f, err := Open(name)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { f.Close() })
	return f
}

// MustOpen is like Open but panics on error.
func MustOpen(name string) *os.File {
	f, err := Open(name)
	if err != nil {
		panic(err)
	}
	return f
}

// Read reads the entire contents of the given runfile.
func Read(name string) ([]byte, error) {
	path, err := Path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// ReadT is like Read but fails the test if reading the file fails.
func ReadT(tb testing.TB, name string) []byte {
	tb.Helper()
	d, err := Read(name)
	if err != nil {
		tb.Fatal(err)
	}
	return d
}

// MustRead is like Read but panics on error.
func MustRead(name string) []byte {
	d, err := Read(name)
	if err != nil {
		panic(err)
	}
	return d
}
