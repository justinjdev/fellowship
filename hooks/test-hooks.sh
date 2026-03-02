#!/usr/bin/env bash
# Test suite for fellowship gate hooks.
# Exercises each hook script with simulated inputs and validates
# exit codes + state file mutations.
#
# Usage: ./hooks/test-hooks.sh
# Requires: jq

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOOKS="$SCRIPT_DIR/scripts"

# Work in a temp directory so we don't pollute the repo.
WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

STATE_FILE="$WORK_DIR/tmp/quest-state.json"
mkdir -p "$WORK_DIR/tmp"

PASS=0
FAIL=0

# --- helpers ---

reset_state() {
  cat > "$STATE_FILE" << 'EOF'
{
  "version": 1,
  "quest_name": "test",
  "task_id": "1",
  "team_name": "test-team",
  "phase": "Research",
  "gate_pending": false,
  "gate_id": null,
  "lembas_completed": false,
  "metadata_updated": false,
  "auto_approve_gates": []
}
EOF
}

state_val() {
  jq -r "$1" "$STATE_FILE"
}

set_state() {
  jq "$1" "$STATE_FILE" > "$STATE_FILE.tmp" && mv "$STATE_FILE.tmp" "$STATE_FILE"
}

# Run a hook script from the work directory (so tmp/quest-state.json resolves).
# Captures exit code into $rc without letting the shell bail.
run_hook() {
  local script="$1"
  shift
  rc=0
  (cd "$WORK_DIR" && "$HOOKS/$script" "$@") 2>/dev/null || rc=$?
}

# Run a hook script with stdin input from the work directory.
run_hook_stdin() {
  local script="$1"
  local input="$2"
  rc=0
  (cd "$WORK_DIR" && echo "$input" | "$HOOKS/$script") 2>/dev/null || rc=$?
}

assert_exit() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [ "$actual" -eq "$expected" ]; then
    echo "  PASS: $name (exit $actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name — expected exit $expected, got $actual"
    FAIL=$((FAIL + 1))
  fi
}

# Run a hook with FELLOWSHIP_VERBOSE=1 and capture stderr.
# Sets $rc (exit code) and $stderr (captured output).
run_hook_verbose() {
  local script="$1"
  shift
  rc=0
  stderr=$(cd "$WORK_DIR" && FELLOWSHIP_VERBOSE=1 "$HOOKS/$script" "$@" 2>&1 >/dev/null) || rc=$?
}

# Run a hook with stdin input and FELLOWSHIP_VERBOSE=1, capturing stderr.
run_hook_stdin_verbose() {
  local script="$1"
  local input="$2"
  rc=0
  stderr=$(cd "$WORK_DIR" && echo "$input" | FELLOWSHIP_VERBOSE=1 "$HOOKS/$script" 2>&1 >/dev/null) || rc=$?
}

assert_stderr_contains() {
  local name="$1"
  local expected="$2"
  if echo "$stderr" | grep -qF "$expected"; then
    echo "  PASS: $name (stderr contains '$expected')"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name — stderr does not contain '$expected'"
    echo "        stderr was: $stderr"
    FAIL=$((FAIL + 1))
  fi
}

assert_stderr_empty() {
  local name="$1"
  if [ -z "$stderr" ]; then
    echo "  PASS: $name (stderr empty)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name — expected empty stderr, got: $stderr"
    FAIL=$((FAIL + 1))
  fi
}

assert_val() {
  local name="$1"
  local jq_path="$2"
  local expected="$3"
  local actual
  actual=$(state_val "$jq_path")
  if [ "$actual" = "$expected" ]; then
    echo "  PASS: $name ($jq_path = $actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name — expected $jq_path = $expected, got $actual"
    FAIL=$((FAIL + 1))
  fi
}

# --- gate-guard.sh ---

echo ""
echo "=== gate-guard ==="

echo "-- allows when gate_pending=false"
reset_state
run_hook gate-guard.sh
assert_exit "allow when not pending" 0 "$rc"

echo "-- blocks when gate_pending=true"
set_state '.gate_pending = true'
run_hook gate-guard.sh
assert_exit "block when pending" 2 "$rc"

