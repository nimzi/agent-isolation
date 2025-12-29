FROM python:3.12-slim

# Useful tools for debugging & typical workflows
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl git jq make openssh-client ripgrep opam gh \
    iputils-ping dnsutils netcat-traditional procps gnupg \
  && rm -rf /var/lib/apt/lists/*

# Install Node.js and npm (for Cursor Agent CLI)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
  && apt-get install -y --no-install-recommends nodejs \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /work

# (No API-provider tooling.) Python is kept for general scripting convenience.

# Intentionally unsafe (by request): bake SSH keys into the image.
# We store them outside /root because /root is typically a mounted volume at runtime.
COPY docker_ssh/ /image_ssh/
RUN set -eu; \
    rm -f /image_ssh/README.md 2>/dev/null || true; \
    chmod 700 /image_ssh || true; \
    if ls /image_ssh/id_* >/dev/null 2>&1; then \
      for f in /image_ssh/id_*; do \
        case "$f" in \
          *.pub) chmod 644 "$f" ;; \
          *) chmod 600 "$f" ;; \
        esac; \
      done; \
    fi; \
    [ -f /image_ssh/config ] && chmod 600 /image_ssh/config || true; \
    [ -f /image_ssh/known_hosts ] && chmod 644 /image_ssh/known_hosts || true

COPY container-entrypoint.sh /usr/local/bin/container-entrypoint.sh
RUN chmod +x /usr/local/bin/container-entrypoint.sh

# Note: cursor-agent CLI is installed in recreate-container.sh after container creation
# This is because /root is mounted as a volume, which would overwrite the installation
# Installing it in the recreate script ensures it persists in the volume

# Keep container running - use tail to prevent immediate exit
ENTRYPOINT ["/usr/local/bin/container-entrypoint.sh"]
CMD ["tail", "-f", "/dev/null"]
