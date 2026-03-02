## ADDED Requirements

### Requirement: State file schema
The system SHALL maintain a JSON state file at `tmp/quest-state.json` in each quest teammate's worktree with the following fields: `version` (integer), `quest_name` (string), `task_id` (string), `team_name` (string), `phase` (enum string), `gate_pending` (boolean), `gate_id` (string or null), `lembas_completed` (boolean), `metadata_updated` (boolean), `auto_approve_gates` (array of strings).

#### Scenario: Valid state file
- **WHEN** a state file exists at `tmp/quest-state.json`
- **THEN** it SHALL be valid JSON conforming to the schema with all required fields present

#### Scenario: Phase enum values
- **WHEN** the `phase` field is read
- **THEN** its value SHALL be one of: `"Onboard"`, `"Research"`, `"Plan"`, `"Implement"`, `"Review"`, `"Complete"`

### Requirement: State file initialization
The quest skill SHALL create `tmp/quest-state.json` during Phase 0 (Onboard) after worktree setup. The initial state SHALL set `phase` to `"Onboard"`, `gate_pending` to `false`, `gate_id` to `null`, `lembas_completed` to `false`, `metadata_updated` to `false`. The `auto_approve_gates` field SHALL be populated from `config.gates.autoApprove` (defaulting to empty array).

#### Scenario: First quest run in fresh worktree
- **WHEN** the quest skill enters Phase 0 and no `tmp/quest-state.json` exists
- **THEN** the skill SHALL create the file with initial values and `phase` set to `"Onboard"`

#### Scenario: Respawn into existing worktree
- **WHEN** the quest skill enters Phase 0 and `tmp/quest-state.json` already exists
- **THEN** the skill SHALL reset `gate_pending` to `false` and preserve the existing `phase` value

#### Scenario: Auto-approve config propagation
- **WHEN** `~/.claude/fellowship.json` contains `gates.autoApprove: ["Research", "Plan"]`
- **THEN** the state file SHALL contain `auto_approve_gates: ["Research", "Plan"]`

### Requirement: Phase transitions
Phase transitions SHALL follow a strict linear order: Onboard → Research → Plan → Implement → Review → Complete. The state file's `phase` field SHALL only advance forward in this sequence, never backward (except during recovery, which resets to a prior phase explicitly).

#### Scenario: Normal forward transition
- **WHEN** Gandalf approves a gate and advances the phase
- **THEN** the `phase` field SHALL be set to the next phase in sequence and `lembas_completed` and `metadata_updated` SHALL be reset to `false`

#### Scenario: Invalid backward transition
- **WHEN** any actor attempts to set `phase` to a value earlier in the sequence than the current phase
- **THEN** the transition SHALL be rejected (no change to state file) unless explicitly tagged as a recovery reset

### Requirement: Gate pending lifecycle
When `gate_pending` is `true`, the teammate is blocked. Only an external actor (Gandalf) SHALL clear `gate_pending` by writing to the state file. The teammate's own session SHALL NOT modify `gate_pending` from `true` to `false`.

#### Scenario: Gate submitted
- **WHEN** a gate message is sent and `gate_pending` transitions from `false` to `true`
- **THEN** a `gate_id` SHALL be generated (format: `gate-<phase>-<unix_timestamp>`) and written to the state file

#### Scenario: Gate approved by Gandalf
- **WHEN** Gandalf writes to the state file setting `gate_pending` to `false`
- **THEN** `phase` SHALL advance to the next phase, `gate_id` SHALL be set to `null`, and `lembas_completed` and `metadata_updated` SHALL be reset to `false`

#### Scenario: Teammate cannot self-clear gate
- **WHEN** the teammate's hooks or tools attempt to set `gate_pending` from `true` to `false`
- **THEN** this transition SHALL NOT occur (hooks only set `gate_pending` to `true`, never to `false`)

### Requirement: State file cleanup
The state file SHALL be treated as ephemeral. It is cleaned up when the worktree is removed at Phase 5 (Complete) or on manual worktree deletion. No archival or persistence beyond the worktree lifetime is required.

#### Scenario: Worktree cleanup
- **WHEN** the worktree is removed after quest completion
- **THEN** `tmp/quest-state.json` SHALL be removed along with all other worktree contents
