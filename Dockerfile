FROM python:3.12-slim

# Useful tools for debugging & typical workflows
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl git jq make openssh-client ripgrep \
    iputils-ping dnsutils netcat-traditional procps gnupg \
  && rm -rf /var/lib/apt/lists/*

# Install Node.js and npm (for Cursor Agent CLI)
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
  && apt-get install -y --no-install-recommends nodejs \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /work

# Install OpenAI SDK + helpers
RUN pip install --no-cache-dir openai python-dotenv rich

# Note: cursor-agent CLI is installed in recreate-container.sh after container creation
# This is because /root is mounted as a volume, which would overwrite the installation
# Installing it in the recreate script ensures it persists in the volume

# Keep container running - use tail to prevent immediate exit
CMD ["tail", "-f", "/dev/null"]
