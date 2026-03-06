#!/usr/bin/env bash
# Install project-level hooks so spawned teammates inherit gate enforcement.
# Plugin hooks only fire in the session that loaded the plugin (the lead).
# This script writes/merges hooks into .claude/settings.json, which
# project-level settings propagate to all sessions including teammates.
#
# Usage: ./hooks/scripts/install-hooks.sh
# Requires: jq

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SETTINGS_FILE=".claude/settings.json"

if ! command -v jq &>/dev/null; then
  echo "fellowship: jq is required for hook installation but was not found." >&2
  exit 2
fi

# Build the hooks JSON block with absolute paths to scripts.
HOOKS_JSON=$(cat <<EOF
{
  "PreToolUse": [
    {
      "matcher": "Edit|Write|Bash|Agent|Skill|NotebookEdit",
      "hooks": [{"type": "command", "command": "$SCRIPT_DIR/gate-guard.sh"}]
    },
    {
      "matcher": "SendMessage",
      "hooks": [{"type": "command", "command": "$SCRIPT_DIR/gate-submit.sh"}]
    },
    {
      "matcher": "TaskUpdate",
      "hooks": [{"type": "command", "command": "$SCRIPT_DIR/completion-guard.sh"}]
    }
  ],
  "PostToolUse": [
    {
      "matcher": "Skill",
      "hooks": [{"type": "command", "command": "$SCRIPT_DIR/gate-prereq.sh"}]
    },
    {
      "matcher": "TaskUpdate",
      "hooks": [{"type": "command", "command": "$SCRIPT_DIR/metadata-track.sh"}]
    }
  ]
}
EOF
)

mkdir -p .claude

if [ -f "$SETTINGS_FILE" ]; then
  # Merge: set .hooks on existing file, preserving all other keys.
  if ! jq --argjson hooks "$HOOKS_JSON" '.hooks = $hooks' "$SETTINGS_FILE" > "$SETTINGS_FILE.tmp"; then
    echo "fellowship: failed to merge hooks into $SETTINGS_FILE" >&2
    rm -f "$SETTINGS_FILE.tmp"
    exit 2
  fi
  mv "$SETTINGS_FILE.tmp" "$SETTINGS_FILE"
else
  # Create new file with just hooks.
  echo "{}" | jq --argjson hooks "$HOOKS_JSON" '.hooks = $hooks' > "$SETTINGS_FILE"
fi
