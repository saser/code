include tools.mk

# This needs to be kept in sync with go.mod.
#
# We could try to invike 'go list -m' but we also install the `go` command via
# Make, which makes it hairy to make sure `go` is available before this is run.
# Also, the module should change very rarely, so hard-coding the value seems
# worth the tradeoff.
go_module := go.saser.se

build_files := $(shell git ls-files -- 'WORKSPACE' '**/BUILD.bazel' '*.bzl')
go_files := $(shell git ls-files -- '*.go')
proto_files := $(shell git ls-files -- '*.proto')

.PHONY: generate
generate: protoc

.PHONY: protoc
protoc: \
	$(proto_files) \
	$(protoc) \
	$(protoc-gen-go) \
	$(protoc-gen-go-grpc)
protoc:
	$(protoc) \
		--plugin='$(protoc-gen-go)' \
		--go_out=. \
		--go_opt=module='$(go_module)' \
		--plugin='$(protoc-gen-go-grpc)' \
		--go-grpc_out=. \
		--go-grpc_opt=module='$(go_module)' \
		$(proto_files)

.PHONY: fix
fix: \
	fix-buildifier \
	fix-clang-format \
	fix-go-buildfiles \
	fix-gofumpt

.PHONY: fix-clang-format
fix-clang-format: \
	$(clang-format) \
	$(proto_files)
fix-clang-format:
	$(clang-format) \
		--Werror \
		-i \
		--style=google \
		$(proto_files)

.PHONY: fix-buildifier
fix-buildifier: \
	$(build_files) \
	$(buildifier)
fix-buildifier:
	$(buildifier) \
		-lint=fix \
		-warnings=all \
		-r \
		-v \
		$(build_files)

.PHONY: fix-gofumpt
fix-gofumpt: \
	$(go_files) \
	$(gofumpt)
fix-gofumpt:
	$(gofumpt) \
		-w \
		$(go_files)

.PHONY: fix-go-buildfiles
fix-go-buildfiles: $(go)
	$(go) mod tidy -v
	./bazel run //:gazelle_update_repos
	./bazel run //:gazelle
