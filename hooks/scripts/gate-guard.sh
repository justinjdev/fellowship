#!/usr/bin/env bash
# Gate guard — blocks work tools when a gate is pending approval.
# Hook: PreToolUse | Matcher: Edit|Write|Bash|Agent|Skill|NotebookEdit

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

GATE_PENDING=$(echo "$STATE" | jq -r '.gate_pending')
verbose "gate-guard: gate_pending=$GATE_PENDING"

if [ "$GATE_PENDING" = "true" ]; then
  verbose "gate-guard: BLOCKED — gate pending"
  echo "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate." >&2
  exit 2
fi

verbose "gate-guard: allowed"
exit 0
