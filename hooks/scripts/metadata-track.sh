#!/usr/bin/env bash
# Metadata track — detects phase metadata updates on TaskUpdate.
# Hook: PostToolUse | Matcher: TaskUpdate

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)

# Check if tool_input contains metadata.phase.
HAS_PHASE=$(echo "$INPUT" | jq -r '.tool_input.metadata.phase // empty')

if [ -z "$HAS_PHASE" ]; then
  exit 0
fi

# Set metadata_updated = true.
echo "$STATE" | jq '.metadata_updated = true' > "$STATE_FILE.tmp" && mv "$STATE_FILE.tmp" "$STATE_FILE"

exit 0
