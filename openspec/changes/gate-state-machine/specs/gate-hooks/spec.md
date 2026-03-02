## ADDED Requirements

### Requirement: Hook no-op when no state file
All hooks SHALL check for the existence of `tmp/quest-state.json` before performing any logic. If the file does not exist, the hook SHALL exit 0 immediately (no-op). This ensures normal user sessions and Gandalf's session are unaffected.

#### Scenario: Normal user session
- **WHEN** a hook fires in a session without `tmp/quest-state.json`
- **THEN** the hook SHALL exit 0 without reading stdin or producing output

#### Scenario: Gandalf's session
- **WHEN** a hook fires in Gandalf's coordinator session (main worktree, no state file)
- **THEN** the hook SHALL exit 0 without affecting Gandalf's tool calls

### Requirement: Graceful degradation without jq
All hooks SHALL check for the `jq` binary before performing JSON operations. If `jq` is not available, the hook SHALL emit a warning to stderr and exit 0 (non-blocking). Gate enforcement degrades to prompt-only behavior rather than breaking the session.

#### Scenario: jq not installed
- **WHEN** a hook fires and `jq` is not found in PATH
- **THEN** the hook SHALL write "fellowship: jq not found, gate enforcement disabled" to stderr and exit 0

### Requirement: Gate guard hook (PreToolUse)
A PreToolUse hook matching `Edit|Write|Bash|Agent|Skill|NotebookEdit` SHALL block tool execution when `gate_pending` is `true` in the state file. The hook SHALL exit 2 with a message directing the teammate to wait for lead approval.

#### Scenario: Tool call while gate pending
- **WHEN** the teammate calls Edit, Write, Bash, Agent, Skill, or NotebookEdit and `gate_pending` is `true`
- **THEN** the hook SHALL exit 2 with stderr message: "Gate pending — waiting for lead approval. Do not take any action until the lead approves your gate."

#### Scenario: Tool call while gate not pending
- **WHEN** the teammate calls any matched tool and `gate_pending` is `false`
- **THEN** the hook SHALL exit 0 (allow the tool call)

#### Scenario: Read and Grep are not blocked
- **WHEN** the teammate calls Read, Grep, Glob, or other read-only tools while `gate_pending` is `true`
- **THEN** those tools SHALL NOT be blocked (they are not in the matcher)

### Requirement: Gate submit hook (PreToolUse on SendMessage)
A PreToolUse hook matching `SendMessage` SHALL detect gate messages by pattern matching on the message content. When a gate message is detected, the hook SHALL verify prerequisites and manage the `gate_pending` state.

#### Scenario: Gate message with prerequisites met
- **WHEN** the teammate sends a message matching the gate pattern and `lembas_completed` is `true` and `metadata_updated` is `true` and `gate_pending` is `false`
- **THEN** the hook SHALL set `gate_pending` to `true`, generate a `gate_id`, write the updated state, and exit 0 (allow the message)

#### Scenario: Gate message with prerequisites not met
- **WHEN** the teammate sends a message matching the gate pattern and either `lembas_completed` is `false` or `metadata_updated` is `false`
- **THEN** the hook SHALL exit 2 with a stderr message listing which prerequisites are missing (e.g., "Gate blocked: lembas not completed, metadata not updated")

#### Scenario: Gate message while gate already pending
- **WHEN** the teammate sends a message matching the gate pattern and `gate_pending` is already `true`
- **THEN** the hook SHALL exit 2 with stderr message: "Gate already pending — wait for lead approval before submitting another gate."

#### Scenario: Non-gate message
- **WHEN** the teammate sends a message that does not match the gate pattern (e.g., "I'm stuck", "question about the API")
- **THEN** the hook SHALL exit 0 (allow the message) regardless of state

#### Scenario: Auto-approved gate
- **WHEN** the teammate sends a gate message and the current phase's gate name is in `auto_approve_gates`
- **THEN** the hook SHALL validate prerequisites, advance `phase` to the next value, reset `lembas_completed` and `metadata_updated` to `false`, and exit 0 WITHOUT setting `gate_pending` to `true`

### Requirement: Gate pattern matching
The gate message detection SHALL match on the presence of a phase keyword (`Research|Plan|Implement|Review|Complete`) combined with checklist markers (`- [x]` or `- [ ]`). Both conditions MUST be present in the message content for it to be classified as a gate message.

#### Scenario: Standard gate checklist
- **WHEN** a SendMessage contains "Research" and "- [x] Key files identified"
- **THEN** it SHALL be classified as a gate message

#### Scenario: Normal discussion mentioning a phase
- **WHEN** a SendMessage contains "I have a question about the Research phase" without checklist markers
- **THEN** it SHALL NOT be classified as a gate message

#### Scenario: Checklist without phase keyword
- **WHEN** a SendMessage contains "- [x] Fixed the bug" without a phase keyword
- **THEN** it SHALL NOT be classified as a gate message

### Requirement: Lembas tracking hook (PostToolUse on Skill)
A PostToolUse hook matching `Skill` SHALL detect when `/lembas` has been invoked and set `lembas_completed` to `true` in the state file.

#### Scenario: Lembas skill invoked
- **WHEN** a PostToolUse event fires for the Skill tool and the tool input indicates `/lembas` or `lembas` was invoked
- **THEN** the hook SHALL set `lembas_completed` to `true` in the state file

#### Scenario: Other skill invoked
- **WHEN** a PostToolUse event fires for the Skill tool and the skill is not lembas (e.g., `/warden`, `/council`)
- **THEN** the hook SHALL NOT modify the state file

### Requirement: Metadata tracking hook (PostToolUse on TaskUpdate)
A PostToolUse hook matching `TaskUpdate` SHALL detect when the teammate updates task metadata with a phase field and set `metadata_updated` to `true` in the state file.

#### Scenario: Phase metadata updated
- **WHEN** a PostToolUse event fires for TaskUpdate and the tool input contains a `metadata` field with a `phase` key
- **THEN** the hook SHALL set `metadata_updated` to `true` in the state file

#### Scenario: Non-phase TaskUpdate
- **WHEN** a PostToolUse event fires for TaskUpdate and the tool input does not contain `metadata.phase`
- **THEN** the hook SHALL NOT modify the state file

### Requirement: Hook distribution via plugin
All hooks SHALL be distributed as part of the fellowship plugin via `hooks/hooks.json` at the plugin root. Hook commands SHALL reference scripts via `${CLAUDE_PLUGIN_ROOT}/hooks/scripts/<name>.sh`. The hooks SHALL merge with any user or project hooks without conflict.

#### Scenario: Plugin installation
- **WHEN** the fellowship plugin is installed and a new session starts
- **THEN** the hooks defined in `hooks/hooks.json` SHALL be active and visible as `[Plugin]` entries in `/hooks`

#### Scenario: Hook coexistence
- **WHEN** a user has their own PreToolUse hooks defined in project settings
- **THEN** both the plugin hooks and user hooks SHALL fire for matching events
