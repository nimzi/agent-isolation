#!/bin/bash
# Script to start or stop the Docker container without rebuilding

set -e

CONTAINER_NAME="${AI_SHELL_CONTAINER:-ai-agent-shell}"

# Function to check if container exists
container_exists() {
    docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"
}

# Function to check if container is running
container_running() {
    docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"
}

# Function to start the container
start_container() {
    if ! container_exists; then
        echo "Error: Container '$CONTAINER_NAME' does not exist!" >&2
        echo "Please run recreate-container.sh first to create the container." >&2
        exit 1
    fi

    if container_running; then
        echo "Container '$CONTAINER_NAME' is already running."
        exit 0
    fi

    echo "Starting container '$CONTAINER_NAME'..."
    if docker start "$CONTAINER_NAME" > /dev/null 2>&1; then
        # Wait a moment for container to be ready
        sleep 1
        if container_running; then
            echo "Container '$CONTAINER_NAME' started successfully."
        else
            echo "Error: Container '$CONTAINER_NAME' failed to start." >&2
            exit 1
        fi
    else
        echo "Error: Failed to start container '$CONTAINER_NAME'." >&2
        exit 1
    fi
}

# Function to stop the container
stop_container() {
    if ! container_exists; then
        echo "Error: Container '$CONTAINER_NAME' does not exist!" >&2
        exit 1
    fi

    if ! container_running; then
        echo "Container '$CONTAINER_NAME' is not running."
        exit 0
    fi

    echo "Stopping container '$CONTAINER_NAME'..."
    if docker stop "$CONTAINER_NAME" > /dev/null 2>&1; then
        echo "Container '$CONTAINER_NAME' stopped successfully."
    else
        echo "Error: Failed to stop container '$CONTAINER_NAME'." >&2
        exit 1
    fi
}

# Main script logic
case "${1:-}" in
    start)
        start_container
        ;;
    stop)
        stop_container
        ;;
    *)
        echo "Usage: $0 {start|stop}" >&2
        echo "" >&2
        echo "Commands:" >&2
        echo "  start  - Start the Docker container" >&2
        echo "  stop   - Stop the Docker container" >&2
        echo "" >&2
        echo "Container name: $CONTAINER_NAME" >&2
        echo "(Override with AI_SHELL_CONTAINER environment variable)" >&2
        exit 1
        ;;
esac
