package gcp

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.saser.se/runfiles"
	"k8s.io/klog/v2"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

// constantDeclaration represents a "CONSTANT = <value>" declaration in
// Starlark.
type constantDeclaration struct {
	Line  int
	Name  string
	Value string
}

// parseConstants is a crude Starlark parser that only accepts constants of the form:
//
//	CONSTANT = <value>
//
// It returns a map from constant name to its corresponding declaration, or an
// error if it cannot parse the given Starlark code.
func parseConstants(bzl string) (map[string]constantDeclaration, error) {
	decls := make(map[string]constantDeclaration)
	for i, line := range strings.Split(bzl, "\n") {
		lineNumber := i + 1
		// We don't care about leading or trailing spaces.
		line = strings.TrimSpace(line)
		// If the line is empty, skip it.
		if line == "" {
			continue
		}
		// If the first character (after trimming spaces) is a '#' or a '"', we
		// assume it's either a comment or a docstring so we skip it.
		if c := line[0]; c == '#' || c == '"' {
			klog.V(1).Infof("Assuming line %d is a comment: %q", lineNumber, line)
			continue
		}
		// We assume the line looks like this:
		// CONSTANT = <value>
		// with any number of spaces surrounding the '=' character.
		name, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d doesn't follow format %q: %q", lineNumber, "CONSTANT = <value>", line)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if d, exists := decls[name]; exists {
			return nil, fmt.Errorf("line %d defines %q, already defined as %q = %q on line %d", lineNumber, name, d.Name, d.Value, d.Line)
		}
		decls[name] = constantDeclaration{
			Line:  lineNumber,
			Name:  name,
			Value: value,
		}
	}
	return decls, nil
}

// TestConstants attempts to verify that the contents of the constants.bzl file
// (not submitted to source control) matches what is in the template.
func TestConstants(t *testing.T) {
	t.Parallel()
	templateConstants, err := parseConstants(string(runfiles.ReadT(t, "gcp/constants.bzl.template")))
	if err != nil {
		t.Fatalf("couldn't parse constants.bzl.template: %v", err)
	}
	realConstants, err := parseConstants(string(runfiles.ReadT(t, "gcp/constants.bzl")))
	if err != nil {
		t.Fatalf("couldn't parse constants.bzl: %v", err)
	}

	// Verify that all constants in the template are defined, and that no other
	// constants are defined.
	template := mapKeys(templateConstants)
	real := mapKeys(realConstants)
	less := func(s1, s2 string) bool { return s1 < s2 }
	if diff := cmp.Diff(template, real, cmpopts.SortSlices(less)); diff != "" {
		t.Errorf("Unexpected diff between defined constant names (-template +real)\n%s", diff)
	}
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	var keys []K
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