echo "-- no-op when state file missing"
rm "$STATE_FILE"
run_hook gate-guard.sh
assert_exit "no-op without state file" 0 "$rc"

# --- gate-prereq.sh ---

echo ""
echo "=== gate-prereq ==="

echo "-- sets lembas_completed on lembas skill"
reset_state
run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"lembas"}}'
assert_val "lembas sets flag" '.lembas_completed' "true"

echo "-- sets lembas_completed on fellowship:lembas"
reset_state
run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"fellowship:lembas"}}'
assert_val "fellowship:lembas sets flag" '.lembas_completed' "true"

echo "-- ignores other skills"
reset_state
run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"council"}}'
assert_val "other skill no-op" '.lembas_completed' "false"

echo "-- matches lembas substring (alternate namespace)"
reset_state
run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"my-plugin:lembas"}}'
assert_val "alternate namespace lembas sets flag" '.lembas_completed' "true"

echo "-- rejects malformed input"
reset_state
run_hook_stdin gate-prereq.sh 'not valid json'
assert_exit "malformed input rejected" 2 "$rc"

echo "-- no-op when state file missing"
rm "$STATE_FILE"
run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"lembas"}}'
assert_exit "no-op without state file" 0 "$rc"

# --- metadata-track.sh ---

echo ""
echo "=== metadata-track ==="

echo "-- sets metadata_updated on phase metadata"
reset_state
run_hook_stdin metadata-track.sh '{"tool_input":{"metadata":{"phase":"Research"}}}'
assert_val "phase metadata sets flag" '.metadata_updated' "true"

echo "-- ignores non-phase metadata"
reset_state
run_hook_stdin metadata-track.sh '{"tool_input":{"metadata":{"status":"in_progress"}}}'
assert_val "non-phase metadata no-op" '.metadata_updated' "false"

echo "-- ignores updates without metadata"
reset_state
run_hook_stdin metadata-track.sh '{"tool_input":{"status":"completed"}}'
assert_val "no metadata no-op" '.metadata_updated' "false"

echo "-- rejects malformed input"
reset_state
run_hook_stdin metadata-track.sh 'not valid json'
assert_exit "malformed input rejected" 2 "$rc"

# --- gate-submit.sh ---

echo ""
echo "=== gate-submit ==="

GATE_MSG='{"tool_input":{"content":"[GATE] Research complete\n- [x] findings documented"}}'
NORMAL_MSG='{"tool_input":{"content":"Here is a status update on my progress"}}'
PHASE_MSG_NO_GATE='{"tool_input":{"content":"Research complete\n- [x] findings documented"}}'
MULTI_GATE_MSG='{"tool_input":{"content":"[GATE] Research complete\n- [x] done\n[GATE] Plan complete\n- [x] also done"}}'

echo "-- allows non-gate messages through"
reset_state
run_hook_stdin gate-submit.sh "$NORMAL_MSG"
assert_exit "non-gate message allowed" 0 "$rc"
assert_val "state unchanged" '.gate_pending' "false"

echo "-- rejects malformed input"
reset_state
run_hook_stdin gate-submit.sh 'not valid json'
assert_exit "malformed input rejected" 2 "$rc"

echo "-- rejects multiple [GATE] markers in one message"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true'
run_hook_stdin gate-submit.sh "$MULTI_GATE_MSG"
assert_exit "multi-gate rejected" 2 "$rc"
assert_val "gate_pending still false after multi-gate" '.gate_pending' "false"

echo "-- allows phase+checklist messages without [GATE] prefix"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true'
run_hook_stdin gate-submit.sh "$PHASE_MSG_NO_GATE"
assert_exit "no [GATE] prefix — not a gate" 0 "$rc"
assert_val "gate_pending still false" '.gate_pending' "false"

echo "-- blocks gate message when lembas missing"
reset_state
set_state '.metadata_updated = true'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "blocks without lembas" 2 "$rc"
assert_val "gate_pending still false" '.gate_pending' "false"

