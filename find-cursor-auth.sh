#!/bin/bash
# Helper script to quickly sanity-check cursor-agent auth setup

echo "=== Cursor Agent CLI Sanity Check ==="
echo ""

# Check if container exists and is running
CONTAINER_NAME="${AI_SHELL_CONTAINER:-ai-agent-shell}"
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Container '$CONTAINER_NAME' is not running."
    echo "Please rebuild the container first: ./recreate-container.sh"
    exit 1
fi

echo "1. Checking cursor-agent CLI..."
docker exec "$CONTAINER_NAME" bash -lc "command -v cursor-agent && cursor-agent --help | head -30" 2>/dev/null || echo "   cursor-agent not found (re-run ./recreate-container.sh)"

echo ""
echo "2. Checking mounted Cursor credentials..."
docker exec "$CONTAINER_NAME" bash -lc "ls -la /root/.config/cursor/ 2>/dev/null | head -50" || echo "   /root/.config/cursor not found (host mount missing)"

echo ""
echo "3. Checking GitHub CLI auth (optional)..."
docker exec "$CONTAINER_NAME" bash -lc "gh auth status 2>&1 | head -50" || true

echo ""
echo "=== Next Steps ==="
echo "1. Ensure you are signed in to Cursor on the host (so $HOME/.config/cursor is populated)."
echo "2. Recreate the container to pick up mounts/env changes: ./recreate-container.sh"
echo "3. Enter the container: ./enter-container.sh"
echo ""
echo "Container name:"
echo "  $CONTAINER_NAME"
