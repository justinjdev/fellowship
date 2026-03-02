#!/usr/bin/env bash
# Gate guard — blocks work tools when a gate is pending, and blocks
# file modifications to production code during early phases.
# Hook: PreToolUse | Matcher: Edit|Write|Bash|Agent|Skill|NotebookEdit

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)

GATE_PENDING=$(echo "$STATE" | jq -r '.gate_pending')
PHASE=$(echo "$STATE" | jq -r '.phase')

# Hard block: gate is pending approval.
if [ "$GATE_PENDING" = "true" ]; then
  echo "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate." >&2
  exit 2
fi

# Phase-aware guard: block file modifications during early phases.
# During Onboard/Research/Plan, only tmp/ writes are allowed (state file,
# checkpoints, notes). Production code changes require Implement phase.
case "$PHASE" in
  Onboard|Research|Plan)
    # Extract file path from Edit, Write, or NotebookEdit tool input.
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.notebook_path // empty' 2>/dev/null)

    if [ -n "$FILE_PATH" ]; then
      case "$FILE_PATH" in
        */tmp/*|tmp/*) ;; # allow writes to tmp/ (state file, checkpoints, notes)
        *)
          echo "Phase '$PHASE' does not allow file modifications outside tmp/. Advance to Implement by submitting gates for each phase." >&2
          exit 2
          ;;
      esac
    fi
    ;;
esac

exit 0
