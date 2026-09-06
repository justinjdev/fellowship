#!/usr/bin/env bash
# Thin wrapper — execs the fellowship binary with all args. Never installs
# it: only the SessionStart hook (ensure-binary.sh) does that, so a hook
# invocation never blocks the tool call it's guarding on a network download.
#
# Posture: gate hooks (gate-guard, gate-submit, gate-prereq, file-track,
# agent-track) fail *closed* — if the binary isn't installed yet, this script blocks
# the tool call (exit 2)
# rather than trying to fetch it inline, so enforcement can't be silently
# skipped. worktree-guard is defense-in-depth behind lead-provisioned
# isolation, so it fails *open* (exit 0) when the binary is unavailable,
# matching its posture inside the CLI itself.

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

if [ ! -x "$BINARY" ]; then
  if [ "$HOOK_NAME" = "worktree-guard" ]; then
    # Fail open: backstop behind lead-provisioned isolation, not the
    # primary enforcement mechanism.
    exit 0
  fi
  echo "fellowship: binary not installed; it is installed at session start — restart the session or run $SCRIPT_DIR/ensure-binary.sh" >&2
  exit 2
fi

exec "$BINARY" "$@"
