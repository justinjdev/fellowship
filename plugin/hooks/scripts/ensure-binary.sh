#!/usr/bin/env bash
# Downloads the fellowship binary from GitHub releases if not present.
# Caches at ~/.claude/fellowship/bin/fellowship.

set -uo pipefail

INSTALL_DIR="$HOME/.claude/fellowship/bin"
BINARY="$INSTALL_DIR/fellowship"
REPO="justinjdev/fellowship"

if [ -x "$BINARY" ]; then
  exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "fellowship: unsupported architecture $ARCH" >&2; exit 2 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VERSION=$(grep -o '"version"[[:space:]]*:[[:space:]]*"[^"]*"' "$PLUGIN_ROOT/.claude-plugin/plugin.json" | head -1 | grep -o '"[^"]*"$' | tr -d '"')

if [ -z "$VERSION" ]; then
  echo "fellowship: could not determine version from plugin.json" >&2
  exit 2
fi

TARBALL="fellowship_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/v${VERSION}/${TARBALL}"

mkdir -p "$INSTALL_DIR"
echo "fellowship: downloading $URL..." >&2

if command -v curl &>/dev/null; then
  curl -sL "$URL" | tar xz -C "$INSTALL_DIR" fellowship
elif command -v wget &>/dev/null; then
  wget -qO- "$URL" | tar xz -C "$INSTALL_DIR" fellowship
else
  echo "fellowship: curl or wget required to download binary" >&2
  exit 2
fi

chmod +x "$BINARY"
echo "fellowship: installed $VERSION ($OS/$ARCH)" >&2
