include tools.mk

build_files := $(shell git ls-files -- 'WORKSPACE' '**/BUILD.bazel' '*.bzl')
go_files := $(shell git ls-files -- '*.go')

.PHONY: fix
fix: buildifier gofumpt

.PHONY: buildifier
buildifier: \
	$(build_files) \
	$(buildifier)
buildifier:
	$(buildifier) \
		-lint=fix \
		-warnings=all \
		-r \
		-v \
		$(build_files)

.PHONY: gofumpt
gofumpt: \
	$(go_files) \
	$(gofumpt)
gofumpt:
	$(gofumpt) \
		-w \
		$(go_files)
