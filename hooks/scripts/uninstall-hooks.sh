#!/usr/bin/env bash
# Remove fellowship gate hooks from .claude/settings.json.
# If hooks were the only content, removes the file entirely.
#
# Usage: ./hooks/scripts/uninstall-hooks.sh
# Requires: jq

set -uo pipefail

SETTINGS_FILE=".claude/settings.json"

if [ ! -f "$SETTINGS_FILE" ]; then
  exit 0
fi

if ! command -v jq &>/dev/null; then
  echo "fellowship: jq is required for hook uninstallation but was not found." >&2
  exit 2
fi

# Remove the hooks key.
if ! jq 'del(.hooks)' "$SETTINGS_FILE" > "$SETTINGS_FILE.tmp"; then
  echo "fellowship: failed to remove hooks from $SETTINGS_FILE" >&2
  rm -f "$SETTINGS_FILE.tmp"
  exit 2
fi

# If the file is now empty (just {}), remove it entirely.
REMAINING=$(jq 'keys | length' "$SETTINGS_FILE.tmp")
if [ "$REMAINING" -eq 0 ]; then
  rm -f "$SETTINGS_FILE.tmp" "$SETTINGS_FILE"
else
  mv "$SETTINGS_FILE.tmp" "$SETTINGS_FILE"
fi
