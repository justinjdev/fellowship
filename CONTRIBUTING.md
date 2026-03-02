# Contributing to Fellowship

Thanks for your interest in contributing to Fellowship — a Claude Code plugin for orchestrating multi-task workflows.

## What This Repo Is

Fellowship is a Claude Code plugin. Skills, agents, and docs are markdown. Gate enforcement hooks are bash scripts. There is no build system and no dependencies beyond `jq` (for hooks).

## Getting Started

1. Clone the repo
2. Test locally: `claude --plugin-dir .`
3. Run hook tests: `./hooks/test-hooks.sh`

## What We're Looking For

- **Bug reports** — especially around gate enforcement, quest lifecycle, and fellowship coordination
- **Skill improvements** — clearer prompts, better phase transitions, edge case handling
- **Hook hardening** — error handling, new test cases, edge case coverage
- **Documentation** — README clarity, config examples, usage guides

## How to Contribute

1. Open an issue first for non-trivial changes so we can discuss the approach
2. Fork the repo and create a branch from `main`
3. Make your changes
4. Run `./hooks/test-hooks.sh` and confirm all tests pass
5. Open a PR with a clear description of what and why

## Conventions

- **Commits**: use conventional commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`)
- **Skills**: SKILL.md files with YAML frontmatter (`name` and `description` fields)
- **Skill names**: must not collide with Claude Code built-in commands (`help`, `clear`, etc.)
- **Hooks**: bash scripts in `hooks/scripts/`, sourcing `_common.sh` for shared state file logic
- **Tests**: add test cases to `hooks/test-hooks.sh` for any hook behavior changes
- **Changelog**: don't edit — maintainer updates it at release time

## Repo Structure

```
.claude-plugin/plugin.json   # Plugin manifest
skills/<name>/SKILL.md       # Skills (markdown with YAML frontmatter)
agents/<name>.md             # Agent definitions
hooks/hooks.json             # Plugin hook definitions
hooks/scripts/*.sh           # Gate enforcement scripts (require jq)
hooks/test-hooks.sh          # Hook test suite
README.md                    # User-facing docs
CLAUDE.md                    # AI assistant conventions
```

## Hook Architecture

The gate enforcement system uses Claude Code's plugin hooks to enforce a linear phase progression during quests. Hooks are bash scripts that intercept tool calls, inspect the quest state, and block or allow actions based on the current phase and gate status.

### State File

All hooks operate on a shared state file at `tmp/quest-state.json` (resolved relative to the git repo root). Key fields:

| Field | Type | Purpose |
|---|---|---|
| `phase` | string | Current quest phase: `Onboard`, `Research`, `Plan`, `Implement`, `Review`, `Complete` |
| `gate_pending` | boolean | Whether a gate is awaiting lead approval |
| `gate_id` | string/null | Unique ID of the pending gate (e.g., `gate-Research-1709308800`) |
| `lembas_completed` | boolean | Whether `/lembas` has been invoked this phase (gate prerequisite) |
| `metadata_updated` | boolean | Whether task metadata has been updated this phase (gate prerequisite) |
| `auto_approve_gates` | array | Phase names that auto-advance without lead approval |

When no state file exists (non-quest sessions), all hooks exit 0 immediately — they are no-ops outside quest context.

### Hook Registration (`hooks.json`)

Hooks are registered by tool matcher and timing:

**PreToolUse** (runs before the tool executes, can block it):

| Matcher | Script | Purpose |
|---|---|---|
| `Edit\|Write\|Bash\|Agent\|Skill\|NotebookEdit` | `gate-guard.sh` | Block work tools when a gate is pending; block file modifications during early phases |
| `SendMessage` | `gate-submit.sh` | Detect gate messages, validate prerequisites, manage gate submission |
| `TaskUpdate` | `completion-guard.sh` | Block task completion unless phase is `Complete` |

**PostToolUse** (runs after the tool executes, tracks state):

| Matcher | Script | Purpose |
|---|---|---|
| `Skill` | `gate-prereq.sh` | Track `/lembas` invocation as a gate prerequisite |
| `TaskUpdate` | `metadata-track.sh` | Track phase metadata updates as a gate prerequisite |

### Scripts

**`_common.sh`** — Shared setup sourced by every hook script. Resolves the state file path via `git rev-parse --show-toplevel`, checks that `jq` is installed, reads and validates the state JSON. If the state file is missing, exits 0 (no-op). If `jq` is missing or the state file contains invalid JSON, exits 2 (fail-closed).

**`gate-guard.sh`** — PreToolUse guard for work tools. Two layers of protection:
1. **Gate block:** When `gate_pending=true`, blocks all matched tools (exit 2). The teammate must wait for lead approval.
2. **Phase-aware file guard:** During `Onboard`, `Research`, and `Plan` phases, blocks `Edit`, `Write`, and `NotebookEdit` calls targeting files outside `tmp/`. This prevents production code changes before the Implement phase while still allowing state file writes and checkpoint saves.

**`gate-submit.sh`** — PreToolUse hook on `SendMessage`. Detects gate messages by the `[GATE]` prefix in message content. When a gate message is detected:
1. Rejects multiple `[GATE]` markers in one message
2. Blocks if a gate is already pending
3. Checks prerequisites: both `lembas_completed` and `metadata_updated` must be `true`
4. Determines the next phase from the current one (Onboard->Research->Plan->Implement->Review->Complete)
5. If the next phase is in `auto_approve_gates`: advances the phase, resets prerequisites, does not set `gate_pending`
6. Otherwise: sets `gate_pending=true` and generates a unique `gate_id`

Non-gate messages (no `[GATE]` prefix) pass through without inspection.

**`gate-prereq.sh`** — PostToolUse hook on `Skill`. After any skill invocation, checks if the skill name contains "lembas" (matching any plugin namespace). If so, sets `lembas_completed=true` in the state file.

**`metadata-track.sh`** — PostToolUse hook on `TaskUpdate`. After any task update, checks if the input contains `metadata.phase`. If so, sets `metadata_updated=true` in the state file.

**`completion-guard.sh`** — PreToolUse hook on `TaskUpdate`. When the update sets `status=completed`, blocks (exit 2) unless the current phase is `Complete`. Non-completion updates (status changes, metadata) pass through.

### Phase Progression

Gates enforce this linear sequence — no phase can be skipped:

```
Onboard → Research → Plan → Implement → Review → Complete
```

To advance through a gate, a teammate must:
1. Invoke `/lembas` (tracked by `gate-prereq.sh`, sets `lembas_completed=true`)
2. Update task metadata with the current phase (tracked by `metadata-track.sh`, sets `metadata_updated=true`)
3. Send a `[GATE]` message to the lead (processed by `gate-submit.sh`)
4. Stop and wait for lead approval (enforced by `gate-guard.sh` blocking all tools while `gate_pending=true`)

After the lead approves, the state file is updated externally: `gate_pending=false`, `phase` advanced, prerequisites reset.

### Error Handling

All hooks follow a fail-closed pattern:
- Malformed JSON input: exit 2 (block the tool call)
- Missing state file: exit 0 (no-op — not a quest session)
- Missing `jq`: exit 2 with an install message
- Invalid state file JSON: exit 2

## Testing

The hook test suite is the primary automated test:

```bash
./hooks/test-hooks.sh
```

For end-to-end testing, run a fellowship or quest locally with `claude --plugin-dir .` and verify gate behavior manually.

## Questions?

Open an issue or start a discussion on the repo.
