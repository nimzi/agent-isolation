#!/bin/sh
set -eu

# Minimal bootstrap: ensure python3 exists, then run the Python bootstrap.
# This script is POSIX sh on purpose (don't assume bash exists yet).

log() { printf '%s\n' "$*" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

detect_pm() {
  if have apt-get; then echo apt; return 0; fi
  if have dnf; then echo dnf; return 0; fi
  if have yum; then echo yum; return 0; fi
  if have zypper; then echo zypper; return 0; fi
  if have pacman; then echo pacman; return 0; fi
  if have apk; then echo apk; return 0; fi
  return 1
}

pm="$(detect_pm 2>/dev/null || true)"
if [ -z "${pm}" ]; then
  log "ERROR: no supported package manager found (expected apt-get/dnf/yum/zypper/pacman/apk)."
  exit 1
fi

if ! have python3; then
  log "Installing python3 (package manager: ${pm})..."
  case "${pm}" in
    apt)
      DEBIAN_FRONTEND=noninteractive apt-get update -y
      DEBIAN_FRONTEND=noninteractive apt-get install -y python3
      ;;
    dnf)
      dnf -y install python3
      ;;
    yum)
      yum -y install python3
      ;;
    zypper)
      # Use non-interactive + auto-import keys to avoid prompts.
      zypper -n --gpg-auto-import-keys refresh || true
      zypper -n --gpg-auto-import-keys install -y python3
      ;;
    pacman)
      pacman -Sy --noconfirm python
      ;;
    apk)
      apk add --no-cache python3
      ;;
    *)
      log "ERROR: unsupported package manager: ${pm}"
      exit 1
      ;;
  esac
fi

exec python3 /docker/bootstrap-tools.py "$@"

