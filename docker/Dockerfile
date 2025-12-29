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

COPY setup-git-ssh.sh /docker/setup-git-ssh.sh
RUN chmod +x /docker/setup-git-ssh.sh

# Note: cursor-agent CLI is installed by `ai-shell up` after container creation
# This is because /root is mounted as a volume, which would overwrite the installation
# Installing it after container creation ensures it persists in the /root volume

# Keep container running - use tail to prevent immediate exit
CMD ["tail", "-f", "/dev/null"]
