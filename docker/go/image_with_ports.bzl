"""Defines a `go_image` target that also exposes ports."""

load("@io_bazel_rules_docker//container:image.bzl", "container_image")
load("@io_bazel_rules_docker//go:image.bzl", "go_image")

def go_image_with_ports(
        name,
        ports = [],
        **kwargs):
    """Defines a `go_image` target that also exposes ports.

    The `ports` argument will be passed to `container_image`. The remaining
    kwargs will be passed to `go_image`.

    Example:
        go_image_with_ports(
            name = "image",
            ports = ["8080/tcp"],
            binary = ":server",
        )
    """

    with_ports = "_" + name + "_with_ports"
    container_image(
        name = with_ports,
        base = "@go_image_base//image",
        ports = ports,
    )

    go_image(
        name = name,
        base = ":" + with_ports,
        **kwargs
    )
