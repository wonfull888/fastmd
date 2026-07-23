#!/bin/sh
set -e

REPO="wonfull888/fastmd"
BIN_NAME="fastmd"
INSTALL_DIR="/usr/local/bin"

# Method 1: go install (recommended, works on all platforms)
if command -v go >/dev/null 2>&1; then
    echo "Installing fastmd via go install..."
    go install "github.com/${REPO}/cmd/cli@latest"
    echo "✓ fastmd installed successfully"
    echo "Run: fastmd --version"
    exit 0
fi

# Method 2: download pre-built binary (fallback, no Go required)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version. Please install Go and try again."
  exit 1
fi

FILENAME="${BIN_NAME}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing fastmd ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp)
curl -fsSL "$URL" -o "$TMP" || {
    echo "Pre-built binary not available. Please install Go and run:"
    echo "  go install github.com/${REPO}/cmd/cli@latest"
    exit 1
}
chmod +x "$TMP"

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
