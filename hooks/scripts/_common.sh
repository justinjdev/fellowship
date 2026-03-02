#!/usr/bin/env bash
# Shared utilities for fellowship gate hooks.
# Source this at the top of every hook script.

STATE_FILE="tmp/quest-state.json"

# No state file = not a quest teammate session. Exit immediately.
if [ ! -f "$STATE_FILE" ]; then
  exit 0
fi

# Check for jq dependency.
if ! command -v jq &>/dev/null; then
  echo "fellowship: jq not found, gate enforcement disabled" >&2
  exit 0
fi

# Read state into variable for use by the sourcing script.
STATE=$(cat "$STATE_FILE")