echo "-- blocks gate message when metadata missing"
reset_state
set_state '.lembas_completed = true'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "blocks without metadata" 2 "$rc"

echo "-- blocks gate message when both prereqs missing"
reset_state
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "blocks without both prereqs" 2 "$rc"

echo "-- allows gate message and sets pending when prereqs met"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "allows with prereqs" 0 "$rc"
assert_val "gate_pending set" '.gate_pending' "true"
# gate_id should be non-null
GATE_ID=$(state_val '.gate_id')
if [ "$GATE_ID" != "null" ] && [ -n "$GATE_ID" ]; then
  echo "  PASS: gate_id generated ($GATE_ID)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: gate_id should be non-null, got $GATE_ID"
  FAIL=$((FAIL + 1))
fi

echo "-- blocks duplicate gate submission"
# gate_pending is already true from the previous test
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "blocks duplicate gate" 2 "$rc"

echo "-- auto-approves configured gates"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true | .auto_approve_gates = ["Plan"]'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "auto-approve exits 0" 0 "$rc"
assert_val "phase advanced to Plan" '.phase' "Plan"
assert_val "gate_pending stays false" '.gate_pending' "false"
assert_val "lembas reset after auto-approve" '.lembas_completed' "false"
assert_val "metadata reset after auto-approve" '.metadata_updated' "false"

echo "-- does not auto-approve unlisted gates"
reset_state
set_state '.phase = "Plan" | .lembas_completed = true | .metadata_updated = true | .auto_approve_gates = ["Research"]'
PLAN_GATE_MSG='{"tool_input":{"content":"[GATE] Plan complete\n- [x] plan reviewed"}}'
run_hook_stdin gate-submit.sh "$PLAN_GATE_MSG"
assert_exit "non-auto gate exits 0" 0 "$rc"
assert_val "gate_pending set for non-auto gate" '.gate_pending' "true"
assert_val "phase unchanged" '.phase' "Plan"

echo "-- does not auto-approve gate FROM the listed phase"
reset_state
set_state '.phase = "Plan" | .lembas_completed = true | .metadata_updated = true | .auto_approve_gates = ["Plan"]'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "not auto-approved by current phase" 0 "$rc"
assert_val "gate_pending set (not auto-approved)" '.gate_pending' "true"
assert_val "phase stays Plan" '.phase' "Plan"

echo "-- blocks gate at Complete phase"
reset_state
set_state '.phase = "Complete" | .lembas_completed = true | .metadata_updated = true'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "Complete phase blocks gate" 2 "$rc"

echo "-- blocks gate at unknown phase"
reset_state
set_state '.phase = "InvalidPhase" | .lembas_completed = true | .metadata_updated = true'
run_hook_stdin gate-submit.sh "$GATE_MSG"
assert_exit "unknown phase blocks gate" 2 "$rc"

# --- completion-guard.sh ---

echo ""
echo "=== completion-guard ==="

COMPLETE_MSG='{"tool_input":{"taskId":"1","status":"completed"}}'
PROGRESS_MSG='{"tool_input":{"taskId":"1","status":"in_progress"}}'
METADATA_MSG='{"tool_input":{"taskId":"1","metadata":{"phase":"Research"}}}'

echo "-- allows non-completion updates"
reset_state
run_hook_stdin completion-guard.sh "$PROGRESS_MSG"
assert_exit "in_progress allowed" 0 "$rc"

echo "-- allows metadata-only updates"
reset_state
run_hook_stdin completion-guard.sh "$METADATA_MSG"
assert_exit "metadata update allowed" 0 "$rc"

echo "-- blocks completion when phase is Onboard"
reset_state
set_state '.phase = "Onboard"'
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "blocks at Onboard" 2 "$rc"

echo "-- blocks completion when phase is Research"
reset_state
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "blocks at Research" 2 "$rc"

echo "-- blocks completion when phase is Plan"
reset_state
set_state '.phase = "Plan"'
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "blocks at Plan" 2 "$rc"

echo "-- blocks completion when phase is Implement"
reset_state
set_state '.phase = "Implement"'
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "blocks at Implement" 2 "$rc"

