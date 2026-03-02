## ADDED Requirements

### Requirement: Worktree path in task metadata
The quest skill SHALL store the teammate's worktree absolute path in task metadata as `metadata.worktree_path` during Phase 0 (Onboard), immediately after worktree creation. This enables Gandalf to locate the teammate's state file for gate approval.

#### Scenario: Worktree path stored after creation
- **WHEN** the quest skill creates or enters a worktree during Phase 0
- **THEN** it SHALL call `TaskUpdate` with `metadata: {"worktree_path": "<absolute_path>"}` where `<absolute_path>` is the worktree root directory

#### Scenario: Gandalf reads worktree path
- **WHEN** Gandalf needs to approve a gate for a teammate
- **THEN** Gandalf SHALL call `TaskGet` for the teammate's task and read `metadata.worktree_path` to locate the state file

### Requirement: Gate approval via state file write
Gandalf SHALL approve gates by writing directly to the teammate's `tmp/quest-state.json` file. The approval write SHALL set `gate_pending` to `false`, advance `phase` to the next value in sequence, set `gate_id` to `null`, and reset `lembas_completed` and `metadata_updated` to `false`.

#### Scenario: Gandalf approves a Research gate
- **WHEN** Gandalf approves quest-auth-bug's Research gate and the teammate's worktree is at `/path/to/worktree`
- **THEN** Gandalf SHALL execute a command that updates `/path/to/worktree/tmp/quest-state.json` setting `gate_pending` to `false`, `phase` to `"Plan"`, `gate_id` to `null`, `lembas_completed` to `false`, `metadata_updated` to `false`

#### Scenario: Gandalf approves a Review gate
- **WHEN** Gandalf approves a teammate's Review gate
- **THEN** the state file SHALL be updated with `phase` set to `"Complete"` (the next phase after Review)

### Requirement: Approval unblocks teammate
After Gandalf writes the approval to the state file, the teammate's next tool call SHALL succeed because the gate-guard hook reads `gate_pending` as `false`. No additional message or signal is required beyond the state file update — the hook reads the file on every tool call.

#### Scenario: Teammate resumes after approval
- **WHEN** Gandalf has written `gate_pending: false` to the teammate's state file and the teammate's next tool call fires
- **THEN** the gate-guard hook SHALL read the updated state, find `gate_pending` is `false`, and exit 0 (allow)

#### Scenario: Teammate blocked before approval
- **WHEN** the teammate attempts a tool call before Gandalf has updated the state file
- **THEN** the gate-guard hook SHALL find `gate_pending` is `true` and exit 2 (block)

### Requirement: Fellowship skill spawn prompt update
The fellowship skill's spawn prompt SHALL document the state machine behavior so teammates understand why their tools are being blocked. The prompt SHALL explain that gates are enforced by hooks, that lembas and metadata are prerequisites, and that only the lead can approve gates.

#### Scenario: Spawn prompt includes state machine explanation
- **WHEN** Gandalf spawns a quest teammate
- **THEN** the spawn prompt SHALL include language explaining: hooks enforce gates, tools are blocked after gate submission, only the lead can unblock by approving, lembas and metadata MUST be completed before gate submission

### Requirement: Fellowship skill gate approval procedure
The fellowship skill SHALL include an explicit procedure for Gandalf to follow when approving gates. This procedure SHALL include: reading the teammate's worktree path from task metadata, constructing the `jq` command to update the state file, executing the command, then sending the approval message to the teammate via SendMessage.

#### Scenario: Gandalf follows approval procedure
- **WHEN** Gandalf receives a gate message from a teammate and decides to approve
- **THEN** Gandalf SHALL: (1) read `worktree_path` from task metadata, (2) update the state file at that path, (3) send approval message to the teammate

#### Scenario: Gandalf rejects a gate
- **WHEN** Gandalf receives a gate message and decides to reject (or the user rejects)
- **THEN** Gandalf SHALL clear `gate_pending` to `false` in the state file (without advancing the phase) so the teammate can address feedback, then send a rejection message via SendMessage

### Requirement: Auto-approved gate handling by Gandalf
When a gate is auto-approved per config, Gandalf SHALL still write to the teammate's state file to advance the phase. The `gate-submit` hook handles auto-approved gates by advancing state without setting `gate_pending`, so Gandalf's write is only needed for non-auto-approved gates. For auto-approved gates, Gandalf SHALL log the auto-approval but does NOT need to write to the state file.

#### Scenario: Auto-approved gate flow
- **WHEN** a teammate submits a gate for a phase listed in `auto_approve_gates`
- **THEN** the `gate-submit` hook advances the state automatically, and Gandalf logs "quest-X: Research gate auto-approved per config" without writing to the state file

#### Scenario: Non-auto-approved gate flow
- **WHEN** a teammate submits a gate for a phase NOT in `auto_approve_gates`
- **THEN** the `gate-submit` hook sets `gate_pending` to `true`, and Gandalf MUST write to the state file to unblock the teammate
