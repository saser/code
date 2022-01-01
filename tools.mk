# tools.mk: rules for installing tools used by this project.

root := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

# The underscore is to prevent the `go` command from considering any Go files that may exist in
# downloaded third-party dependencies.
tools := $(root)/_tools
$(tools):
	mkdir -p '$@'

# bazelisk: a script to run Bazel with a given version.
bazelisk := $(tools)/bazelisk
$(bazelisk): go.mod $(tools)
	go \
		build \
		-o='$@' \
		github.com/bazelbuild/bazelisk

# buildifier: a formatter and linter for BUILD.bazel files.
buildifier := $(tools)/buildifier
$(buildifier): go.mod
	go \
		build \
		-o='$@' \
		github.com/bazelbuild/buildtools/buildifier