echo "-- blocks completion when phase is Review"
reset_state
set_state '.phase = "Review"'
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "blocks at Review" 2 "$rc"

echo "-- allows completion when phase is Complete"
reset_state
set_state '.phase = "Complete"'
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "allows at Complete" 0 "$rc"

echo "-- rejects malformed input"
reset_state
run_hook_stdin completion-guard.sh 'not valid json'
assert_exit "malformed input rejected" 2 "$rc"

echo "-- no-op when state file missing"
rm "$STATE_FILE"
run_hook_stdin completion-guard.sh "$COMPLETE_MSG"
assert_exit "no-op without state file" 0 "$rc"

# --- phase transitions ---

echo ""
echo "=== phase transitions ==="

for phase_pair in "Onboard:Research" "Research:Plan" "Plan:Implement" "Implement:Review" "Review:Complete"; do
  FROM="${phase_pair%%:*}"
  TO="${phase_pair##*:}"
  reset_state
  set_state "$(printf '.phase = "%s" | .lembas_completed = true | .metadata_updated = true | .auto_approve_gates = ["%s"]' "$FROM" "$TO")"
  TRANSITION_MSG='{"tool_input":{"content":"[GATE] Phase complete\n- [x] done"}}'
  run_hook_stdin gate-submit.sh "$TRANSITION_MSG"
  assert_exit "$FROM -> $TO exits 0" 0 "$rc"
  assert_val "$FROM -> $TO advances phase" '.phase' "$TO"
done

# --- full lifecycle ---

echo ""
echo "=== full lifecycle (manual gates) ==="

reset_state
set_state '.phase = "Onboard"'

# Gate messages must contain a phase keyword + checklist to trigger detection.
# Use the TO phase name so even Onboard gates are detected.
for phase_pair in "Onboard:Research" "Research:Plan" "Plan:Implement" "Implement:Review" "Review:Complete"; do
  FROM="${phase_pair%%:*}"
  TO="${phase_pair##*:}"
  LIFECYCLE_MSG='{"tool_input":{"content":"[GATE] '"$TO"' gate\n- [x] checklist complete"}}'

  # 1. prereqs not met — gate blocked
  run_hook_stdin gate-submit.sh "$LIFECYCLE_MSG"
  assert_exit "lifecycle $FROM: blocked without prereqs" 2 "$rc"

  # 2. complete prereqs
  run_hook_stdin gate-prereq.sh '{"tool_input":{"skill":"lembas"}}'
  run_hook_stdin metadata-track.sh '{"tool_input":{"metadata":{"phase":"'"$FROM"'"}}}'

  # 3. submit gate — should succeed and set pending
  run_hook_stdin gate-submit.sh "$LIFECYCLE_MSG"
  assert_exit "lifecycle $FROM: gate accepted" 0 "$rc"
  assert_val "lifecycle $FROM: gate_pending" '.gate_pending' "true"

  # 4. tools blocked while pending
  run_hook gate-guard.sh
  assert_exit "lifecycle $FROM: tools blocked" 2 "$rc"

  # 5. simulate lead approval (write to state file directly)
  set_state "$(printf '.gate_pending = false | .phase = "%s" | .gate_id = null | .lembas_completed = false | .metadata_updated = false' "$TO")"

  # 6. tools unblocked after approval
  run_hook gate-guard.sh
  assert_exit "lifecycle $TO: tools unblocked" 0 "$rc"
done

assert_val "lifecycle ends at Complete" '.phase' "Complete"

# --- verbose output (FELLOWSHIP_VERBOSE) ---

echo ""
echo "=== verbose output ==="

echo "-- gate-guard produces verbose output when FELLOWSHIP_VERBOSE=1"
reset_state
run_hook_verbose gate-guard.sh
assert_exit "verbose gate-guard exits 0" 0 "$rc"
assert_stderr_contains "verbose gate-guard logs allowed" "gate-guard: allowed"

