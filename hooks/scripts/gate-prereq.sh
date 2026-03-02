#!/usr/bin/env bash
# Gate prereq — tracks /lembas invocation.
# Hook: PostToolUse | Matcher: Skill

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)
if ! echo "$INPUT" | jq empty 2>/dev/null; then
  echo "fellowship: malformed hook input" >&2
  exit 2
fi
SKILL=$(echo "$INPUT" | jq -r '.tool_input.skill // empty')

# Only act on lembas invocations (match any plugin namespace).
case "$SKILL" in
  *lembas*) ;;
  *) exit 0 ;;
esac

# Set lembas_completed = true.
if ! echo "$STATE" | jq '.lembas_completed = true' > "$STATE_FILE.tmp"; then
  echo "fellowship: failed to update state file" >&2
  rm -f "$STATE_FILE.tmp"
  exit 2
fi
mv "$STATE_FILE.tmp" "$STATE_FILE"

exit 0
