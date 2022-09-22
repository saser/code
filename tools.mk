# tools.mk: rules for installing tools used by this project.

root := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

# The underscore is to prevent the `go` command from considering any Go files that may exist in
# downloaded third-party dependencies.
tools := $(root)/_tools
$(tools):
	mkdir -p '$@'

# go: the Go toolchain.
# NOTE: this is only installed to be used to install other tools. The Go
# toolchain used for actually building stuff is pulled down via Bazel and
# specified in the WORKSPACE file.
#
# The version must be kept in sync with the WORKSPACE file.
go_version := 1.19.1
go_archive := $(tools)/go_$(go_version).tar.gz
$(go_archive): | $(tools)
	curl \
		--fail \
		--location \
		--show-error \
		--silent \
		--output '$@' \
		'https://go.dev/dl/go$(go_version).linux-amd64.tar.gz'
go_dir := $(tools)/go_$(go_version)
$(go_dir): $(go_archive)
	mkdir \
		--parents \
		'$@'
	tar \
		--extract \
		--directory '$@' \
		--gzip \
		--strip-components 1 \
		--file '$(go_archive)'
go := $(go_dir)/bin/go
$(go): $(go_dir)

# bazelisk: a script to run Bazel with a given version.
bazelisk := $(tools)/bazelisk
$(bazelisk): go.mod $(go) | $(tools)
	$(go) \
		build \
		-o='$@' \
		github.com/bazelbuild/bazelisk

# buildifier: a formatter and linter for BUILD.bazel files.
buildifier := $(tools)/buildifier
$(buildifier): go.mod $(go) | $(tools)
	$(go) \
		build \
		-o='$@' \
		github.com/bazelbuild/buildtools/buildifier

# gofumpt: a stricter subset of gofmt.
gofumpt := $(tools)/gofumpt
$(gofumpt): go.mod $(go) | $(tools)
	$(go) \
		build \
		-o='$@' \
		mvdan.cc/gofumpt

# protoc: the protobuf compiler.
protoc_version := 21.1
protoc_archive := $(tools)/protoc_$(protoc_version).zip
$(protoc_archive): | $(tools)
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

# protoc-gen-go: the protoc plugin for generating Go code from protobufs.
protoc-gen-go := $(tools)/protoc-gen-go
$(protoc-gen-go): go.mod $(go) | $(tools)
	$(go) \
		build \
		-o='$@' \
		google.golang.org/protobuf/cmd/protoc-gen-go

# protoc-gen-go-grpc: the protoc plugin for generating gRPC-Go code from protobufs.
protoc-gen-go-grpc := $(tools)/protoc-gen-go-grpc
$(protoc-gen-go-grpc): go.mod $(go) | $(tools)
	$(go) \
		build \
		-o='$@' \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc
