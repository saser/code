"""cc_aoc_test : generate unit tests for C++ solutions."""

load("@rules_cc//cc:defs.bzl", "cc_test")

def _canonical_target(target):
    # Assume that all targets that start with ":" are targets in the same
    # package, and that all other targets are absolute.
    if target.startswith(":"):
        return "//" + native.package_name() + target

    if not target.startswith("//"):
        fail('Target % is neither of the form ":target" nor "//path/to:target" nor "//path/to/target"' % target)

    if not ":" in target:
        # Assume it's an absolute target, given in the short form
        # "//path/to/target", which should be expanded to
        # "//path/to/target:target".
        #
        # Skip over the leading "//" and split the rest of the target into
        # segments. Construct the canonical path by taking the last segment and
        # repeating it.
        segments = target[2:].split("/")
        return target + ":" + segments[-1]

    # Assume that the target is already in canonical form, i.e.,
    # "//path/to:target".
    return target

def cc_aoc_test(
        name,
        library,
        header_file = "",
        namespace = "",
        part1_func = "Part1",
        part2_func = "Part2",
        part1 = {},
        part2 = {}):
    """Generates a `cc_test` target for testing inputs against outputs.

    Args:
        name: string. Name of the test target.
        library: label. The `cc_library` target containing the solution.
        header_file: string. That which should be #include-d as the main module
            in the test. If empty, it will be derived from `library`: if the
            BUILD target of `library` is "//path/to:day01", `header_file` will
            be "path/to/day01.h".
        namespace: string. In which the solution functions live. If empty, it
            will be derived from `header_file`: if `header_file` is
            "path/to/day01.h", `namespace` will be "path::to::day01".
        part1_func: string. The function within `namespace` solving part 1.
        part2_func: string. The function within `namespace` solving part 2.
        part1: map[label]label. Keys are files containing inputs, values are
            files containing corresponding expected outputs. Must not be empty.
        part2: map[label]label. Keys are files containing inputs, values are
            files containing corresponding expected outputs. Must not be empty."""

    library = _canonical_target(library)

    if not header_file:
        # Split "//path/to:target" into dirs = ["path", "to:target"].
        dirs = library[2:].split("/")

        # Remove "to:target" from dirs and split into last = ["to", "target"].
        last = dirs.pop().split(":")

        # Append ".h" to last[-1] => last = ["to", "target.h"].
        last[-1] += ".h"

        # Join ["path"] + ["to", "target.h"] => "path/to/target.h".
        header_file = "/".join(dirs + last)

    if not namespace:
        # Transform header_file = "path/to/target.h" into "path::to::target".
        namespace = "::".join(header_file.removesuffix(".h").split("/"))

    if not part1:
        fail("part1_pairs must not be empty")

    if not part2:
        fail("part2_pairs must not be empty")

    output = name + ".cc"
    srcs = []
    outs = [output]
    cmd = [
        "$(location //adventofcode/cc/tools/generate_test)",
        "--header_file='%s'" % header_file,
        "--namespace='%s'" % namespace,
        "--part1_func='%s'" % part1_func,
        "--part2_func='%s'" % part2_func,
        "--output='$(location %s)'" % output,
    ]

    part1_pairs = []
    for in_file, out_file in part1.items():
        part1_pairs.append("$(location %s):$(location %s)" % (in_file, out_file))
        if in_file not in srcs:
            srcs.append(in_file)
        if out_file not in srcs:
            srcs.append(out_file)
    part1_pairs_arg = ",".join(part1_pairs)
    cmd.append("--part1_pairs='%s'" % part1_pairs_arg)

    part2_pairs = []
    for in_file, out_file in part2.items():
        part2_pairs.append("$(location %s):$(location %s)" % (in_file, out_file))
        if in_file not in srcs:
            srcs.append(in_file)
        if out_file not in srcs:
            srcs.append(out_file)
    part2_pairs_arg = ",".join(part2_pairs)
    cmd.append("--part2_pairs='%s'" % part2_pairs_arg)

    native.genrule(
        name = name + "_cc",
        srcs = srcs,
        outs = outs,
        cmd = " ".join(cmd),
        exec_tools = ["//adventofcode/cc/tools/generate_test"],
    )

    cc_test(
        name = name,
        deps = [library] + [
            "//adventofcode/cc:trim",
            "@com_google_googletest//:gtest_main",
        ],
        srcs = [output],
    )
