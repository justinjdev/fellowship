## 1. Plugin Hook Infrastructure

- [x] 1.1 Create `hooks/hooks.json` with all four hook definitions (gate-guard, gate-submit, gate-prereq, metadata-track) using `${CLAUDE_PLUGIN_ROOT}/hooks/scripts/` paths
- [x] 1.2 Create `hooks/scripts/` directory and a shared `_common.sh` sourced by all scripts — contains state file path resolution, `jq` check, state file existence check, and early exit logic

## 2. Gate Guard Hook (PreToolUse)

- [x] 2.1 Write `hooks/scripts/gate-guard.sh` — reads state file, exits 2 if `gate_pending` is `true`, exits 0 otherwise. Matcher: `Edit|Write|Bash|Agent|Skill|NotebookEdit`
- [x] 2.2 Verify gate-guard does not block Read, Grep, Glob, or other read-only tools

## 3. Gate Submit Hook (PreToolUse on SendMessage)

- [x] 3.1 Write `hooks/scripts/gate-submit.sh` — reads stdin for tool_input, extracts message content, applies gate pattern matching (phase keyword + checklist markers)
- [x] 3.2 Implement prerequisite check: if gate message detected, verify `lembas_completed` and `metadata_updated` are both `true`; exit 2 with specific missing prereqs if not
- [x] 3.3 Implement gate-pending guard: if `gate_pending` is already `true`, exit 2 ("gate already pending")
- [x] 3.4 Implement auto-approve logic: if current phase gate is in `auto_approve_gates`, advance phase and reset prereqs without setting `gate_pending`
- [x] 3.5 Implement normal gate flow: set `gate_pending` to `true`, generate `gate_id`, write updated state

## 4. Prerequisite Tracking Hooks (PostToolUse)

- [x] 4.1 Write `hooks/scripts/gate-prereq.sh` — PostToolUse on Skill, detects lembas invocation from tool_input, sets `lembas_completed` to `true`
- [x] 4.2 Write `hooks/scripts/metadata-track.sh` — PostToolUse on TaskUpdate, detects `metadata.phase` in tool_input, sets `metadata_updated` to `true`

## 5. Quest Skill Integration

- [x] 5.1 Add state file initialization to quest SKILL.md Phase 0 — create `tmp/quest-state.json` with schema fields after worktree setup, populate `auto_approve_gates` from config
- [x] 5.2 Add `TaskUpdate` with `metadata: {"worktree_path": "<cwd>"}` to Phase 0 after worktree creation
- [x] 5.3 Add respawn handling: if `tmp/quest-state.json` already exists at Phase 0, reset `gate_pending` to `false` and preserve existing `phase`

## 6. Fellowship Skill Integration

- [x] 6.1 Rewrite spawn prompt gate handling section — document state machine behavior, explain hook enforcement, remove ambiguous "STOP" language
- [x] 6.2 Add gate approval procedure to the Gate Handling section — read `worktree_path` from task metadata, construct `jq` command to update state file, execute, then send approval message
- [x] 6.3 Update gate config override template — default to strictest language, only relax for explicitly auto-approved gates
- [x] 6.4 Add Gandalf gate rejection procedure — send rejection message without modifying state file, teammate stays blocked

## 7. Validation

- [ ] 7.1 Manual test: run a single quest as fellowship teammate, verify state file created at Phase 0, verify hooks block after gate submission, verify Gandalf approval unblocks
- [ ] 7.2 Manual test: verify non-fellowship sessions are unaffected (no state file → all hooks no-op)
- [ ] 7.3 Manual test: verify auto-approved gates advance state without blocking
- [ ] 7.4 Manual test: verify missing `jq` degrades gracefully (hooks exit 0 with warning)
