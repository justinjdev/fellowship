#!/usr/bin/env bash
# Metadata track — detects phase metadata updates on TaskUpdate.
# Hook: PostToolUse | Matcher: TaskUpdate

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)
if ! echo "$INPUT" | jq empty 2>/dev/null; then
  echo "fellowship: malformed hook input" >&2
  exit 2
fi

# Check if tool_input contains metadata.phase.
HAS_PHASE=$(echo "$INPUT" | jq -r '.tool_input.metadata.phase // empty')
verbose "metadata-track: phase metadata=$HAS_PHASE"

if [ -z "$HAS_PHASE" ]; then
  verbose "metadata-track: no phase metadata, skipping"
  exit 0
fi

# Set metadata_updated = true.
verbose "metadata-track: setting metadata_updated=true"
if ! echo "$STATE" | jq '.metadata_updated = true' > "$STATE_FILE.tmp"; then
  echo "fellowship: failed to update state file" >&2
  rm -f "$STATE_FILE.tmp"
  exit 2
fi
mv "$STATE_FILE.tmp" "$STATE_FILE"

exit 0
