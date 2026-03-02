#!/usr/bin/env bash
# Completion guard — blocks TaskUpdate with status=completed unless phase is Complete.
# Prevents teammates from finishing a quest without going through all gates.
# Hook: PreToolUse | Matcher: TaskUpdate

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)
if ! echo "$INPUT" | jq empty 2>/dev/null; then
  echo "fellowship: malformed hook input — blocking for safety" >&2
  exit 2
fi

# Only act on completion updates.
STATUS=$(echo "$INPUT" | jq -r '.tool_input.status // empty')
verbose "completion-guard: status=$STATUS"
if [ "$STATUS" != "completed" ]; then
  verbose "completion-guard: not a completion update, passing through"
  exit 0
fi

# Block completion unless phase is Complete.
PHASE=$(echo "$STATE" | jq -r '.phase')
verbose "completion-guard: phase=$PHASE"
if [ "$PHASE" != "Complete" ]; then
  verbose "completion-guard: BLOCKED — phase is $PHASE, not Complete"
  echo "Cannot complete task — current phase is '$PHASE'. You must submit gates for all phases (Onboard → Research → Plan → Implement → Review → Complete) before completing." >&2
  exit 2
fi

verbose "completion-guard: allowed"
exit 0
