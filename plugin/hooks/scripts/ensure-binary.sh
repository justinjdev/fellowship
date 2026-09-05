#!/usr/bin/env bash
# Downloads the fellowship binary from GitHub releases if not present.
# Caches at ~/.claude/fellowship/bin/fellowship.
#
# Safety properties:
#   - Downloads are checksum-verified against the release's checksums.txt
#     before the binary is installed or ".version" is written.
#   - The binary is assembled in a scratch directory next to the final
#     location and moved into place atomically; a failed or interrupted
#     install never leaves a partial/corrupt binary at $BINARY.
#   - A mkdir-based lock keeps two concurrent sessions from racing the
#     same install.
#   - "installed" is only ever printed after the binary is verified,
#     executable, and moved into place.

set -euo pipefail

INSTALL_DIR="$HOME/.claude/fellowship/bin"
BINARY="$INSTALL_DIR/fellowship"
REPO="justinjdev/fellowship"
VERSION_FILE="$INSTALL_DIR/.version"
LOCK_DIR="$INSTALL_DIR/.lock"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# A plain `grep -o '"version"...'` would match ANY key named "version"
# anywhere in the file — including one day showing up nested inside some
# other object (e.g. a future per-entry version field) — and `head -1`
# would then silently prefer whichever happens to appear first in the file,
# not the top-level manifest version this script actually needs. There's no
# jq (or any JSON tooling) guaranteed present on end-user machines, and
# pulling one in just to read a single field isn't worth the added
# dependency, so this walks brace depth by hand and only accepts a
# "version" key seen while depth == 1 (directly inside the outermost { }).
# That's not a general JSON parser — a string value containing a literal
# `{` or `}` would throw the count off — but plugin.json is a small,
# hand-maintained, one-key-per-line manifest, so that trade-off is fine
# here. Guarded with `|| true`: under `set -o pipefail` a no-match here
# would otherwise trip `set -e` before we get a chance to report a clean
# error.
VERSION=$(awk '
  {
    line = $0
    for (i = 1; i <= length(line); i++) {
      c = substr(line, i, 1)
      if (c == "{") depth++
      else if (c == "}") depth--
    }
  }
  depth == 1 && match($0, /"version"[[:space:]]*:[[:space:]]*"[^"]*"/) {
    val = substr($0, RSTART, RLENGTH)
    sub(/^"version"[[:space:]]*:[[:space:]]*"/, "", val)
    sub(/"$/, "", val)
    print val
    exit
  }
' "$PLUGIN_ROOT/.claude-plugin/plugin.json" 2>/dev/null) || true

if [ -z "$VERSION" ]; then
  echo "fellowship: could not determine version from plugin.json" >&2
  exit 2
fi

# Fast path: already installed at the right version. No lock needed.
if [ -x "$BINARY" ] && [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE")" = "$VERSION" ]; then
  exit 0
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin|linux) ;;
  *)
    echo "fellowship: unsupported OS '$OS' — fellowship only ships darwin and linux binaries" >&2
    exit 2
    ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "fellowship: unsupported architecture $ARCH" >&2; exit 2 ;;
esac

if command -v sha256sum >/dev/null 2>&1; then
  SHA_TOOL="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
  SHA_TOOL="shasum"
else
  echo "fellowship: sha256sum or shasum required to verify the download" >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
  echo "fellowship: curl or wget required to download the binary" >&2
  exit 2
fi

download() {
  # download <url> <dest>
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "$2" "$1"
  else
    wget -q -O "$2" "$1"
  fi
}

mkdir -p "$INSTALL_DIR"

LOCK_ACQUIRED=0
acquire_lock() {
  local tries=0
  while ! mkdir "$LOCK_DIR" 2>/dev/null; do
    tries=$((tries + 1))
    if [ "$tries" -ge 100 ]; then
      return 1
    fi
    sleep 0.1
  done
  LOCK_ACQUIRED=1
  return 0
}

TMP_DIR=""
cleanup() {
  local ec=$?
  if [ -n "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
  if [ "$LOCK_ACQUIRED" = "1" ]; then
    rmdir "$LOCK_DIR" 2>/dev/null || true
  fi
  exit "$ec"
}
trap cleanup EXIT

if ! acquire_lock; then
  # Another session is (or was) installing. If it finished successfully in
  # the meantime, just use its result instead of failing this session.
  if [ -x "$BINARY" ] && [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE")" = "$VERSION" ]; then
    exit 0
  fi
  echo "fellowship: timed out waiting for another install to finish (lock: $LOCK_DIR)" >&2
  exit 2
fi

# Re-check now that we hold the lock: another session may have just finished.
if [ -x "$BINARY" ] && [ -f "$VERSION_FILE" ] && [ "$(cat "$VERSION_FILE")" = "$VERSION" ]; then
  exit 0
fi

TARBALL="fellowship_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/v${VERSION}"

# Scratch dir on the same filesystem as INSTALL_DIR so the final `mv` is atomic.
TMP_DIR=$(mktemp -d "$INSTALL_DIR/.install.XXXXXX")

echo "fellowship: downloading $TARBALL ($OS/$ARCH) for v$VERSION..." >&2

if ! download "$BASE_URL/$TARBALL" "$TMP_DIR/$TARBALL"; then
  echo "fellowship: failed to download $BASE_URL/$TARBALL" >&2
  exit 2
fi

if ! download "$BASE_URL/checksums.txt" "$TMP_DIR/checksums.txt"; then
  echo "fellowship: failed to download $BASE_URL/checksums.txt" >&2
  exit 2
fi

CHECKSUM_LINE=$(grep " ${TARBALL}\$" "$TMP_DIR/checksums.txt" || true)
if [ -z "$CHECKSUM_LINE" ]; then
  echo "fellowship: no checksum entry for $TARBALL in checksums.txt — refusing to install" >&2
  exit 2
fi

VERIFY_OK=0
case "$SHA_TOOL" in
  sha256sum)
    if (cd "$TMP_DIR" && echo "$CHECKSUM_LINE" | sha256sum -c - >/dev/null 2>&1); then
      VERIFY_OK=1
    fi
    ;;
  shasum)
    if (cd "$TMP_DIR" && echo "$CHECKSUM_LINE" | shasum -a 256 -c - >/dev/null 2>&1); then
      VERIFY_OK=1
    fi
    ;;
esac

if [ "$VERIFY_OK" != "1" ]; then
  echo "fellowship: checksum verification failed for $TARBALL — refusing to install" >&2
  exit 2
fi

if ! tar xzf "$TMP_DIR/$TARBALL" -C "$TMP_DIR" fellowship; then
  echo "fellowship: failed to extract $TARBALL" >&2
  exit 2
fi

chmod +x "$TMP_DIR/fellowship"

if [ ! -x "$TMP_DIR/fellowship" ]; then
  echo "fellowship: extracted binary is not executable" >&2
  exit 2
fi

# Atomic within INSTALL_DIR: same filesystem as $BINARY.
mv -f "$TMP_DIR/fellowship" "$BINARY"

# Only recorded once the binary is verified, executable, and in place.
echo "$VERSION" > "$VERSION_FILE"
echo "fellowship: installed $VERSION ($OS/$ARCH)" >&2
