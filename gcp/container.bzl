"""container.bzl: macros to work with rules_docker definitions but adjusted for
the GCP environment."""

load("@io_bazel_rules_docker//container:container.bzl", "container_push")
load(":constants.bzl", "DOCKER_REPOSITORY", "PROJECT_ID", "REGION")

def gcp_docker_push(name, image, repository, tag, **kwargs):
    """Wraps container_push to provide default arguments for pushing to GCP.

    Args:
        name: string. Name of target.
        image: Label. the container image to push.
        repository: string. What path under the Artifact Registry repository to push to. Example: "tasks/server/server_image".
        tag: the tag to attach to the image.
        **kwargs: arguments for container_push in rules_docker.
    """

    container_push(
        name = name,
        image = image,
        format = "Docker",
        registry = REGION + "-docker.pkg.dev",
        repository = PROJECT_ID + "/" + DOCKER_REPOSITORY + "/" + repository,
        tag = tag,
        **kwargs
    )
