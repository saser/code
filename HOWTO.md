# HOWTO

## Running builds and tests

Requirements:

*   `curl`.
*   `make`.
*   `tar`.
*   `unzip`.

This repository uses [`bazelisk`](https://github.com/bazelbuild/bazelisk) to install and run Bazel. In other words, you don't need to install it yourself. `bazelisk` is invoked via a script called `bazel` in the root of the repository.

To run Bazel for building and testing

```shell
# Assuming the root of the repository is the working directory.
$ ./bazel build //...
$ ./bazel test //...
```

## Updating Go dependencies

This sequence of commands is used to update all direct and transitive Go dependencies, and making sure the changes are reflected in Bazel files.

```shell
$ go get -u -t ./...             # updates direct dependencies
$ go get -u -t -tags=tools ./... # updates direct tool dependencies, see tools.go in the root of the repository
$ go mod tidy                    # clean up go.mod and go.sum
$ make generate                  # regenerate code
$ make fix                       # update repositories.bzl, BUILD files, etc
$ make fix                       # possibly needed due to non-hermetic code formatters
```
