#!/bin/sh
set -eu

# Runtime customization hook — runs on every 'ai-shell up'.
#
# Base tools (git, gh, curl, ssh, etc.) are installed at image build time
# via the Dockerfile. Use this script for anything that must be installed
# at runtime (e.g., into the persistent /root volume).
#
# Example:
#   pip install numpy pandas
