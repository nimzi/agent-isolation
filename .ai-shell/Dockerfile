ARG BASE_IMAGE=ubuntu:24.04
FROM ${BASE_IMAGE}

WORKDIR /work

# Run as root: some base images default to a non-root user, but ai-shell
# bootstrap scripts need root for package installation.
USER root
ENV HOME=/root
ENV PATH="/root/.local/bin:${PATH}"

# Clear any base-image entrypoint so the CMD below runs directly.
# Some images (e.g. ocaml/opam) set an entrypoint that fails on a fresh /root volume.
ENTRYPOINT []

# Keep container running without assuming tail/coreutils.
CMD ["sh", "-c", "while true; do sleep 3600; done"]
