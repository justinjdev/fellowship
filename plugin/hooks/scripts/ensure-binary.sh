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
#     same install. The lock records its holder's PID and a timestamp, so a
#     session killed mid-install (kill -9, OOM) doesn't wedge every future
#     install: a contending session reclaims the lock once the recorded PID
#     is no longer alive, or once 120s have passed regardless. Leftover
#     scratch dirs from a killed install are swept once they're 10 minutes old.
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

PLUGIN_JSON="$PLUGIN_ROOT/.claude-plugin/plugin.json"

# Fast path: a real JSON parser reads the top-level "version" field exactly,
# with no risk of matching a same-named key nested elsewhere. Neither
# python3 nor node is guaranteed present on end-user machines (that's why
# this is a fast path in front of the awk fallback below, not a hard
# dependency) but both are common on dev/CI machines, where they make this
# start-of-session hook do less string-munging. Each is only used if found
# on PATH; either failure mode (not found, or the read/parse itself fails)
# falls through to awk. Constraint: keep plugin.json parseable as plain JSON
# by both of these paths and by the awk fallback — no comments, no trailing
# commas, "version" a direct top-level string field.
VERSION=""
if command -v python3 >/dev/null 2>&1; then
  VERSION=$(python3 -c '
import json, sys
try:
    with open(sys.argv[1]) as f:
        sys.stdout.write(json.load(f)["version"])
except Exception:
    sys.exit(1)
' "$PLUGIN_JSON" 2>/dev/null) || VERSION=""
elif command -v node >/dev/null 2>&1; then
  VERSION=$(node -e '
const fs = require("fs");
try {
  const data = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  process.stdout.write(String(data.version));
} catch (e) {
  process.exit(1);
}
' "$PLUGIN_JSON" 2>/dev/null) || VERSION=""
fi

if [ -z "$VERSION" ]; then
  # Fallback: no python3 or node on PATH (or the fast path above failed).
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
  ' "$PLUGIN_JSON" 2>/dev/null) || true
fi

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

# Sweep scratch dirs left behind by a session that was killed (kill -9, OOM,
# etc.) mid-install — the EXIT trap that would normally remove them never
# gets to run. Bounded to dirs older than 10 minutes so a scratch dir from an
# install that is still genuinely in flight is never touched.
find "$INSTALL_DIR" -maxdepth 1 -name '.install.*' -mmin +10 -exec rm -rf {} + 2>/dev/null || true

LOCK_DIR_OWNER="$LOCK_DIR/owner"
STALE_LOCK_SECONDS=120

# lock_is_stale: the lock dir has no identifiable, still-running owner, so it
# is safe to reclaim. True (0) when: there's no owner record at all, the
# recorded PID is not alive (kill -0 fails — the holder session exited
# without running its EXIT trap, e.g. kill -9), or the owner's timestamp is
# older than STALE_LOCK_SECONDS regardless of liveness (a wedged holder).
lock_is_stale() {
  local pid ts now
  if [ ! -f "$LOCK_DIR_OWNER" ]; then
    return 0
  fi
  read -r pid ts < "$LOCK_DIR_OWNER" 2>/dev/null || true
  if [ -z "$pid" ] || ! kill -0 "$pid" 2>/dev/null; then
    return 0
  fi
  if [ -z "$ts" ]; then
    return 0
  fi
  now=$(date +%s)
  if [ $((now - ts)) -ge "$STALE_LOCK_SECONDS" ]; then
    return 0
  fi
  return 1
}

LOCK_ACQUIRED=0
acquire_lock() {
  local tries=0
  while true; do
    if mkdir "$LOCK_DIR" 2>/dev/null; then
      LOCK_ACQUIRED=1
      # Record who holds the lock so a contending session can tell a live
      # install apart from an abandoned one. Best-effort: if this write
      # fails, the lock still works, it just always looks reclaimable.
      echo "$$ $(date +%s)" > "$LOCK_DIR_OWNER" 2>/dev/null || true
      return 0
    fi
    if lock_is_stale; then
      # Reclaim: the holder is gone or wedged. Remove the whole lock dir
      # (owner file included) and retry mkdir immediately, without counting
      # it against the contention budget below.
      rm -rf "$LOCK_DIR" 2>/dev/null || true
      continue
    fi
    tries=$((tries + 1))
    if [ "$tries" -ge 100 ]; then
      return 1
    fi
    sleep 0.1
  done
}

TMP_DIR=""
cleanup() {
  local ec=$?
  if [ -n "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
  if [ "$LOCK_ACQUIRED" = "1" ]; then
    rm -rf "$LOCK_DIR" 2>/dev/null || true
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
