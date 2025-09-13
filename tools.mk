# tools.mk: rules for installing tools used by this project.

root := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))

# The underscore is to prevent the `go` command from considering any Go files that may exist in
# downloaded third-party dependencies.
tools := $(root)/_tools
$(tools):
	mkdir -p '$@'

# protoc: the protobuf compiler.
protoc_version := 32.0
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
	bazel run @rules_go//go -- \
		build \
		-o='$@' \
		google.golang.org/protobuf/cmd/protoc-gen-go

# protoc-gen-go-grpc: the protoc plugin for generating gRPC-Go code from protobufs.
protoc-gen-go-grpc := $(tools)/protoc-gen-go-grpc
$(protoc-gen-go-grpc): go.mod $(go) | $(tools)
	bazel run @rules_go//go -- \
		build \
		-o='$@' \
		google.golang.org/grpc/cmd/protoc-gen-go-grpc