echo "-- gate-guard silent when FELLOWSHIP_VERBOSE unset"
reset_state
stderr=""
rc=0
stderr=$(cd "$WORK_DIR" && "$HOOKS/gate-guard.sh" 2>&1 >/dev/null) || rc=$?
assert_exit "silent gate-guard exits 0" 0 "$rc"
assert_stderr_empty "no verbose output without env var"

echo "-- gate-submit verbose on gate detection"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true'
GATE_MSG='{"tool_input":{"content":"[GATE] Research complete\n- [x] findings documented"}}'
run_hook_stdin_verbose gate-submit.sh "$GATE_MSG"
assert_exit "verbose gate-submit exits 0" 0 "$rc"
assert_stderr_contains "verbose gate-submit logs marker" "gate-submit: [GATE] marker detected"
assert_stderr_contains "verbose gate-submit logs phase" "gate-submit: phase=Research"
assert_stderr_contains "verbose gate-submit logs pending" "gate-submit: normal gate, setting pending"

echo "-- gate-submit verbose on auto-approve"
reset_state
set_state '.lembas_completed = true | .metadata_updated = true | .auto_approve_gates = ["Plan"]'
run_hook_stdin_verbose gate-submit.sh "$GATE_MSG"
assert_exit "verbose auto-approve exits 0" 0 "$rc"
assert_stderr_contains "verbose gate-submit logs auto-approve" "gate-submit: auto-approved Plan"

echo "-- gate-prereq verbose on lembas match"
reset_state
run_hook_stdin_verbose gate-prereq.sh '{"tool_input":{"skill":"lembas"}}'
assert_exit "verbose gate-prereq exits 0" 0 "$rc"
assert_stderr_contains "verbose gate-prereq logs lembas" "gate-prereq: lembas detected"

echo "-- gate-prereq verbose on non-lembas skip"
reset_state
run_hook_stdin_verbose gate-prereq.sh '{"tool_input":{"skill":"council"}}'
assert_exit "verbose gate-prereq skip exits 0" 0 "$rc"
assert_stderr_contains "verbose gate-prereq logs skip" "gate-prereq: not lembas, skipping"

echo "-- completion-guard verbose on allowed completion"
reset_state
set_state '.phase = "Complete"'
run_hook_stdin_verbose completion-guard.sh '{"tool_input":{"taskId":"1","status":"completed"}}'
assert_exit "verbose completion-guard exits 0" 0 "$rc"
assert_stderr_contains "verbose completion-guard logs allowed" "completion-guard: allowed"

echo "-- completion-guard verbose on blocked completion"
reset_state
run_hook_stdin_verbose completion-guard.sh '{"tool_input":{"taskId":"1","status":"completed"}}'
assert_exit "verbose completion-guard blocks" 2 "$rc"
assert_stderr_contains "verbose completion-guard logs blocked" "completion-guard: BLOCKED"

echo "-- metadata-track verbose on phase metadata"
reset_state
run_hook_stdin_verbose metadata-track.sh '{"tool_input":{"metadata":{"phase":"Research"}}}'
assert_exit "verbose metadata-track exits 0" 0 "$rc"
assert_stderr_contains "verbose metadata-track logs update" "metadata-track: setting metadata_updated=true"

echo "-- metadata-track verbose on skip"
reset_state
run_hook_stdin_verbose metadata-track.sh '{"tool_input":{"status":"in_progress"}}'
assert_exit "verbose metadata-track skip exits 0" 0 "$rc"
assert_stderr_contains "verbose metadata-track logs skip" "metadata-track: no phase metadata, skipping"

echo "-- common verbose logs state file path"
reset_state
run_hook_verbose gate-guard.sh
assert_stderr_contains "verbose common logs state file" "state file:"

echo "-- common verbose logs phase from state"
reset_state
run_hook_verbose gate-guard.sh
assert_stderr_contains "verbose common logs phase" "state loaded (phase=Research)"

# --- summary ---

echo ""
echo "================================"
TOTAL=$((PASS + FAIL))
echo "Results: $PASS/$TOTAL passed, $FAIL failed"
if [ "$FAIL" -gt 0 ]; then
  echo "FAILED"
  exit 1
else
  echo "ALL PASSED"
  exit 0
fi
