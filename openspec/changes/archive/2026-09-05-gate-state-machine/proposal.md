> **SUPERSEDED.** This proposal's design — bash + `jq` hook scripts enforcing gate state through a JSON file (`tmp/quest-state.json`), across a 6-phase quest lifecycle — was superseded starting with v1.5.1 (Go CLI binary replacing bash scripts) and finalized in v2.0.0 (SQLite storage replacing JSON state files) and v2.2.0 (the Adversarial phase added, making the lifecycle 7 phases with 6 gates). The lifecycle was later collapsed to its current form: 4 phases (Research, Plan, Implement, Review) with 3 gates. Gate enforcement now lives in the `fellowship` Go binary's hooks, reading and writing SQLite-backed state instead of files a shell script parses with `jq`. This document is kept for historical context only; see the README and `plugin/CLAUDE.md` for the current design.

## Why

Fellowship quest teammates violate the gate protocol ~67% of the time (4/6 quests non-compliant across two fellowship runs). Agents continue working after submitting gates, self-approve by writing "approved" in their output, combine multiple gates into single messages, and skip lembas/metadata updates between phases. The root cause is that gate enforcement is entirely prompt-based — agents are *asked* to stop rather than *prevented* from continuing. Industry research confirms prompt-only gates have an inherent failure rate because LLMs are trained toward helpfulness and completion, not stopping.

## What Changes

- **Add a quest state file** (`tmp/quest-state.json`) that tracks current phase, gate status, and prerequisite completion per quest teammate worktree
- **Add plugin hooks** (`hooks/hooks.json` + `hooks/scripts/`) that enforce the state machine deterministically — blocking tool calls when a gate is pending, verifying lembas and metadata were completed before gate submission
- **Add a gate approval mechanism** where Gandalf writes to the teammate's state file to clear `gate_pending`, making self-approval structurally impossible
- **Modify the quest skill** to initialize and update the state file at phase transitions
- **Modify the fellowship skill** spawn prompt to document the state machine and remove ambiguous gate language
- **BREAKING**: The plugin now ships executable shell scripts (previously pure markdown)

## Capabilities

### New Capabilities
- `quest-state-machine`: State file schema, lifecycle (create/read/update/delete), phase transitions, and gate pending/approval flow
- `gate-hooks`: Plugin-distributed hooks that enforce gate discipline — PreToolUse blocking when gate pending, PostToolUse tracking of lembas/metadata completion
- `gate-approval`: Mechanism for Gandalf to approve gates by writing to teammate state files across worktrees

### Modified Capabilities
- None (no existing specs)

## Impact

- **Plugin structure**: Adds `hooks/` directory with `hooks.json` and `scripts/` — first executable code in the plugin
- **Skills**: `quest` SKILL.md needs state file init at Phase 0 and updates at each transition; `fellowship` SKILL.md spawn prompt needs gate language revision
- **Filesystem**: Each quest worktree gets `tmp/quest-state.json` (ephemeral, cleaned up with worktree)
- **Cross-worktree access**: Gandalf must write to teammate worktree paths to approve gates — requires worktree path tracking in task metadata
- **Dependencies**: Shell scripts require `jq` for JSON parsing (standard on macOS, common on Linux)
