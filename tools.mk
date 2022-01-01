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

# gofumpt: a stricter subset of gofmt.
gofumpt := $(tools)/gofumpt
$(gofumpt): go.mod
	go \
		build \
		-o='$@' \
		mvdan.cc/gofumpt

# protoc: the protobuf compiler.
protoc_version := 3.19.1
protoc_archive := $(tools)/protoc_$(protoc_version).zip
$(protoc_archive):
	curl \
		--fail \
		--location \
		--show-error \
		--silent \
		--output '$@' \
		'https://github.com/protocolbuffers/protobuf/releases/download/v$(protoc_version)/protoc-$(protoc_version)-linux-x86_64.zip'
protoc_dir := $(tools)/protoc_$(protoc_version)
$(protoc_dir): $(protoc_archive)
	unzip \
		'$(protoc_archive)' \
		-d '$@'
protoc := $(protoc_dir)/bin/protoc
$(protoc): $(protoc_dir)
