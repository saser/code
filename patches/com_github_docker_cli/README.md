# Patches for https://github.com/docker/cli

## `01-fix-broken-embeds.patch`

Background:

The source file `cli/compose/schema/schema.go` has a `//go:embed` directive that embeds the files `cli/compose/schema/data/*.json`.

There is also a file `cli/compose/schema/data/doc.go` that essentially makes a Go package out of the `data` directory, even though that package doesn't contain any real Go code. The purpose seems to be to keep the `data/*.json` files when `github.com/docker/cli` is a vendored dependency: https://github.com/docker/cli/pull/1169.

Gazelle does support generating `embedsrcs` attributes for `//go:embed` directives. However, for complicated reasons that I don't understand, it seems that Gazelle won't generate `embedsrcs` if the files being embedded are also part of a Bazel package. See these issues:
- https://github.com/bazel-contrib/bazel-gazelle/issues/1392
- https://github.com/bazel-contrib/bazel-gazelle/issues/1316

This means that this directive:
```go
//go:embed data/config_schema_v*.json
var schemas embed.FS
```
will silently fail to generate an `embedsrcs` attribute. Anything that later transitively depends on the generated `go_library` target for the `cli/compose/schema` Go package will fail to build, with an error saying that no such files `data/config_schema_v*.json` exist.

This patch works around this by:
- Adding the right `embedsrcs` attribute
- Removing the `BUILD.bazel` and `doc.go` files in the `data/` directory.

The patch must be applied _after_ Gazelle has already generated `BUILD.bazel` files, and the way to achieve this in the Bzlmod world with `go_deps.from_file("go.mod")` seems to be to use `go_deps.module_override`:
```starlark
go_deps.module_override(
    path = "github.com/docker/cli",
    patches = ["//patches/com_github_docker_cli:01-fix-broken-embeds.patch"],
    patch_strip = 1,
)
```
