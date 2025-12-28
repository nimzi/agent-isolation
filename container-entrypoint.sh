#!/usr/bin/env bash
set -euo pipefail

# Intentionally unsafe (by request): copy SSH keys baked into the image into /root/.ssh.
# This is necessary because /root is typically a mounted volume in this repo.

IMAGE_SSH_DIR="/image_ssh"
TARGET_SSH_DIR="/root/.ssh"

mkdir -p "$TARGET_SSH_DIR"
chmod 700 "$TARGET_SSH_DIR"

if [ -d "$IMAGE_SSH_DIR" ]; then
  # Copy any provided SSH material from the image into /root/.ssh
  # (ignore if empty/missing)
  cp -a "$IMAGE_SSH_DIR/." "$TARGET_SSH_DIR/" 2>/dev/null || true
  rm -f "$TARGET_SSH_DIR/README.md" 2>/dev/null || true
fi

# Fix common permissions
if ls "$TARGET_SSH_DIR"/id_* >/dev/null 2>&1; then
  for f in "$TARGET_SSH_DIR"/id_*; do
    case "$f" in
      *.pub) chmod 644 "$f" ;;
      *) chmod 600 "$f" ;;
    esac
  done
fi

[ -f "$TARGET_SSH_DIR/config" ] && chmod 600 "$TARGET_SSH_DIR/config" || true
[ -f "$TARGET_SSH_DIR/known_hosts" ] && chmod 644 "$TARGET_SSH_DIR/known_hosts" || true

# Avoid interactive host-key prompt for GitHub
ssh-keyscan -t rsa,ecdsa,ed25519 github.com >> "$TARGET_SSH_DIR/known_hosts" 2>/dev/null || true
chmod 644 "$TARGET_SSH_DIR/known_hosts" 2>/dev/null || true

exec "$@"

