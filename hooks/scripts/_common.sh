#!/usr/bin/env bash
# Shared utilities for fellowship gate hooks.
# Source this at the top of every hook script.

# Resolve state file path from the repo/worktree root, so hooks work
# even if the session's cwd drifts within the repo.
_ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || _ROOT="$PWD"
STATE_FILE="$_ROOT/tmp/quest-state.json"

# No state file = not a quest teammate session. Exit immediately.
if [ ! -f "$STATE_FILE" ]; then
  exit 0
fi

# Check for jq dependency.
if ! command -v jq &>/dev/null; then
  echo "fellowship: jq is required for gate enforcement but was not found. Install jq to proceed." >&2
  exit 2
fi

# Read state into variable for use by the sourcing script.
STATE=$(cat "$STATE_FILE") || {
  echo "fellowship: failed to read state file $STATE_FILE" >&2
  exit 2
}

if [ -z "$STATE" ] || ! echo "$STATE" | jq empty 2>/dev/null; then
  echo "fellowship: state file $STATE_FILE is empty or contains invalid JSON" >&2
  exit 2
fi
