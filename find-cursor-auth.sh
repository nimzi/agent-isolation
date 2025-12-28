#!/bin/bash
# Helper script to discover Cursor Agent CLI authentication requirements

echo "=== Cursor Agent CLI Authentication Discovery ==="
echo ""

# Check if container exists and is running
CONTAINER_NAME="${OAI_SHELL_CONTAINER:-openai-shell}"
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "Error: Container '$CONTAINER_NAME' is not running."
    echo "Please rebuild the container first: ./recreate-container.sh"
    exit 1
fi

echo "1. Checking installed Cursor packages..."
docker exec "$CONTAINER_NAME" npm list -g 2>/dev/null | grep -i cursor || echo "   No Cursor packages found"

echo ""
echo "2. Checking for cursor command..."
docker exec "$CONTAINER_NAME" which cursor 2>/dev/null || echo "   'cursor' command not found in PATH"

echo ""
echo "3. Checking cursor --help (if available)..."
docker exec "$CONTAINER_NAME" cursor --help 2>&1 | head -30 || echo "   Command not available"

echo ""
echo "4. Checking for common Cursor environment variables..."
docker exec "$CONTAINER_NAME" env | grep -i cursor || echo "   No CURSOR_* environment variables found"

echo ""
echo "5. Checking for Cursor config files..."
docker exec "$CONTAINER_NAME" ls -la /root/.cursor 2>/dev/null || echo "   No ~/.cursor directory found"
docker exec "$CONTAINER_NAME" find /root -name "*cursor*" -type f 2>/dev/null | head -10 || echo "   No cursor config files found"

echo ""
echo "6. Checking npm registry for Cursor packages..."
echo "   Searching npm for Cursor-related packages..."
docker exec "$CONTAINER_NAME" npm search cursor 2>&1 | grep -i "@cursor\|cursor-agent" | head -10 || echo "   npm search not available or no results"

echo ""
echo "=== Next Steps ==="
echo "1. Visit https://cursor.sh/settings/api to get your API key"
echo "2. Check Cursor's documentation: https://docs.cursor.com"
echo "3. Try running 'cursor login' inside the container if available"
echo "4. Common environment variable names:"
echo "   - CURSOR_API_KEY"
echo "   - CURSOR_TOKEN"
echo "   - CURSOR_AUTH_TOKEN"
echo ""
echo "To test inside the container:"
echo "  docker exec -it $CONTAINER_NAME bash"
