#!/bin/bash
set -e

REPO="nimzi/agent-isolation"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

# Get latest version and download
VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)
curl -fsSL "https://github.com/$REPO/releases/download/$VERSION/ai-shell-$OS-$ARCH" -o ai-shell
chmod +x ai-shell
sudo mv ai-shell "$INSTALL_DIR/ai-shell"
echo "Installed ai-shell $VERSION to $INSTALL_DIR/ai-shell"
