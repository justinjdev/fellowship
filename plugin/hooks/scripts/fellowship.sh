#!/usr/bin/env bash
# Thin wrapper — ensures the fellowship binary exists, then execs it with
# all args.
#
# Posture: gate hooks (gate-guard, gate-submit, gate-prereq,
# completion-guard, metadata-track, file-track) fail *closed* — if the
# binary cannot be made available, this script blocks the tool call
# (exit 2) so enforcement can't be silently skipped just because the
# binary hasn't been installed yet. worktree-guard is defense-in-depth
# behind lead-provisioned isolation, so it fails *open* (exit 0) when the
# binary is unavailable, matching its posture inside the CLI itself.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$HOME/.claude/fellowship/bin/fellowship"

# The hook name is the argument immediately after "hook", e.g.
# `fellowship.sh hook gate-guard` -> HOOK_NAME=gate-guard.
HOOK_NAME=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "hook" ]; then
    HOOK_NAME="$arg"
    break
  fi
  prev="$arg"
done

binary_ready() {
  [ -x "$BINARY" ]
}

if ! binary_ready; then
  "$SCRIPT_DIR/ensure-binary.sh" || true
fi

if ! binary_ready; then
  if [ "$HOOK_NAME" = "worktree-guard" ]; then
    # Fail open: backstop behind lead-provisioned isolation, not the
    # primary enforcement mechanism.
    exit 0
  fi
  echo "fellowship: binary unavailable at $BINARY after install attempt — blocking (hook: ${HOOK_NAME:-unknown}) to fail closed" >&2
  exit 2
fi

exec "$BINARY" "$@"
