include tools.mk

# This needs to be kept in sync with go.mod.
#
# We could try to invike 'go list -m' but we also install the `go` command via
# Make, which makes it hairy to make sure `go` is available before this is run.
# Also, the module should change very rarely, so hard-coding the value seems
# worth the tradeoff.
go_module := go.saser.se

build_files := $(shell git ls-files -- 'MODULE.bazel' '**/BUILD.bazel' '*.bzl' | xargs realpath)
cc_files := $(shell git ls-files -- '*.cc' '*.h' | xargs realpath)
go_files := $(shell git ls-files -- '*.go' | xargs grep -L '^// Code generated .* DO NOT EDIT\.$$' | xargs realpath)
proto_files := $(shell git ls-files -- '*.proto' | xargs realpath)

.PHONY: generate
generate: protoc

.PHONY: protoc
protoc: \
	$(protoc) \
	$(protoc-gen-go) \
	$(protoc-gen-go-grpc)
protoc:
	$(protoc) \
		--proto_path='$(root)' \
		--plugin='$(protoc-gen-go)' \
		--go_out=. \
		--go_opt=module='$(go_module)' \
		--plugin='$(protoc-gen-go-grpc)' \
		--go-grpc_out=. \
		--go-grpc_opt=module='$(go_module)' \
		$(proto_files)

.PHONY: fix
fix: \
	fix-bazel-mod-tidy \
	fix-buildifier \
	fix-clang-format \
	fix-gazelle \
	fix-go-mod-tidy \
	fix-gofumpt

.PHONY: fix-clang-format
fix-clang-format:
	bazel run @llvm_toolchain//:clang-format -- \
		-Werror \
		-i \
		$(cc_files) \
		$(proto_files)

.PHONY: fix-buildifier
fix-buildifier:
	bazel run @rules_go//go -- tool buildifier \
		-lint=fix \
		-warnings=all \
		-r \
		-v \
		$(build_files)

.PHONY: fix-gofumpt
fix-gofumpt:
	bazel run @rules_go//go -- tool gofumpt \
		-w \
		$(go_files)

.PHONY: fix-go-mod-tidy
fix-go-mod-tidy:
	bazel run @rules_go//go -- mod tidy -v

.PHONY: fix-bazel-mod-tidy
fix-bazel-mod-tidy:
	bazel mod tidy

.PHONY: fix-gazelle
fix-gazelle:
	bazel run //:gazelle
