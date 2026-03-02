#!/bin/sh
set -e

REPO="wonfull888/fastmd"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="fastmd"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version. Check your internet connection."
  exit 1
fi

FILENAME="${BIN_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing fastmd ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp)
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

# Try to install to /usr/local/bin, fallback to ~/.local/bin
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BIN_NAME}"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
  mv "$TMP" "${INSTALL_DIR}/${BIN_NAME}"
  echo "Installed to ${INSTALL_DIR}/${BIN_NAME}"
  echo "Make sure ${INSTALL_DIR} is in your PATH:"
  echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

echo "✓ fastmd ${VERSION} installed successfully"
echo "Run: fastmd --version"
