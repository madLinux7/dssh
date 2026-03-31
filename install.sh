#!/bin/sh
set -e

REPO="madLinux7/dssh"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
BINARY="dssh"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  freebsd) OS="freebsd" ;;
  *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  armv7l|armv6l)  ARCH="arm" ;;
  *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$TAG" ]; then
  echo "Error: could not determine latest release" >&2
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${TAG}/dssh-${OS}-${ARCH}"
echo "Downloading dssh ${TAG} for ${OS}/${ARCH}..."

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

if ! curl -fSL --progress-bar "$URL" -o "$TMP"; then
  echo "Error: download failed. Check that a binary exists for ${OS}/${ARCH}." >&2
  exit 1
fi

chmod +x "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP" "${INSTALL_DIR}/${BINARY}"

echo "dssh ${TAG} installed to ${INSTALL_DIR}/${BINARY}"

# Check if INSTALL_DIR is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Warning: ${INSTALL_DIR} is not in your PATH. Add it with:"
     echo "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac
