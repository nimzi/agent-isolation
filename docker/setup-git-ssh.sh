#!/usr/bin/env bash
set -euo pipefail

# Setup script to enable SSH access for git operations with GitHub
# This script:
# 1. Generates an SSH key pair if one doesn't exist
# 2. Adds the SSH key to GitHub using gh CLI
# 3. Configures git to use SSH URLs instead of HTTPS

SSH_DIR="${HOME}/.ssh"
SSH_KEY="${SSH_DIR}/id_ed25519"
SSH_KEY_PUB="${SSH_KEY}.pub"

# Ensure .ssh directory exists with correct permissions
mkdir -p "$SSH_DIR"
chmod 700 "$SSH_DIR"

# Check if gh is installed and authenticated
if ! command -v gh &> /dev/null; then
    echo "Error: gh CLI is not installed" >&2
    exit 1
fi

if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with gh CLI. Run 'gh auth login' first" >&2
    exit 1
fi

# Generate SSH key if it doesn't exist
if [ ! -f "$SSH_KEY" ]; then
    echo "Generating SSH key pair..."
    ssh-keygen -t ed25519 -C "gh-setup-$(hostname)" -f "$SSH_KEY" -N ""
    chmod 600 "$SSH_KEY"
    chmod 644 "$SSH_KEY_PUB"
    echo "✓ SSH key generated at $SSH_KEY"
else
    echo "SSH key already exists at $SSH_KEY"
fi

# Add GitHub to known_hosts to avoid interactive prompt
if [ ! -f "${SSH_DIR}/known_hosts" ] || ! grep -q "github.com" "${SSH_DIR}/known_hosts" 2>/dev/null; then
    echo "Adding GitHub to known_hosts..."
    ssh-keyscan -t rsa,ecdsa,ed25519 github.com >> "${SSH_DIR}/known_hosts" 2>/dev/null || true
    chmod 644 "${SSH_DIR}/known_hosts"
fi

# Check if this SSH public key is already added to GitHub
PUBKEY="$(<"$SSH_KEY_PUB")"
KEY_FINGERPRINT="$(ssh-keygen -lf "$SSH_KEY_PUB" | awk '{print $2}')"
KEY_TITLE="ai-shell-${KEY_FINGERPRINT}"

if gh api user/keys --jq '.[].key' 2>/dev/null | grep -Fqx "$PUBKEY"; then
    echo "SSH key already added to GitHub account"
else
    echo "Adding SSH key to GitHub account..."
    gh ssh-key add "$SSH_KEY_PUB" --title "$KEY_TITLE"
    echo "✓ SSH key added to GitHub account"
fi

# Configure git to use SSH URLs instead of HTTPS
if git config --global --get url."git@github.com:".insteadOf "https://github.com/" &> /dev/null; then
    echo "Git is already configured to use SSH URLs"
else
    echo "Configuring git to use SSH URLs..."
    git config --global url."git@github.com:".insteadOf "https://github.com/"
    echo "✓ Git configured to use SSH URLs"
fi

# Test SSH connection
echo ""
echo "Testing SSH connection to GitHub..."
if ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
    echo "✓ SSH access verified successfully"
else
    echo "Warning: SSH connection test did not succeed as expected" >&2
    exit 1
fi

echo ""
echo "Setup complete! Git operations will now use SSH authentication."
