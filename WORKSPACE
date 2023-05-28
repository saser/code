workspace(name = "code")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

RULES_GO_TAG = "v0.39.1"

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "6dc2da7ab4cf5d7bfc7c949776b1b7c733f05e56edc4bcd9022bb249d2e2a996",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/{tag}/rules_go-{tag}.zip".format(tag = RULES_GO_TAG),
        "https://github.com/bazelbuild/rules_go/releases/download/{tag}/rules_go-{tag}.zip".format(tag = RULES_GO_TAG),
    ],
)

GAZELLE_TAG = "v0.30.0"

http_archive(
    name = "bazel_gazelle",
    sha256 = "727f3e4edd96ea20c29e8c2ca9e8d2af724d8c7778e7923a854b2c80952bc405",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/{tag}/bazel-gazelle-{tag}.tar.gz".format(tag = GAZELLE_TAG),
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/{tag}/bazel-gazelle-{tag}.tar.gz".format(tag = GAZELLE_TAG),
    ],
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies")
load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("//:repositories.bzl", "go_repositories")

# gazelle:repository_macro repositories.bzl%go_repositories
go_repositories()

go_rules_dependencies()

# This version number must be kept in sync with tools.mk.
go_register_toolchains(version = "1.20.2")

gazelle_dependencies()

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "b1e80761a8a8243d03ebca8845e9cc1ba6c82ce7c5179ce2b295cd36f7e394bf",
    urls = ["https://github.com/bazelbuild/rules_docker/releases/download/v0.25.0/rules_docker-v0.25.0.tar.gz"],
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
    name = "nginx_image",
    digest = "sha256:6b06964cdbbc517102ce5e0cef95152f3c6a7ef703e4057cb574539de91f72e6",
    # tag = "1.25",
    registry = "index.docker.io",
    repository = "library/nginx",
)

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

BAZEL_TOOLCHAIN_TAG = "0.8.2"

BAZEL_TOOLCHAIN_SHA = "0fc3a2b0c9c929920f4bed8f2b446a8274cad41f5ee823fd3faa0d7641f20db0"

http_archive(
    name = "com_grail_bazel_toolchain",
    canonical_id = BAZEL_TOOLCHAIN_TAG,
    sha256 = BAZEL_TOOLCHAIN_SHA,
    strip_prefix = "bazel-toolchain-{tag}".format(tag = BAZEL_TOOLCHAIN_TAG),
    url = "https://github.com/grailbio/bazel-toolchain/archive/{tag}.tar.gz".format(tag = BAZEL_TOOLCHAIN_TAG),
)

load("@com_grail_bazel_toolchain//toolchain:deps.bzl", "bazel_toolchain_dependencies")

bazel_toolchain_dependencies()

load("@com_grail_bazel_toolchain//toolchain:rules.bzl", "llvm_toolchain")

llvm_toolchain(
    name = "llvm_toolchain",
    llvm_version = "15.0.6",
)

load("@llvm_toolchain//:toolchains.bzl", "llvm_register_toolchains")

llvm_register_toolchains()

HEDRON_COMPILE_COMMANDS_COMMIT = "3dddf205a1f5cde20faf2444c1757abe0564ff4c"

http_archive(
    name = "hedron_compile_commands",
    sha256 = "3cd0e49f0f4a6d406c1d74b53b7616f5e24f5fd319eafc1bf8eee6e14124d115",
    strip_prefix = "bazel-compile-commands-extractor-{commit}".format(commit = HEDRON_COMPILE_COMMANDS_COMMIT),
    url = "https://github.com/hedronvision/bazel-compile-commands-extractor/archive/{commit}.tar.gz".format(commit = HEDRON_COMPILE_COMMANDS_COMMIT),
)

load("@hedron_compile_commands//:workspace_setup.bzl", "hedron_compile_commands_setup")

hedron_compile_commands_setup()

http_archive(
    name = "rules_cc",
    sha256 = "af6cc82d87db94585bceeda2561cb8a9d55ad435318ccb4ddfee18a43580fb5d",
    strip_prefix = "rules_cc-0.0.4",
    urls = ["https://github.com/bazelbuild/rules_cc/releases/download/0.0.4/rules_cc-0.0.4.tar.gz"],
)

http_archive(
    name = "com_google_googletest",
    sha256 = "28744548b5c6dcd70b69dddba8ebb1c8623ace5dbe4e4457541f704290052957",
    strip_prefix = "googletest-a16bfcfda1ea994c1abec23cca8f530953042dfa",
    urls = ["https://github.com/google/googletest/archive/a16bfcfda1ea994c1abec23cca8f530953042dfa.zip"],
)

http_archive(
    name = "com_github_google_benchmark",
    sha256 = "6430e4092653380d9dc4ccb45a1e2dc9259d581f4866dc0759713126056bc1d7",
    strip_prefix = "benchmark-1.7.1",
    urls = ["https://github.com/google/benchmark/archive/refs/tags/v1.7.1.tar.gz"],
)

http_archive(
    name = "com_google_absl",
    sha256 = "8964b0abac57f94f4ddc3a1dcd8565e5fa5edd1620e326d931c1eabeb9267977",
    strip_prefix = "abseil-cpp-4e5ff1559ca3bd7bb777a1c48106464cb656e041",
    urls = ["https://github.com/abseil/abseil-cpp/archive/4e5ff1559ca3bd7bb777a1c48106464cb656e041.zip"],
)

http_archive(
    name = "bazel_skylib",
    sha256 = "74d544d96f4a5bb630d465ca8bbcfe231e3594e5aae57e1edbf17a6eb3ca2506",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-skylib/releases/download/1.3.0/bazel-skylib-1.3.0.tar.gz",
        "https://github.com/bazelbuild/bazel-skylib/releases/download/1.3.0/bazel-skylib-1.3.0.tar.gz",
    ],
)

load("@bazel_skylib//:workspace.bzl", "bazel_skylib_workspace")

bazel_skylib_workspace()

http_archive(
    name = "com_googlesource_code_re2",
    sha256 = "665b65b6668156db2b46dddd33405cd422bd611352c5052ab3dae6a5fbac5506",
    strip_prefix = "re2-2022-12-01",
    urls = ["https://github.com/google/re2/archive/refs/tags/2022-12-01.tar.gz"],
)
