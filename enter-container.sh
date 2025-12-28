#!/bin/bash
# Script to launch an interactive bash session in the container

CONTAINER_NAME="${OAI_SHELL_CONTAINER:-openai-shell}"

# Check if container exists
if ! docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Container '$CONTAINER_NAME' does not exist!" >&2
    echo "Please run recreate-container.sh first to create the container." >&2
    exit 1
fi

# Check if container is running, start it if not
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Container '$CONTAINER_NAME' is not running. Starting it..."
    if ! docker start "$CONTAINER_NAME" > /dev/null 2>&1; then
        echo "Error: Failed to start container '$CONTAINER_NAME'" >&2
        exit 1
    fi
    # Wait a bit longer for container to be ready
    sleep 2
    # Verify it's actually running
    if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Error: Container '$CONTAINER_NAME' failed to start" >&2
        exit 1
    fi
fi

# Ensure PATH includes ~/.local/bin for cursor-agent in .bashrc if not already there
docker exec "$CONTAINER_NAME" bash -c 'grep -q "\.local/bin" ~/.bashrc 2>/dev/null || echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> ~/.bashrc' 2>/dev/null || true

# Check if we have a TTY (interactive terminal)
if [ -t 0 ] && [ -t 1 ]; then
    # Launch interactive bash session with TTY
    # Ensure PATH includes ~/.local/bin for cursor-agent
    docker exec -it "$CONTAINER_NAME" bash -l -c "exec bash"
else
    # Non-interactive mode (no TTY available)
    echo "Warning: No TTY available. Running in non-interactive mode." >&2
    docker exec "$CONTAINER_NAME" bash -l -c "exec bash"
fi
