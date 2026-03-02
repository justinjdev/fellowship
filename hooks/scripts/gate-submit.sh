#!/usr/bin/env bash
# Gate submit — detects gate messages on SendMessage, verifies prerequisites,
# and sets gate_pending=true for non-auto-approved gates.
# Hook: PreToolUse | Matcher: SendMessage

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/_common.sh"

# Read tool input from stdin.
INPUT=$(cat)
CONTENT=$(echo "$INPUT" | jq -r '.tool_input.content // empty')

# Gate detection: message must start with [GATE] marker.
GATE_MARKER=$(echo "$CONTENT" | grep -m1 '^\[GATE\]' || true)

# Not a gate message — allow through.
if [ -z "$GATE_MARKER" ]; then
  exit 0
fi

# --- This is a gate message ---

GATE_PENDING=$(echo "$STATE" | jq -r '.gate_pending')
LEMBAS=$(echo "$STATE" | jq -r '.lembas_completed')
METADATA=$(echo "$STATE" | jq -r '.metadata_updated')
PHASE=$(echo "$STATE" | jq -r '.phase')
AUTO_GATES=$(echo "$STATE" | jq -r '.auto_approve_gates // [] | .[]')

# Block if a gate is already pending.
if [ "$GATE_PENDING" = "true" ]; then
  echo "Gate already pending — wait for lead approval before submitting another gate." >&2
  exit 2
fi

# Check prerequisites.
MISSING=""
if [ "$LEMBAS" != "true" ]; then
  MISSING="lembas not completed"
fi
if [ "$METADATA" != "true" ]; then
  if [ -n "$MISSING" ]; then
    MISSING="$MISSING, metadata not updated"
  else
    MISSING="metadata not updated"
  fi
fi

if [ -n "$MISSING" ]; then
  echo "Gate blocked: $MISSING. Run /lembas and update task metadata before submitting a gate." >&2
  exit 2
fi

# Determine next phase.
case "$PHASE" in
  Onboard)   NEXT_PHASE="Research" ;;
  Research)  NEXT_PHASE="Plan" ;;
  Plan)      NEXT_PHASE="Implement" ;;
  Implement) NEXT_PHASE="Review" ;;
  Review)    NEXT_PHASE="Complete" ;;
  Complete)
    echo "Quest already complete — no further gates to submit." >&2
    exit 2
    ;;
  *)
    echo "fellowship: unknown phase '$PHASE' in state file — cannot submit gate" >&2
    exit 2
    ;;
esac

# Check if this gate is auto-approved.
IS_AUTO="false"
for gate in $AUTO_GATES; do
  if [ "$gate" = "$NEXT_PHASE" ]; then
    IS_AUTO="true"
    break
  fi
done

if [ "$IS_AUTO" = "true" ]; then
  # Auto-approved: advance phase, reset prereqs, don't set gate_pending.
  if ! echo "$STATE" | jq \
    --arg phase "$NEXT_PHASE" \
    '.phase = $phase | .lembas_completed = false | .metadata_updated = false' \
    > "$STATE_FILE.tmp"; then
    echo "fellowship: failed to update state file during auto-approve" >&2
    rm -f "$STATE_FILE.tmp"
    exit 2
  fi
  mv "$STATE_FILE.tmp" "$STATE_FILE"
  exit 0
fi

# Normal gate: set gate_pending, generate gate_id.
GATE_ID="gate-${PHASE}-$(date +%s)"
if ! echo "$STATE" | jq \
  --arg gid "$GATE_ID" \
  '.gate_pending = true | .gate_id = $gid' \
  > "$STATE_FILE.tmp"; then
  echo "fellowship: failed to update state file during gate submission" >&2
  rm -f "$STATE_FILE.tmp"
  exit 2
fi
mv "$STATE_FILE.tmp" "$STATE_FILE"

exit 0
