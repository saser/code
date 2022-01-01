include tools.mk

build_files := $(shell git ls-files -- 'WORKSPACE' '**/BUILD.bazel' '*.bzl')

.PHONY: fix
fix: buildifier

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
