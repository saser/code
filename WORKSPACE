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

BAZEL_TOOLCHAIN_TAG = "0.7.2"

BAZEL_TOOLCHAIN_SHA = "f7aa8e59c9d3cafde6edb372d9bd25fb4ee7293ab20b916d867cd0baaa642529"

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
    # Version 14.0.0 is also available, but I can't seem to get it to work. I
    # get errors complaining about zlib not being available.
    # https://stackoverflow.com/questions/72230186/clang-14-warning-cannot-compress-debug-sections-zlib-not-installed-wdebug
    # makes it seem like version 14.0.0 was built improperly, and the easiest
    # solution is just to fall back to the previous working version, which was
    # 13.0.0. Another workaround would be to install Clang on the system and not
    # configure the toolchain in Bazel, but I'm trading off having a later
    # version for reproducibility.
    llvm_version = "13.0.0",
)

load("@llvm_toolchain//:toolchains.bzl", "llvm_register_toolchains")

llvm_register_toolchains()

http_archive(
    name = "hedron_compile_commands",
    sha256 = "1e9a72130f8cc7e52dc6e05baa7f1d690c699397dde56ac0fb9c15b98d168f08",
    strip_prefix = "bazel-compile-commands-extractor-1f154d0e1aaadb92aa25e901004b4c018eebbfc3",
    url = "https://github.com/hedronvision/bazel-compile-commands-extractor/archive/1f154d0e1aaadb92aa25e901004b4c018eebbfc3.tar.gz",
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
