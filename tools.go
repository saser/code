//go:build tools

package tools

import (
	_ "github.com/bazelbuild/bazelisk"
	_ "github.com/bazelbuild/buildtools/buildifier"
	_ "mvdan.cc/gofumpt"
)
