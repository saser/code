workspace(name = "code")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "099a9fb96a376ccbbb7d291ed4ecbdfd42f6bc822ab77ae6f1b5cb9e914e94fa",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.35.0/rules_go-v0.35.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.35.0/rules_go-v0.35.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "5982e5463f171da99e3bdaeff8c0f48283a7a5f396ec5282910b9e8a49c0dd7e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:repositories.bzl", "go_repositories")

# gazelle:repository_macro repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

# This version number must be kept in sync with tools.mk.
go_register_toolchains(version = "1.19.2")

gazelle_dependencies()

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz"],
)

load("@io_bazel_rules_docker//toolchains/docker:toolchain.bzl", docker_toolchain_configure = "toolchain_configure")

docker_toolchain_configure(
    name = "docker_config",
    client_config = "//docker/config:config.json",
)

load("@io_bazel_rules_docker//repositories:repositories.bzl", container_repositories = "repositories")

container_repositories()

load("@io_bazel_rules_docker//repositories:deps.bzl", container_deps = "deps")

container_deps()

load(
    "@io_bazel_rules_docker//go:image.bzl",
    _go_image_repos = "repositories",
)

_go_image_repos()

load("@io_bazel_rules_docker//container:container.bzl", "container_pull")

container_pull(
    name = "postgres_image",
    digest = "sha256:3691c00fc177519261bc07b06d0aa990bb17e1bfc31dd79662c9dbd432d2d48b",  # tag "14.1" as of 2022-01-01
    # tag = "14.1",
    registry = "index.docker.io",
    repository = "library/postgres",
)

container_pull(
    name = "hello_world_image",
    # tag = "linux",
    digest = "sha256:f54a58bc1aac5ea1a25d796ae155dc228b3f0e11d046ae276b39c4bf2f13d8c4",  # tag "linux" as of 2022-01-08
    registry = "index.docker.io",
    repository = "library/hello-world",
)
