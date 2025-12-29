#!/bin/bash
# Script to recreate the container with updated image and environment variables

set -e

CONTAINER_NAME="${AI_SHELL_CONTAINER:-ai-agent-shell}"
IMAGE_NAME="${AI_SHELL_IMAGE:-ai-agent-shell}"
VOLUME_NAME="${AI_SHELL_VOLUME:-ai_agent_shell_home}"

ENV_FILE_ARGS=()
if [ -f .env ]; then
  ENV_FILE_ARGS=(--env-file .env)
else
  echo "Note: No .env found (this is fine)."
  echo "      If you want non-interactive GitHub auth for 'gh', create .env with: GH_TOKEN=..."
fi

echo "Stopping and removing existing container (if it exists)..."
docker stop "$CONTAINER_NAME" 2>/dev/null || true
docker rm "$CONTAINER_NAME" 2>/dev/null || true

echo "Building Docker image..."
docker build -t "$IMAGE_NAME" .

echo "Creating new container..."

# Ensure Cursor config directory exists on host
CURSOR_CONFIG_DIR="$HOME/.config/cursor"
if [ ! -d "$CURSOR_CONFIG_DIR" ]; then
    echo "Creating Cursor config directory: $CURSOR_CONFIG_DIR"
    mkdir -p "$CURSOR_CONFIG_DIR"
    echo "Note: Directory created, but you need to install and configure Cursor on your host machine"
    echo "      for Cursor Agent CLI to work properly."
    echo "      Install Cursor from: https://cursor.sh"
else
    echo "Found Cursor config directory: $CURSOR_CONFIG_DIR"
    # Verify it has content (optional check)
    if [ -z "$(ls -A "$CURSOR_CONFIG_DIR" 2>/dev/null)" ]; then
        echo "Warning: Directory exists but is empty. Cursor may not be configured yet."
    else
        echo "Directory contains Cursor configuration files."
    fi
fi

echo "Mounting Cursor credentials from $CURSOR_CONFIG_DIR to /root/.config/cursor"

docker run -d \
    --name "$CONTAINER_NAME" \
    -v "$(pwd):/work" \
    -v "$VOLUME_NAME:/root" \
    -v "$CURSOR_CONFIG_DIR:/root/.config/cursor" \
    "${ENV_FILE_ARGS[@]}" \
    "$IMAGE_NAME"

echo "Container '$CONTAINER_NAME' created and started!"

# Install cursor-agent if not already installed (it needs to be in the volume, not the image)
echo "Installing cursor-agent CLI (if not already installed)..."
if ! docker exec "$CONTAINER_NAME" bash -c "command -v cursor-agent >/dev/null 2>&1"; then
    echo "Installing cursor-agent..."
    docker exec "$CONTAINER_NAME" bash -c "curl https://cursor.com/install -fsSL | bash" > /dev/null 2>&1
    # Ensure ~/.local/bin is in PATH for future sessions
    docker exec "$CONTAINER_NAME" bash -c 'echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> ~/.bashrc' 2>/dev/null || true
    echo "cursor-agent installed successfully!"
else
    echo "cursor-agent already installed."
fi

echo "You can now use: ./enter-container.sh"
